package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"hit.edu/framework/pkg/apimachinery/watch"
	"hit.edu/framework/pkg/apis/meta"
	"hit.edu/framework/pkg/client-go/clients/typed/core"

	"hit.edu/framework/pkg/client-go/util/manager"
	cross_core "hit.edu/framework/test/etcd_sync/active/clients/typed/core"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"hit.edu/framework/pkg/apimachinery/types"
	"hit.edu/framework/pkg/apimachinery/util/wait"
	apis "hit.edu/framework/pkg/apis/cores"
	metav1 "hit.edu/framework/pkg/apis/meta"
	"hit.edu/framework/pkg/client-go/clients"
	"hit.edu/framework/pkg/client-go/util/workqueue"
	"hit.edu/framework/pkg/component-base/logs"
	"hit.edu/framework/pkg/nodelet/events"
	"hit.edu/framework/pkg/nodelet/task/group"
	"hit.edu/framework/pkg/nodelet/task/runtime"
)

const (
	resyncPeriod = 30 * time.Second
)

type MigrationController struct { // 自定义的业务控制器（适配迁移触发），依赖与Informer来收集群中资源变化的事件，并可以基于这些事件采取某些操作，如创建、删除或更新某些资源。
	// client-go
	//groupClient   core.GroupInterface
	//actionClient  core.ActionInterface
	//runtimeClient core.RuntimeInterface
	clientsManager *manager.Manager
	nodeName       string
	// 增加获取所有Namespace下的group的client-go
	allGroupsClient core.GroupInterface
	//// Event 相关组件
	//eventIndexer  cache.Indexer    // 这个参数就是cache.Controller当中的Indexer缓存
	//eventInformer cache.Controller // cache.Controller当中包含了cache.Index ,这里我们将cache.Controller中的Indexer拎出来，是为了更好地编写代码而已，其实不要这个Indexer也是OK的，因为cache.Controller当中也是含有Indexer的
	allEventClient core.EventInterface
	// 工作队列
	//queue workqueue.TypedRateLimitingInterface[string]
	queue workqueue.TypedRateLimitingInterface[*apis.Event]
	// 运行时依赖组件
	runtimeManager *runtime.RuntimeManager

	groupQueues *group.GroupQueues
	// 当前Controller的启动时间
	startTime time.Time
	// event事件发布器
	//recorder recorder.EventRecorder
	// 跨域迁移
	groupTargets   map[string]cross_core.GroupInterface
	actionTargets  map[string]cross_core.ActionInterface
	runtimeTargets map[string]cross_core.RuntimeInterface
	// group_manager
	groupManager group.Manager
	// client-go
	//nodeClient core.NodeInterface
}

func NewMigrationController(clientSet *clients.ClientSet, clientsManager *manager.Manager,
	runtimeManager *runtime.RuntimeManager, groupQueues *group.GroupQueues, nodeName string, groupTarget map[string]cross_core.GroupInterface,
	ActionTarget map[string]cross_core.ActionInterface, runtimeTarget map[string]cross_core.RuntimeInterface, groupManager group.Manager) *MigrationController {
	nowTime := time.Now()
	//创建资源的List Watcher
	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[*apis.Event]())
	// 先创建控制器实例
	ctrl := &MigrationController{
		//groupClient:    groupClient,
		//actionClient:   actionClient,
		//runtimeClient:  runtimeClient,
		//eventClient:     eventClient,
		clientsManager:  clientsManager,
		allGroupsClient: clientSet.Core().Groups(metav1.NamespaceAll),
		allEventClient:  clientSet.Core().Events(metav1.NamespaceAll),
		queue:           queue,
		runtimeManager:  runtimeManager,
		groupQueues:     groupQueues,
		startTime:       nowTime,
		//recorder:        recorder,
		groupTargets:   groupTarget,
		actionTargets:  ActionTarget,
		runtimeTargets: runtimeTarget,
		groupManager:   groupManager,
		nodeName:       nodeName,
		//nodeClient:      nodeClient,
	}
	return ctrl
}
func (mc *MigrationController) eventWatcher() {
	// 筛选出 type是 EventTypeMigration 的事件
	// fieldSelector := fmt.Sprintf("type=%v", apis.EventTypeMigration)
	// 设置长超时时间
	var timeout int64 = 7 * 24 * 60 * 60
	watchOptions := meta.ListOptions{
		TimeoutSeconds: &timeout,
		// FieldSelector:  fieldSelector,
	}
	watcher, err := mc.allEventClient.Watch(context.TODO(), watchOptions)
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
				//logs.Infof("++++++++++++++++++++++Events,event name:%v, event reason:%v,crtl.startTime:%v", event.Name, event.Reason, ctrl.startTime)
				if !ok {
					return
				}
				//logs.Infof("*****************now time:%v,event time:%v", time.Now(), event.EventTime.Time)
				// 合并时间判断和事件条件判断
				//if event.EventTime.Time.Before(ctrl.startTime) ||
				//	event.InvolvedObject.Name != nodeName ||
				//	(event.Reason != events.TriggerLocalMigration && event.Reason != events.TriggerCrossMigration) { // 不是跨域迁移或者本域迁移的话，跳过
				//	return // 跳过历史事件/非本节点事件/非迁移触发事件
				//}
				if event.EventTime.Time.Before(mc.startTime) ||
					(event.Reason != events.TriggerLocalMigration && event.Reason != events.TriggerCrossMigration) { // 不是跨域迁移或者本域迁移的话，跳过
					continue // 跳过历史事件/非本节点事件/非迁移触发事件
				}
				if event.InvolvedObject.Kind == "Node" && event.InvolvedObject.Name != mc.nodeName { //我们希望处理对Group的Migration和Node的Migration，如果是Node的Migration，需要保证和节点的名字一致
					continue
				}
				if event.InvolvedObject.Kind == "Group" {
					_, err := mc.groupManager.GetGroupByName(event.InvolvedObject.Name)
					if err != nil { // 说明该节点上没这个任务，那就不用触发迁移
						continue
					}
				}
				logs.Info("Received migration event[Start Migration]")
				// // 所有条件满足时入队
				logs.Infof("switch controller: event informer AddFunc(): %v", event.Name)
				// key, _ := cache.MetaNamespaceKeyFunc(obj)
				mc.queue.Add(event)
			case watch.Modified:
			case watch.Deleted:
			case watch.Error:
			default:
			}
		}
	}
}

// Run方法
func (mc *MigrationController) Run(workers int, stopCh <-chan struct{}) {
	defer mc.queue.ShutDown()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		mc.eventWatcher() //将事件加入队列
		//mc.eventInformer.Run(stopCh)
	}()

	////缓存同步仅指初始的列表操作完成，后续的更新是由Informer的Watch机制自动处理的，不需要手动同步。因此，在控制器启动时只需要等待一次初始同步即可，之后Informer会自动维护缓存的更新，不需要循环检查。
	//// 等待缓存同步 启动后第一次将全量数据加载到本地缓存中
	//if !cache.WaitForCacheSync(stopCh, mc.eventInformer.HasSynced) { //WaitForCacheSync:是否同步完成，返回false的话报错
	//	logs.Errorf("Timed out waiting for caches to sync")
	//	return
	//}
	//logs.Trace("缓存同步完成=======================")
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			wait.Until(mc.runWorker, time.Second, stopCh) //消费队列
		}()
	}
	<-stopCh
	wg.Wait()
}

func (mc *MigrationController) runWorker() {
	for {
		// 获取队列项
		item, quit := mc.queue.Get()
		if quit {
			return
		}
		defer mc.queue.Done(item)
		if err := mc.handleEventEvent(item); err != nil {
			// 使用指数退避的重试机制
			if mc.queue.NumRequeues(item) < 5 {
				mc.queue.AddRateLimited(item)
			} else {
				logs.Errorf("Error syncing item %v: %v", item, err)
				mc.queue.Forget(item)
			}
		}
	}
}

// 处理Event事件
func (c *MigrationController) handleEventEvent(event *apis.Event) error {
	//obj, exists, err := c.eventIndexer.GetByKey(key)
	//if err != nil {
	//	return fmt.Errorf("error fetching object with key %s from store: %v", key, err)
	//}
	//// 情况1：Evnet已删除
	//if !exists {
	//	return c.handleDeleteEvent(key)
	//}
	//event := obj.(*apis.Event)
	// 1、节点资源不足触发的迁移
	if event.InvolvedObject.Kind == "Node" {
		nodeName := event.InvolvedObject.Name          // TODO 目前还只适配了节点资源不足触发的迁移
		return c.triggerNodeMigration(nodeName, event) // 节点资源不足，目前设计为：仅域内迁移
	} else { // 2、人为想触发迁移某个任务  Kind可以定为Group
		groupName := event.InvolvedObject.Name
		groupNamespace := event.InvolvedObject.Namespace
		return c.triggerGroupMigration(groupNamespace, groupName, event)
	}
}
func (mc *MigrationController) handleDeleteEvent(key string) error {
	// 这块TODO 如果Event删除了，那就说明会不会是Event上传了又马上撤销了，那不管了
	return nil
}

// 触发Group迁移，带重试机制
func (mc *MigrationController) triggerGroupMigration(groupNamespace, groupName string, event *apis.Event) error {
	logs.Trace("-----------------------人为触发迁移-------------------------")
	group, err := mc.clientsManager.GetGroup(groupName, groupNamespace)
	if err != nil {
		logs.Errorf("Get group %s failed: %v", groupName, err)
	}
	// 带基本的重试的逻辑
	if err := retryOnError(3, func() error {
		return mc.migrateGroup(group, event)
	}); err != nil {
		logs.Errorf("Migrate group %s failed: %v", groupName, err)
	}
	return nil
}

// 这里有个优化的地方，就是迁移的话，涉及到多个group同时迁移，那么这里普通做法是遍历，但是效果不咋好，所以我们这里可以加一个工作池（协程）
func (c *MigrationController) triggerNodeMigration(nodeName string, event *apis.Event) error {
	logs.Info("-----------------------节点资源不足，触发迁移-------------------------")
	// 标记节点为迁移状态，这样就不要调度到本节点---这个好像没有必要？
	// 通过索引获取关联的Groups
	//groups, err := c.groupIndexer.ByIndex("ByNode", nodeName)
	//groupClient := c.clientsManager.GetGroupClient("test")
	groupList, err := c.allGroupsClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logs.Errorf("List groups failed: %v", err)
	}
	var groups []*apis.Group
	for i := range groupList.Items {
		group := groupList.Items[i]
		if group.Status.Phase == apis.Running && (group.Status.Node != nil && *group.Status.Node == nodeName) {
			groups = append(groups, &group)
		}
	}
	logs.Infof("group的长度：%v", len(groups))
	const maxWorkers = 5
	var (
		wg     sync.WaitGroup
		errMu  sync.Mutex
		errors []error
	)

	workCh := make(chan interface{}, len(groups))
	for _, group := range groups {
		workCh <- group
	}
	close(workCh)

	// 启动工作池
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() { // 相当于每个Workers开一个协程去处理，他们遍历workCh，空闲的Workers就去取
			defer wg.Done()
			for obj := range workCh {
				group, ok := obj.(*apis.Group)
				if !ok {
					errMu.Lock()
					errors = append(errors, fmt.Errorf("无效对象类型：%T", obj))
					errMu.Unlock()
					continue
				}
				// 带基本的重试的逻辑
				if err := retryOnError(3, func() error {
					return c.migrateGroup(group, event)
				}); err != nil {
					errMu.Lock()
					errors = append(errors, fmt.Errorf("迁移失败[%s]:%w", group.Name, err))
					errMu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("部分迁移失败(%d/%d): 首个错误: %v", len(errors), len(groups), errors[0])
	}
	return nil
}

func retryOnError(attempts int, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		time.Sleep(time.Duration(i*i) * time.Second)
	}
	return err
}

// TODO 任务组迁移核心逻辑---我感觉这里的核心是要改成如果这个方法当中有一步没执行成功，那么如何再次执行，让其成功
func (mc *MigrationController) migrateGroup(group *apis.Group, event *apis.Event) error { // 该group是从etcd当中获取的
	//版本一：最优的方法
	go func() {
		nowtime := apis.Time{time.Now()}
		patchGroup, _ := json.Marshal(map[string]interface{}{
			"status": map[string]interface{}{
				"storeTime": nowtime,
			},
		})
		_, err := mc.clientsManager.PatchGroup(group.Name, group.Namespace, patchGroup)
		if err != nil {
			logs.Errorf("Patch group err100:%v", err)
		}
	}()
	// 版本2
	//nowtime := apis.Time{time.Now()}
	//patchGroup1, _ := json.Marshal(map[string]interface{}{
	//	"status": map[string]interface{}{
	//		"storeTime": nowtime,
	//	},
	//})
	//_, err := mc.clientsManager.PatchGroup(group.Name, group.Namespace, patchGroup1)
	//if err != nil {
	//	logs.Errorf("Patch group err100:%v", err)
	//}
	//time.Sleep(130 * time.Millisecond)
	// -----截止
	// 增加一条规则：如果Group不是细粒度控制的，那么就不进行迁移---可能
	// 1、检查当前状态
	if group.Status.Phase == apis.Migrating || group.Status.Phase == apis.Migrated {
		logs.Infof("Group:%v has Migrated", group.Name)
		return nil
	}
	// 检查该Group下面是否有Runtime是细粒度控制的，如果有的话，才适合迁移 TODO 目前是做成只有细粒度控制的才迁移，没有细粒度控制的，不能迁移
	var needMigration = false
	for actionIndex := range group.Spec.Actions {
		actionSpec := group.Spec.Actions[actionIndex]
		for runtimeIndex := range actionSpec.Runtimes {
			runtimeSpec := actionSpec.Runtimes[runtimeIndex]
			if runtimeSpec.EnableFineGrainedControl {
				needMigration = true
				break
			}
		}
		if needMigration {
			break
		}
	}
	if !needMigration {
		logs.Infof("No-trigger switching")
		return nil
	}
	// 2、迁移
	logs.Infof("Group:%v migration start", group.Name) //此处作为迁移的开始
	var groupCopyName string
	var copiesInDomain, copiesInOtherDomain int32 = 0, 0
	// 安全处理逻辑
	// 情况1：用户未传参时 Replicas == nil
	if group.Spec.Replicas == nil {
		logs.Info("Replicas未配置，使用默认值[0,0]")
	} else if len(group.Spec.Replicas) < 2 {
		logs.Warn("Replicas长度不足，使用前N个值并用0补全",
			"输入值", group.Spec.Replicas,
			"有效长度", len(group.Spec.Replicas))
		// 安全取值（避免越界）
		if len(group.Spec.Replicas) >= 1 {
			copiesInDomain = group.Spec.Replicas[0]
		}
		// 第二个值保持默认0
	} else { // 情况3：正常情况
		copiesInDomain = group.Spec.Replicas[0]
		copiesInOtherDomain = group.Spec.Replicas[1]
	}
	if group.Spec.HasReplca == true {
		copiesInDomain = 1
	}
	// 加一个判断，如果说是域内迁移，迁移的点和副本的点一致，那么走副本预部署这条路；如果说迁移的点和副本不一致，走实时迁移这条路
	// 如果是跨域迁移，迁移的域和副本的域一致，走副本预部署这条路；如果说迁移的域和副本不一致，走实时迁移这条路
	if copiesInDomain+copiesInOtherDomain > 0 { // 提前部署了副本，那么就是修改副本的状态，让副本真正启动（Init->Running）
		// 说明当前group已经提前往etcd里写入了副本group，那么此处就不用再写入了，只需要将原先写的副本group信息当中的groupCopy.Status.CopyStatus 改为"Starting"即可-采用patch
		patchGroup, err := json.Marshal(map[string]interface{}{
			"status": map[string]interface{}{
				"copy_status": "Starting",
			},
		})
		if err != nil {
			logs.Errorf("Json Marshal failed, err:%v", err)
		}
		// 从etcd-GroupSpec-copyInfo当中获取,还需要获取Condition，是执行一个副本还是执行全部副本---目前做的简单一下 就选一个副本进行迁移即可
		logs.Trace("init copy_status===================================")
		for key, value := range group.Spec.CopyInfo { // 这里相当于只遍历CopyInfo这个数组当中的第一个元素
			groupCopyName = key
			if value == "local" { // && event.Reason == events.TriggerCrossMigration
				if event.Reason == events.TriggerLocalMigration || event.Reason == events.TriggerCrossMigration { //适配天数环境
					// 查副本group所在的节点是否为要迁移的目的节点
					logs.Trace("init copy_status===================================1")
					getCopyGroup, err := mc.clientsManager.GetGroup(groupCopyName, group.Namespace)
					if err != nil {
						logs.Errorf("Get group %s failed-66: %v", groupCopyName, err)
					}
					logs.Trace("init copy_status===================================2")
					if event.MigrationTarget == "" || event.MigrationTarget == *getCopyGroup.Status.Node { //所要迁的目的地正好和副本所在的节点相同，或者是所要迁的目的地没指定，那么直接走副本的流程
						// 本域迁移  使用本域的通信总线通信copyGroup进行状态的恢复
						logs.Infof("time:%v", time.Now())
						_, err = mc.clientsManager.PatchGroup(groupCopyName, group.Namespace, patchGroup)
						if err != nil {
							logs.Errorf("Patch group error-6:%v", err)
						}
						logs.Info("Change the copy_Status of the copy task to Starting")
					} else {
						continue //说明本域迁移，指定的迁移目的域和副本所在的节点不一致，那就接着查看其他副本，如果遍历完没有副本的话，那就走非预部署的流程
					}
				}
			} else {
				if (event.Reason == events.TriggerCrossMigration) && (event.MigrationTarget == value || event.MigrationTarget == "") { //所要迁的目的地正好和副本所在的域相同，或者是所要迁的目的地没指定，那么直接走副本的流程
					// 跨域迁移 通过跨域的通信总线通知另一个域的副本copyGroup进行状态的恢复,暂时使用update--后期改成patch
					groupTarget := mc.groupTargets[value] //=-=-=-=-
					//copyGroup, err := groupTarget.Get(context.TODO(), "test", groupCopyName, "groups", metav1.GetOptions{})
					//if err != nil {
					//	logs.Infof("Failed to get group from other domain: %v", err)
					//}
					//copyGroup.Status.CopyStatus = "Starting"
					//patchGroup, err := json.Marshal(map[string]interface{}{
					//	"status": map[string]interface{}{
					//		"copy_status": "Starting",
					//	},
					//})
					_, err = groupTarget.Patch(context.TODO(), groupCopyName, types.StrategicMergePatchType, patchGroup, metav1.PatchOptions{})
					if err != nil {
						logs.Errorf("Patch group %s cross domain failed: %v", groupCopyName, err)
					}
				} else {
					continue //说明跨域迁移，指定的迁移目的域和副本所在的域不一致，那就接着查看其他副本，如果遍历完没有副本的话，那就走非预部署的流程
				}

			}
			break
		}
	} else { // 还未部署副本，那就是直接即使触发迁移，粗粒度控制的任务或者细粒度控制的任务都有可能
		// 先根据源group生成一个副本group的Name
		//groupCopyName = "Reason-copy"          // TODO 这里之后改成随机生成即可源group.Name + 一串随机字符,这里刚开始这么定义，是想让副本在指定的节点上生成
		//node, err2 := mc.nodeClient.Get(context.TODO(), *group.Status.Node, metav1.GetOptions{})
		//if err2 != nil {
		//	logs.Errorf("Get node %s failed-66: %v", *group.Status.Node, err2)
		//}
		if event.Reason == events.TriggerLocalMigration { // 本域迁移，指定了目标节点
			logs.Info("-------------未部署副本-非跨域")
			nodeName := event.MigrationTarget // 如果nodeName未获取到，则nodeName = ""
			// TODO 本域迁移，则直接生成group副本并写入本域etcd当中
			// 复制创建一个全新的副本group信息（注意Succeed的Phase不用修改，DeployCheck和Running状态需要修改），另外还需要将副本的groupStatus改为Starting
			groupCopy := NewGroupInfoCopy(group, false, nodeName) //第二个参数表示是否为提前写入etcd，这里为否 ;第三个为副本的名字，第四个主要是，如果指定了迁移到哪个节点，这个值就非空
			// 遍历action和Runtime，依次创建
			for _, actionReference := range group.Status.Actions {
				actionClient := mc.clientsManager.GetActionClient(actionReference.Namespace)
				action, err := actionClient.Client.Get(context.TODO(), actionReference.Name, metav1.GetOptions{})
				if err != nil {
					logs.Errorf("Get action %s failed: %v", actionReference.Name, err)
				}
				actionCopy := NewActionInfoCopy(action)
				_, err = actionClient.Client.Create(context.TODO(), actionCopy, metav1.CreateOptions{}) // 因为是创建同一个域内的Action副本，所以说副本的namespace和源任务相同，直接用源action的namespace
				if err != nil {
					logs.Errorf("Create copy action %s in local failed: %v", actionReference.Name, err)
				}
				logs.Infof("Create actionCopy:%v", actionCopy.Name)
				for _, runtimeReference := range action.Status.Runtimes {
					runtimeClient := mc.clientsManager.GetRuntimeClient(runtimeReference.Namespace)
					runtime, err := runtimeClient.Client.Get(context.TODO(), runtimeReference.Name, metav1.GetOptions{})
					if err != nil {
						logs.Errorf("Get runtime %s failed: %v", runtimeReference.Name, err)
					}
					runtimeCopy := NewRuntimeInfoCopy(runtime, false)
					_, err = runtimeClient.Client.Create(context.TODO(), runtimeCopy, metav1.CreateOptions{})
					if err != nil {
						logs.Errorf("Create copy runtime %s in local failed: %v", runtimeReference.Name, err)
					}
					logs.Infof("Create runtimeCopy:%v", runtimeCopy.Name)
				}
			}
			logs.Infof("group:%v######################################1", groupCopy.Name)
			groupClient := mc.clientsManager.GetGroupClient(group.Namespace)
			_, err := groupClient.Client.Create(context.TODO(), groupCopy, metav1.CreateOptions{})
			if err != nil {
				logs.Errorf("Create group:%s err: %v", groupCopy.Name, err)
			}
			// 将副本任务信息写入到源任务的CopyInfo当中
			patchGroup, err := json.Marshal(map[string]interface{}{
				"spec": map[string]interface{}{
					"copy_info": map[string]string{groupCopy.Name: "local"}, //value值不同
				},
			})
			if err != nil {
				logs.Errorf("Json Marshal failed, err:%v", err)
			}
			group, err = groupClient.Client.Patch(context.TODO(), group.Name, types.StrategicMergePatchType, patchGroup, metav1.PatchOptions{}) //注意的点：这里将更新好源Group重新赋值给源Group
			if err != nil {
				logs.Errorf("Patch group error-2:%v", err)
			}
			logs.Info("Source CopyInfo:[value:%v]", group.Spec.CopyInfo[groupCopy.Name])
		} else { // 为跨域迁移
			//if nodeName != "" { // 跨域迁移，指定了别的域的目标节点 TODO:后面这块应该删了，跨域迁移时不指定节点，只指定域的
			//	// TODO 首先还得根据nodeName找到是哪个域，然后连接这个与的api-server ---这个得想想怎么操作 还未解决，可能有个问题，就是怎么根据nodeName来定位哪个域的通信链路
			//	// TODO 这里需要和调度器沟通，如果说group的Status中node属性已经指定了，就不需要让调度器再指定节点了
			//	groupCopy := NewGroupInfoCopy(group, false, nodeName) //第二个参数表示是否为提前写入etcd，这里为否 ;第三个为副本的名字，第四个主要是，如果指定了迁移到哪个节点，这个值就非空
			//	groupCopyName = groupCopy.Name
			//	// 遍历action和Runtime，依次创建
			//	for _, actionReference := range group.Status.Actions {
			//		action, err := mc.clientsManager.GetAction(actionReference.Name, actionReference.Namespace) // 从本域获得Action
			//		if err != nil {
			//			logs.Errorf("Get action %s failed: %v", actionReference.Name, err)
			//		}
			//		actionCopy := NewActionInfoCopy(action)
			//		actionTarget := mc.actionTargets["broker"] //-=-=-=-= 这里应该先根据nodeName查到clusterID，然后再使用这个ClusterID TODO 跨域可能也得加上namespace
			//		_, err = actionTarget.Create(context.TODO(), actionCopy, metav1.CreateOptions{})
			//		if err != nil {
			//			logs.Errorf("Create copy action %s in other domainfailed: %v", actionReference.Name, err)
			//		}
			//		logs.Infof("Create actionCopy:%v", actionCopy.Name)
			//		for _, runtimeReference := range action.Status.Runtimes {
			//			runtime, err := mc.clientsManager.GetRuntime(runtimeReference.Name, runtimeReference.Namespace)
			//			if err != nil {
			//				logs.Errorf("Get runtime %s failed: %v", runtimeReference.Name, err)
			//			}
			//			runtimeCopy := NewRuntimeInfoCopy(runtime, true) // 区别，这里是true
			//			runtimeTarget := mc.runtimeTargets["broker"]     //-=-=-=-= 这里应该先根据nodeName查到clusterID，然后再使用这个ClusterID
			//			_, err = runtimeTarget.Create(context.TODO(), runtimeCopy, metav1.CreateOptions{})
			//			if err != nil {
			//				logs.Errorf("Create copy runtime in other domain %s failed: %v", actionReference.Name, err)
			//			}
			//			logs.Infof("Create runtimeCopy:%v", runtimeCopy.Name)
			//		}
			//	}
			//	logs.Infof("group:%v######################################1", groupCopy.Name)
			//	groupTarget := mc.groupTargets["broker"] //-=-=-=-= 这里应该先根据nodeName查到clusterID，然后再使用这个ClusterID
			//	_, err := groupTarget.Create(context.TODO(), groupCopy, metav1.CreateOptions{})
			//	// 将副本任务信息写入到源任务的CopyInfo当中，将nodeName转换为连接
			//	patchGroup, err := json.Marshal(map[string]interface{}{
			//		"spec": map[string]interface{}{
			//			"copy_info": map[string]string{groupCopyName: "broker"}, // TODO 后期这里也得改
			//		},
			//	})
			//	if err != nil {
			//		logs.Errorf("Json Marshal failed, err:%v", err)
			//	}
			//	patchResult, err := mc.clientsManager.PatchGroup(groupCopyName, group.Namespace, patchGroup)
			//	if err != nil {
			//		logs.Errorf("Patch group error:%v", err)
			//	}
			//	logs.Info("Source CopyInfo:[value:%v]", patchResult.Spec.CopyInfo[groupCopyName])
			//} else { // 跨域迁移，未指定迁移到哪个节点，这里直接生成一个事件通过调度器，里面放Group信息
			// TODO （需要和调度器确认）发送一个事件通知调度器去选择一个域（不能为本域），事件里面放group信息
			logs.Info("-------------未部署副本-跨域")
			domainName := event.MigrationTarget
			groupCopy := NewGroupInfoCopy(group, false, "") //第二个参数表示是否为提前写入etcd，这里为否 ;第三个为创建的副本是写到本域还是跨域，主要是为了创建完副本，将该副本信息写到源任务当中的GroupSpec的CopyInfo当中，第四个主要是，如果指定了迁移到哪个节点，这个值就非空
			// 遍历action和Runtime，依次创建
			for _, actionReference := range group.Status.Actions {
				action, err := mc.clientsManager.GetAction(actionReference.Name, actionReference.Namespace) // 从本域获得Action
				if err != nil {
					logs.Errorf("Get action %s failed: %v", actionReference.Name, err)
				}
				actionCopy := NewActionInfoCopy(action)
				actionTarget := mc.actionTargets[domainName] //-=-=-=-= 这里应该先根据nodeName查到clusterID，然后再使用这个ClusterID
				_, err = actionTarget.Create(context.TODO(), actionCopy, metav1.CreateOptions{})
				if err != nil {
					logs.Errorf("Create copy action %s in other domainfailed: %v", actionReference.Name, err)
				}
				logs.Infof("Create actionCopy:%v", actionCopy.Name)
				for _, runtimeReference := range action.Status.Runtimes {
					runtime, err := mc.clientsManager.GetRuntime(runtimeReference.Name, runtimeReference.Namespace)
					if err != nil {
						logs.Errorf("Get runtime %s failed: %v", runtimeReference.Name, err)
					}
					runtimeCopy := NewRuntimeInfoCopy(runtime, true) // 区别，这里是true
					runtimeTarget := mc.runtimeTargets[domainName]   //-=-=-=-= 这里应该先根据nodeName查到clusterID，然后再使用这个ClusterID
					_, err = runtimeTarget.Create(context.TODO(), runtimeCopy, metav1.CreateOptions{})
					if err != nil {
						logs.Errorf("Create copy runtime in other domain %s failed: %v", actionReference.Name, err)
					}
					logs.Infof("Create runtimeCopy:%v", runtimeCopy.Name)
				}
			}
			mc.clientsManager.LogEvent(groupCopy, apis.EventTypeNormal, events.SelectOtherDomain, fmt.Sprintf("Need Scheduler to choose the domain to cross"), groupCopy.Namespace)
			// TODO 这里还是需要调度器选择完节点后生成一个事件来通知部署器，接下来要做的是，监听事件，这块的方法可以自己从group_handler.go当中拿，已经写好

			// 往跨域的etcd里写入group数据
			groupTarget, ok := mc.groupTargets[domainName]
			if !ok {
				logs.Info("[groupTarget]键 'broker' 不存在==========================================")
			}
			_, err := groupTarget.Create(context.TODO(), groupCopy, metav1.CreateOptions{})
			if err != nil {
				logs.Errorf("Create cross-domain group error:%v", err)
			}
			// 将跨域的连接写入到源任务的copyInfo当中
			patchGroup, err := json.Marshal(map[string]interface{}{
				"spec": map[string]interface{}{
					"copy_info": map[string]string{groupCopy.Name: "broker"},
				},
			})
			if err != nil {
				logs.Errorf("Json Marshal failed, err:%v", err)
			}
			group, err = mc.clientsManager.PatchGroup(group.Name, group.Namespace, patchGroup) //注意的点：这里将更新好源Group重新赋值给源Group
			if err != nil {
				logs.Errorf("Patch group error-3:%v", err)
			}
			logs.Info("-------------未部署副本-跨域==============success")
		}
	}

	// 修改源group的Status.phase为Migrating
	go func() {
		patchGroup, err := json.Marshal(map[string]interface{}{
			"status": map[string]interface{}{
				"phase": apis.Migrating,
			},
		})
		if err != nil {
			logs.Errorf("Json Marshal failed, err:%v", err)
		}
		group, err = mc.clientsManager.PatchGroup(group.Name, group.Namespace, patchGroup) //注意的点：这里将更新好源Group重新赋值给源Group
		if err != nil {
			logs.Errorf("Patch group error-7:%v", err)
		}
		logs.Infof("Source groupStatus.Phase:%v", group.Status.Phase)
	}()
	// 获取任务group中需要迁移的runtime的当前的执行状态，并将该状态写入到副本group上的runtimeStatus中的keyStatus，并关闭源group中Running的runtime   注意对于DeployCheck的任务，就不采用这种读取状态并写入的方式
	// 注意：如果说Runtime本身没有细粒度控制的话，就不用再保存任务状态以及写入到副本任务上去
	for _, actionReference := range group.Status.Actions {
		action, err := mc.clientsManager.GetAction(actionReference.Name, actionReference.Namespace)
		if err != nil {
			logs.Errorf("Get action error-121:%v", err)
		}
		actionStatus := &action.Status
		if actionStatus.Phase == apis.Successed {
			continue
		}
		if actionStatus.Phase == apis.Running {
			for _, runtimeReference := range actionStatus.Runtimes {
				runtime, err := mc.clientsManager.GetRuntime(runtimeReference.Name, runtimeReference.Namespace)
				runtimeStatus := &runtime.Status
				if runtimeStatus.Phase == apis.Successed {
					continue
				}
				if runtimeStatus.Phase == apis.Running { // runtime正在运行
					if runtime.Spec.EnableFineGrainedControl { //且runtime是细粒度控制的，那么我们要保存这个runtime的状态，然后传给副本Group中的runtime，然后关闭任务
						//logs.Info("!!!!!!!!!!!!!!!!!!!!!!!!!")
						data := mc.runtimeManager.StoreData(group, action, runtime, action.Spec.Name, runtime.Spec.Name) // 获取group下的正在执行的runtime的关键数据
						//err := mc.runtimeManager.StopRuntime(group, action, runtime, i, j)
						//err := mc.runtimeManager.Kill(group, action, runtime, i, j) // 关闭runtime进程,挪到外面
						//if err != nil {
						//	logs.Errorf("Stop runtime error:%v", err)
						//}
						// 将获取到的任务关键装填数据写入到本域或者跨域的etcd上的副本group当中
						// 遍历copyInfo，
						for _, value := range group.Spec.CopyInfo {
							patchRuntime, err := json.Marshal(map[string]interface{}{
								"status": map[string]interface{}{
									"key_status": data,
								},
							})
							if value != "local" {
								runtimeTarget := mc.runtimeTargets[value]
								// 为跨域迁移--根据GroupName一直找到副本runtime，修改其key_status,这里可以直接根据源Runtime的Name，知道副本任务的Name
								// 进行patch
								_, err = runtimeTarget.Patch(context.TODO(), runtime.Name+"-copy", types.StrategicMergePatchType, patchRuntime, metav1.PatchOptions{})
								if err != nil {
									logs.Errorf("Patch runtime %s failed: %v", runtimeReference.Name+"-copy", err)
								}
							} else { // 本域迁移 根据GroupName一直找到副本runtime，修改其key_status，这里可以直接根据源Runtime的Name，知道副本任务的Name
								_, err = mc.clientsManager.PatchRuntime(runtime.Name+"-copy", runtime.Namespace, patchRuntime)
								if err != nil {
									logs.Errorf("Patch group error-5:%v", err)
								}
							}
						}
						// 关闭源任务当中的runtime
						logs.Info("Shutdown runtime successfully")
					}
					err = mc.runtimeManager.Kill(group, action, runtime, action.Spec.Name, runtime.Spec.Name) //最后都需要将runtime进程关闭
					if err != nil {
						logs.Errorf("Stop group error:%v", err)
					}
				}
			}
		}
		// 将源group从Running队列迁移到Migrated队列
		logs.Trace("--------------DeleteFromRunningAndAddToMigratedQueue=====================")
		ok := mc.groupQueues.DeleteFromRunningAndAddToMigrated(group.Name)
		if !ok {
			logs.Error("Delete group from running queue and add to completed queue failed-2")
		}
	}
	return nil
}

// 举例：str：    migration to：CloudNode1
func extractNode(str string) string {
	re := regexp.MustCompile(`(?i)^.*migration\s+to:\s*(.*)$`) // 忽略大小写，匹配空格  (.*)是用来捕获的内容返回的是CloudNode1
	matches := re.FindStringSubmatch(str)                      //在字符串 str 中查找与正则表达式匹配的内容，返回值：返回一个字符串切片 matches，其中： matches[0]：完整匹配的字符串。 matches[1]：第一个捕获组的内容。 matches[2]：第二个捕获组的内容（如果有），依此类推。
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1]) //去除字符串开头和结尾的空白字符
}

// 新增一个创建一个空白的Group信息，删除不必要的内容（例如Running、DeployCheck的属性都得改为Unknown，时间也得修改）
func NewGroupInfoCopy(g *apis.Group, isAhead bool, nodeName string) *apis.Group { // NodeName表示指定这个Group迁移到哪个节点
	logs.Trace("=====================NewGroupInfoCopy=======================================")
	// 将原始对象序列化为JSON
	data, err := json.Marshal(g)
	if err != nil {
		logs.Errorf("Marshal group:%v error:%v", g.Name, err)
	}
	// 反序列化为新的对象
	var copyGroup apis.Group // 非指针
	err = json.Unmarshal(data, &copyGroup)
	if err != nil {
		logs.Errorf("Unmarshal group:%v error:%v", g.Name, err)
	}
	var groupCopy = &copyGroup // 转换为指针
	// 接下来修改这个复制出来的Group信息，首先修改group.Name
	groupCopy.Name = groupCopy.Name + "-copy"

	// 将groupCopy的ResourceVersion置空，注意：这是必须的，否则报错
	groupCopy.ResourceVersion = ""
	// 修改GroupSpec下的Actions数组当中ActionStatus的Phase和time
	groupCopy.Spec.IsCopy = true // 标记改Group为副本group
	// 这个副本group信息当中，其副本数量直接置为0（意思是：不再为副本订制副本）
	groupCopy.Spec.Replicas = []int32{0, 0}
	groupCopy.Spec.HasReplca = false

	// 修改副本group信息中的属性来标记副本任务需要马上启动(这个属性会在copyPending队列当中去轮询检查的)
	if isAhead {
		logs.Infof("Predeployed replica Group===========")
		groupCopy.Status.CopyStatus = "Waiting" //注意后面真正切换的时候，需要将这个参数改为Starting
	} else {
		logs.Infof("Start the replica Group directly===========")
		groupCopy.Status.CopyStatus = "Starting"
	}
	//修改group_copy_info
	//groupCopy.Spec.CopyInfo = make(map[string]string)
	// 将groupStatus下的node属性进行设置
	groupCopy.Status.Node = StringPtr("")
	if groupCopy.Status.Phase != apis.Successed {
		groupCopy.Status.Phase = apis.Unknown
	}
	if nodeName != "" && groupCopy.Status.Phase != apis.Successed { // 用户指定了group迁移到哪个节点，那么这里直接把调度器的工作给做了，把Group的Status.phase改为ReadyToDeploy，并且把Group的Staus.Node改为指定的节点 TODO 这里需要和调度器说一下，就是调度器读取到分配了的Group，不会再修改
		groupCopy.Status.Node = &nodeName
		groupCopy.Status.Phase = apis.ReadyToDeploy
	}
	// 遍历GroupStatus下面的Action数组，把Reference里面的Actionname，后面都加上"-copy"
	for aSpecName, _ := range groupCopy.Status.Actions {
		objRef := groupCopy.Status.Actions[aSpecName]
		objRef.Name += "-copy"
		groupCopy.Status.Actions[aSpecName] = objRef
	}
	//// 将副本Group的Status的CopyBelongClustID属性设置为源任务所在的集群的id
	//if localClusterID != "" {
	//	groupCopy.Status.CopyBelongClustID = &localClusterID
	//}
	return groupCopy
}

func NewActionInfoCopy(a *apis.Action) *apis.Action {
	logs.Trace("=====================NewActionInfoCopy=======================================")
	// 将原始对象序列化为JSON
	data, err := json.Marshal(a)
	if err != nil {
		logs.Errorf("Marshal Action:%v error:%v", a.Name, err)
	}
	// 反序列化为新的对象
	var copyAction apis.Action // 非指针
	err = json.Unmarshal(data, &copyAction)
	if err != nil {
		logs.Errorf("Unmarshal Action:%v error:%v", a.Name, err)
	}
	var actionCopy = &copyAction // 转换为指针
	// 接下来修改这个复制出来的Action信息，首先修改Action.Name
	actionCopy.Name += "-copy"
	// 将actionCopy的ResourceVersion置空，注意：这是必须的，否则报错
	actionCopy.ResourceVersion = ""
	// 将一些状态置为空 5.26
	actionCopy.Status.Waiting = false

	if actionCopy.Status.Phase != apis.Successed {
		actionCopy.Status.Phase = apis.Unknown
	}
	// 遍历ActionStatus下面的Runtime数组，把Reference里面的Runtimename，后面都加上"-copy"
	for rSpecName, _ := range actionCopy.Status.Runtimes {
		objRef := actionCopy.Status.Runtimes[rSpecName]
		objRef.Name += "-copy"
		actionCopy.Status.Runtimes[rSpecName] = objRef
	}
	return actionCopy
}
func NewRuntimeInfoCopy(r *apis.Runtime, isCrossDomain bool) *apis.Runtime {
	logs.Trace("=====================NewRuntimeInfoCopy=======================================")
	// 将原始对象序列化为JSON
	data, err := json.Marshal(r)
	if err != nil {
		logs.Errorf("Marshal Runtime:%v error:%v", r.Name, err)
	}
	// 反序列化为新的对象
	var copyRuntime apis.Runtime // 非指针
	err = json.Unmarshal(data, &copyRuntime)
	if err != nil {
		logs.Errorf("Unmarshal Action:%v error:%v", r.Name, err)
	}
	var runtimeCopy = &copyRuntime // 转换为指针
	// 接下来修改这个复制出来的Runtime信息，首先修改Runtime.Name
	runtimeCopy.Name += "-copy"
	// 将runtimeCopy的ResourceVersion置空，注意：这是必须的，否则报错
	runtimeCopy.ResourceVersion = ""
	runtimeCopy.Status.ProcessId = StringPtr("")
	// 将一些状态置为空 5.26
	runtimeCopy.Status.Waiting = false
	runtimeCopy.Status.Starting = false
	runtimeCopy.Status.Initing = false
	runtimeCopy.Status.IsDependencySatisf = false
	runtimeCopy.Status.IsParsed = false
	runtimeCopy.Status.DepenPreparing = false

	if runtimeCopy.Status.Phase != apis.Successed {
		runtimeCopy.Status.Phase = apis.Unknown
	}
	if runtimeCopy.Spec.Type == apis.ByPod {
		// 1、因为pod是通过yaml创建，所以的话，这里得修改yaml文件当中的pod.ObjectMeta.Name，让其唯一创建，
		yamlFilePath := r.Spec.Inputs[0].From
		runtimeCopy.Spec.Inputs[0].From = AddCopySuffixToFilePath(yamlFilePath)     //yaml文件名加上-copy后缀
		runtimeCopy.Spec.EnableFineGrainedControlService = StringPtr("10.31.10.20") // 将string字符串转换为指针类型 StringPtr("172.110.0.104")
		runtimeCopy.Spec.EnableFineGrainedControlPort = StringPtr("30053")
		// 2、接着修改yaml当中Service的Selector、修改Pod的ObjectMeta.Labels
		// 3、判断是否为跨域迁移，如果是的话，yaml当中pod下面的Env,连接服务端需要加上域名
		if isCrossDomain {
			runtimeCopy.Spec.EnableFineGrainedControlService = StringPtr("10.31.10.33")
		}

		// 4、接着修改runtime.Spec.EnableFineGrainedControlService
		//parts := strings.Split(*runtimeCopy.Spec.EnableFineGrainedControlService, ".")
		//if len(parts) >= 2 { // 至少包含 service.namespace.svc...
		//	// 从yaml当中读取新修改后的serviceName
		//	parts[0] = "new-service-name"
		//	joinedStr := strings.Join(parts, ".")
		//	runtimeCopy.Spec.EnableFineGrainedControlService = &joinedStr // 替换服务名（此处直接使用 newServiceName，也可动态替换如 oldServiceName+"-copy"）
		//}

		//
		//for j := range action.Status.RuntimeStatus {
		//	runtimeStatus := &action.Status.RuntimeStatus[j]
		//	runtimeStatus.ProcessId = ""
		//	copyRuntime := &action.Spec.Runtimes[j]
		//	if runtimeStatus.Phase != apis.Successed {
		//		runtimeStatus.Phase = apis.Unknown
		//		runtimeStatus.StartAt = apis.Time{time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)}
		//		runtimeStatus.LastTime = apis.Time{time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)}
		//	}
		//	if copyRuntime.Type == apis.ByPod { // 如果pod资源要创建副本的话，pod资源的name不能重复
		//		copyRuntime.Pod.ObjectMeta.Name = copyRuntime.Pod.ObjectMeta.Name + "-copy"
		//		copyRuntime.Service.ObjectMeta.Name = copyRuntime.Service.ObjectMeta.Name + "-copy"
		//
		//		parts := strings.Split(copyRuntime.EnableFineGrainedControlService, ".")
		//		if len(parts) >= 2 { // 至少包含 service.namespace.svc...
		//			// 提取原服务名和命名空间
		//			parts[0] = copyRuntime.Service.ObjectMeta.Name
		//			copyRuntime.EnableFineGrainedControlService = strings.Join(parts, ".") // 替换服务名（此处直接使用 newServiceName，也可动态替换如 oldServiceName+"-copy"）
		//		}
		//		// 修改Service的Selector
		//		//修改Pod的ObjectMeta.Labels
		//		for key, value := range copyRuntime.Service.Spec.Selector {
		//			// 更新Pod的Label值（追加后缀）
		//			newValue := value + "-v2"
		//			copyRuntime.Pod.ObjectMeta.Labels[key] = newValue
		//			// 同步更新Service的Selector值
		//			copyRuntime.Service.Spec.Selector[key] = newValue
		//		}
		//		// 判断是否是跨域迁移，如果是的话，得再加一个ClusterID
		//		if isCrossDomain { // 如果是跨域迁移，那么Pod之间的连接需要加上域名
		//			copyRuntime.Pod.Spec.Containers[0].Env[0].Value = "pve2." + copyRuntime.Pod.Spec.Containers[0].Env[0].Value
		//		}
		//		//copyRuntime.Pod.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": "broker"} // 这个之后想办法  ---二进制测试
		//		//copyRuntime.EnableFineGrainedControlService = "172.100.0.130"                             // 这个之后想办法  ---二进制测试
		//	}
		//	if copyRuntime.Type == apis.ByDeployment {
		//		copyRuntime.Deployment.ObjectMeta.Name = copyRuntime.Deployment.ObjectMeta.Name + "-copy"
		//	}
		//}
	}

	return runtimeCopy
}

func StringPtr(s string) *string {
	return &s
}

// 在文件名中插入 "-copy" 后缀（保持路径结构不变）
func AddCopySuffixToFilePath(originalPath string) string {
	// 分离目录和文件名
	dir := filepath.Dir(originalPath)
	base := filepath.Base(originalPath)

	// 分离文件名和扩展名
	ext := filepath.Ext(base)             // 获取扩展名（如 `.yaml`）
	name := strings.TrimSuffix(base, ext) // 去除扩展名的纯文件名（如 `grpc-client-pod`）

	// 构建新文件名
	newBase := name + "-copy" + ext

	// 组合新路径
	return filepath.Join(dir, newBase)
}
