package task

import (
	"context"
	"sync"
	"time"

	"hit.edu/framework/pkg/apimachinery/util/wait"
	"hit.edu/framework/pkg/apimachinery/watch"
	meta "hit.edu/framework/pkg/apis/meta"
	"hit.edu/framework/pkg/client-go/util/workqueue"
	"hit.edu/framework/pkg/nodelet/events"

	"hit.edu/framework/pkg/client-go/util/manager"

	metav1 "hit.edu/framework/pkg/apis/meta"
	fileManager "hit.edu/framework/pkg/nodelet/registry"
	"hit.edu/framework/pkg/nodelet/task/controller"
	"hit.edu/framework/pkg/nodelet/task/group/dependency"
	"hit.edu/framework/pkg/utils"

	apis "hit.edu/framework/pkg/apis/cores"
	"hit.edu/framework/pkg/client-go/clients"
	"hit.edu/framework/pkg/client-go/clients/typed/core"
	"hit.edu/framework/pkg/component-base/logs"
	"hit.edu/framework/pkg/nodelet/events/eventbus"
	"hit.edu/framework/pkg/nodelet/task/group"
	"hit.edu/framework/pkg/nodelet/task/monitor"
	"hit.edu/framework/pkg/nodelet/task/runtime"
	"hit.edu/framework/pkg/nodelet/task/types"
)

type Exporter interface {
	Run(ctx context.Context) error
}

type TaskExporter struct {
	// TODO: 增加Client-Go配置
	clientsManager *manager.Manager
	// 增加获取所有Namespace下的group的client-go
	allGroupsClient core.GroupInterface
	// 增加获取所有Namespace下的event的client-go
	allEventsClient core.EventInterface
	// TODO: 增加Event Broadcaster Recorder
	//eventBroadcaster recorder.EventBroadcaster

	// TODO: 增加Group Lister
	groupLister []*apis.Group

	// 管理所有所有的Group
	groupManager group.Manager
	// Group的实际执行单元
	groupWorkers group.GroupWorkers

	// 监控正在执行中Group，包括已经部署的Action的执行状态，和未部署的Action的执行条件
	groupMonitor *monitor.GroupMonitor

	// 处理从上游（API-Server）中的Group的更新事件
	groupHandler *monitor.GroupHandler

	//处理condition
	conditionEngine *utils.ConditionEngine

	// 切换模块
	//groupSwitcher *_switch.GroupSwitch
	migrationController *controller.MigrationController
	nodeMonitor         *controller.NodeMonitor
	// 当前Taskexporter所部署的节点的Name
	nodeName string
	queue    workqueue.TypedRateLimitingInterface[*apis.Event]
	updateCh chan types.GroupUpdate
}

var _ Exporter = &TaskExporter{}

func NewTaskExporter(cfg *Config, clientset *clients.ClientSet, ctx context.Context) (*TaskExporter, error) {
	// Task Exporter配置 config
	taskTargetMap := cfg.taskTargetMap
	groupTargetMap := cfg.groupTargetMap
	actionTargetMap := cfg.actionTargetMap
	runtimeTargetMap := cfg.runtimeTargetMap
	// Client-Go配置
	//nodeClient := clientset.Core().Nodes("test")
	//taskClient := clientset.Core().Tasks("test")
	//groupClient := clientset.Core().Groups("test")
	//eventClient := clientset.Core().Events("test")
	//actionClient := clientset.Core().Actions("test")
	//deviceClient := clientset.Core().Devices("test")
	//runtimeClient := clientset.Core().Runtimes("test")
	clientsManager := manager.NewManager(clientset)
	//事件总线--只使用与Runtime运行时传输状态的
	eb := eventbus.NewEventBus()
	// 全局事件组件的配置
	//eventBroadcaster := recorder.NewBroadcaster()
	//eventBroadcaster.StartRecordingToSink(context.Background(), &core.EventSinkImpl{Interface: eventClient})
	//scheme := scheme.NewScheme()
	//apis.AddToScheme(scheme)
	//recorder := eventBroadcaster.NewRecorder(scheme, "TaskExporter")

	// Manager配置 group
	groupManager := group.NewGroupManager()

	// lister
	lister := groupManager.GetGroups(nil)
	// runtimeManager的配置
	runtimeManager := runtime.NewRuntimeManager(ctx, eb, clientsManager, cfg.NodeName, cfg.wasmToolchainDir, cfg.wasmRuntimePort)
	//dependencyManager配置
	depenManager := dependency.NewDependencyManager()
	//condition engine配置
	conditionEngine := utils.NewConditionEngine() // 初始化时传入 clientset
	// fileManager配置
	fileManager := fileManager.NewFileManager(cfg.FileRegistryAddr)
	// queue_manager
	groupQueues := group.NewGroupQueues(groupManager)
	// workers
	workers := group.NewGroupWorkers(groupManager, groupQueues, runtimeManager, clientsManager)
	// 当前Taskexporter所在节点的NodeName
	nodeName := cfg.NodeName
	//
	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[*apis.Event]())
	taskExporter := &TaskExporter{
		// Monitor配置
		//nodesClient:         nodeClient,
		//tasksClient:         taskClient,
		//gropsClient:         groupClient,
		allGroupsClient: clientset.Core().Groups(metav1.NamespaceAll),
		allEventsClient: clientset.Core().Events(metav1.NamespaceAll),
		clientsManager:  clientsManager,
		//eventBroadcaster:    eventBroadcaster,
		conditionEngine:     conditionEngine,
		groupManager:        groupManager,
		groupLister:         lister,
		groupWorkers:        workers,
		groupMonitor:        monitor.NewGroupMonitor(groupManager, groupQueues, eb, runtimeManager, clientsManager, depenManager, conditionEngine, taskTargetMap, groupTargetMap, actionTargetMap, runtimeTargetMap),
		groupHandler:        monitor.NewGroupHandler(groupManager, workers, groupQueues, clientsManager, groupTargetMap, actionTargetMap, runtimeTargetMap, fileManager),
		migrationController: controller.NewMigrationController(clientset, clientsManager, runtimeManager, groupQueues, nodeName, groupTargetMap, actionTargetMap, runtimeTargetMap, groupManager),
		nodeMonitor:         controller.NewNodeMonitor(clientset, clientsManager, nodeName),
		nodeName:            nodeName,
		updateCh:            make(chan types.GroupUpdate),
		queue:               queue,
	}

	// 需要一个TaskCache,存储当前节点所有的Task信息 ====这是什么意思,有点没懂 ？-hzy
	logs.Info("Init task exporter")
	return taskExporter, nil
}

func (te *TaskExporter) Run(ctx context.Context) error {
	//监听Group资源
	// 任务监控中会产生各类事件，事件也通过Client-Go更新
	// +optional,如果任务开启动态资源调整的需求
	// 任务执行过程中需要动态调整任务进程的资源, 根据当前任务执行的Spec和Status, 通过cGroup动态调整任务执行资源使用情况
	defer close(te.updateCh)
	var wg sync.WaitGroup
	wg.Add(4) //等待三个协程

	go func() {
		defer wg.Done()
		te.groupHandler.Loop(ctx, te.updateCh) //主要监控上层发来的消息，主要是启动、停止任务
	}()

	go func() {
		defer wg.Done()
		te.groupMonitor.Start(ctx) //主要监控正在启动的任务，获取任务状态信息
	}()
	go func() {
		defer wg.Done()
		te.ReceiveGroupInfo(ctx) // 持续从etcd当中读取group
		// te.ReceiveKillEventInfo(2, ctx.Done()) // 持续从etcd当中读取kill-group的指令
	}()
	go func() {
		defer wg.Done()
		te.ReceiveKillEventInfo(2, ctx.Done()) // 持续从etcd当中读取kill-group的指令
	}()

	go te.migrationController.Run(5, ctx.Done())
	//go te.nodeMonitor.Run(2, ctx.Done())
	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
	//select {
	//case <-ctx.Done():
	//	return ctx.Err() //退出是返回错误
	//}
}

//func (te *TaskExporter) ReceiveGroupInfo(ctx context.Context) {
//	for {
//		select {
//		case <-ctx.Done():
//			logs.Info("ReceiveGroupInfo exiting due to context cancel")
//			return
//		default:
//			//读取 etcd当中的group列表
//			//groupsClient := te.clientsManager.GetGroupClient("test")
//			groupList, err := te.allGroupsClient.List(context.TODO(), metav1.ListOptions{})
//			if err != nil {
//				logs.Errorf("List task err:%v", err)
//			}
//			// 遍历group
//			for i := range groupList.Items {
//				gr := &groupList.Items[i] // 修改了此处，如果不行的话，改为原来的
//				//groupName := gr.Name // 这里是一个坑
//				//// 从etcd当中读group的信息
//				//gr, err := te.gropsClient.Get(context.TODO(), groupName, metav1.GetOptions{})
//				//if err != nil {
//				//	logs.Errorf("get group:%s failed", groupName)
//				//}
//				if gr.Status.Node != nil && *gr.Status.Node == te.nodeName { //gr.Status.Node == "CloudNode1"       gr.Status.Node == "EdgeNode1" || gr.Status.Node == "EndNode1"
//					if gr.Status.Phase == apis.ReadyToDeploy {
//						logs.Infof("Receive-GroupName：%v,groupStatus:%v", gr.Name, gr.Status.Phase)
//						groupUpdate := types.GroupUpdate{
//							Group: gr,
//							Op:    types.ADD,
//						}
//						te.updateCh <- groupUpdate
//					} else if gr.Status.Phase == apis.ReadyToKill {
//						groupUpdate := types.GroupUpdate{
//							Group: gr,
//							Op:    types.KILL,
//						}
//						te.updateCh <- groupUpdate
//					}
//				}
//				// 读完一个Group暂停一会
//				select {
//				case <-ctx.Done():
//					return
//				case <-time.After(100 * time.Millisecond):
//				}
//			}
//		}
//		// GroupList 读完暂停一会
//		select {
//		case <-ctx.Done():
//			return
//		case <-time.After(100 * time.Millisecond):
//		}
//	}
//}

func (te *TaskExporter) ReceiveGroupInfo(ctx context.Context) {
	var watchTimeout int64 = 7 * 24 * 3600
	watchOptions := metav1.ListOptions{
		TimeoutSeconds: &watchTimeout,
	}
	watcher, err := te.allGroupsClient.Watch(context.TODO(), watchOptions)
	if err != nil {
		logs.Errorf("Watch start false")
	}
	defer watcher.Stop() // 确保退出时关闭watch
	watchChan := watcher.ResultChan()
	for {
		select {
		case event, ok := <-watchChan:
			if !ok {
				logs.Info("WatchChan closed")
				return
			}
			switch event.Type {
			case watch.Added, watch.Modified:
				if gr, ok := event.Object.(*apis.Group); ok {
					if gr.Status.Node != nil && *gr.Status.Node == te.nodeName { //gr.Status.Node == "CloudNode1"       gr.Status.Node == "EdgeNode1" || gr.Status.Node == "EndNode1"
						if gr.Status.Phase == apis.ReadyToDeploy {
							logs.Infof("Receive-GroupName：%v,groupStatus:%v", gr.Name, gr.Status.Phase)
							groupUpdate := types.GroupUpdate{
								Group: gr,
								Op:    types.ADD,
							}
							te.updateCh <- groupUpdate
						} else if gr.Status.Phase == apis.ReadyToKill {
							groupUpdate := types.GroupUpdate{
								Group: gr,
								Op:    types.KILL,
							}
							te.updateCh <- groupUpdate
						}
					}
				}
			}
		case <-ctx.Done():
			logs.Info("Context cancle")
			return
		}
	}
}

//	func (te *TaskExporter) ReceiveKillEventInfo(ctx context.Context) {
//		nowtime := time.Now() // 启动监听的时刻，为了放置启动nodelet组件的时候，etcd里已经有这类的数据，导致收到了老的event
//		fieldSelector := fmt.Sprintf("reason=%v", events.KillingCommand)
//		watchOptions := meta.ListOptions{
//			FieldSelector: fieldSelector,
//		}
//		watcher, err := te.allEventsClient.Watch(context.TODO(), watchOptions)
//		if err != nil {
//			logs.Errorf("Watch group error:%v", err)
//		}
//		defer watcher.Stop() // 确保 watcher 被停止
//		watchChan := watcher.ResultChan()
//		for {
//			select {
//			case event, ok := <-watchChan:
//				if !ok {
//					logs.Infof("watchChan closed")
//					return
//				}
//				// 打印事件类型和对象的相关信息
//				logs.Tracef("接收到事件类型: %v\n", event.Type)
//				switch event.Type {
//				case watch.Added:
//					logs.Infof("资源被添加: ", event.Object)
//					newEvent := event.Object.(*apis.Event)
//					group, err := te.groupManager.GetGroupByName(newEvent.InvolvedObject.Name) //groupManager当中有此group
//					if err != nil {
//						logs.Infof("Group not in nodelet")
//						return
//					}
//					if group != nil && newEvent.EventTime.Time.After(nowtime) { //前者晚于后者返回true
//						// 调用kill方法
//						groupUpdate := types.GroupUpdate{
//							Group: group,
//							Op:    types.KILL,
//						}
//						te.updateCh <- groupUpdate
//					}
//				default:
//					logs.Infof("未识别的事件类型: ", event.Type)
//				}
//			}
//		}
//	}
func (te *TaskExporter) eventWatcher() {
	// 筛选出 type是 EventTypeMigration 的事件
	// fieldSelector := fmt.Sprintf("type=%v", apis.EventTypeMigration)
	startTime := time.Now()
	// 设置长超时时间
	var timeout int64 = 7 * 24 * 60 * 60
	watchOptions := meta.ListOptions{
		TimeoutSeconds: &timeout,
		// FieldSelector:  fieldSelector,
	}
	watcher, err := te.allEventsClient.Watch(context.TODO(), watchOptions)
	//watcher, err := mc.eventClient.Watch(context.TODO(), watchOptions)
	if err != nil {
		panic(err)
	}
	defer watcher.Stop() // 确保 watcher 被停止
	watchChan := watcher.ResultChan()

	for {
		select {
		case event, ok := <-watchChan:
			if !ok {
				logs.Infof("watchChan closed")
				return
			}
			// 打印事件类型和对象的相关信息
			//logs.Infof("接收到事件类型: %v\n", event.Type)
			switch event.Type {
			case watch.Added:
				// 类型断言放在最外层，避免重复断言
				event, ok := event.Object.(*apis.Event)
				logs.Tracef("++++++++++++++++++++++Events,event name:%v, event reason:%v", event.Name, event.Reason)
				if !ok {
					return
				}
				//logs.Infof("*****************now time:%v,event time:%v", time.Now(), event.EventTime.Time)
				// 合并时间判断和事件条件判断
				if event.EventTime.Time.Before(startTime) ||
					(event.Reason != events.KillingCommand) {
					continue
				}
				if event.InvolvedObject.Kind == "Group" {
					_, err := te.groupManager.GetGroupByName(event.InvolvedObject.Name)
					if err != nil { // 说明该节点上没这个任务，那就不用触发kill
						continue
					}
				}
				logs.Info("++++++++++++++++++++++Events--------事件为kill事件")
				// // 所有条件满足时入队
				logs.Infof("task exporter: event informer AddFunc(): %v", event.Name)
				// key, _ := cache.MetaNamespaceKeyFunc(obj)
				te.queue.Add(event)
			case watch.Modified:
			case watch.Deleted:
			case watch.Error:
			default:
			}
		}
	}
}

// Run方法
func (te *TaskExporter) ReceiveKillEventInfo(workers int, stopCh <-chan struct{}) {
	defer te.queue.ShutDown()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		te.eventWatcher() //将event加入对嘞
		//mc.eventInformer.Run(stopCh)
	}()
	////缓存同步仅指初始的列表操作完成，后续的更新是由Informer的Watch机制自动处理的，不需要手动同步。因此，在控制器启动时只需要等待一次初始同步即可，之后Informer会自动维护缓存的更新，不需要循环检查。
	//// 等待缓存同步 启动后第一次将全量数据加载到本地缓存中
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			wait.Until(te.runWorker, time.Second, stopCh)
		}()
	}
	<-stopCh
	wg.Wait()
}

func (te *TaskExporter) runWorker() {
	for {
		// 获取队列项
		item, quit := te.queue.Get()
		if quit {
			return
		}
		defer te.queue.Done(item)
		te.handleEventEvent(item)
	}
}

func (te *TaskExporter) handleEventEvent(item *apis.Event) {
	group, _ := te.groupManager.GetGroupByName(item.InvolvedObject.Name)
	groupUpdate := types.GroupUpdate{
		Group: group,
		Op:    types.KILL,
	}
	te.updateCh <- groupUpdate
}
