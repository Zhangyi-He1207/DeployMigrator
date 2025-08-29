package group

import (
	"encoding/json"
	"hit.edu/framework/pkg/client-go/util/manager"
	"sync"
	"time"

	apis "hit.edu/framework/pkg/apis/cores"
	"hit.edu/framework/pkg/component-base/logs"
	"hit.edu/framework/pkg/nodelet/task/runtime"
)

// 负责对Group的操作进行管理和调度，比如创建、更新、删除、终止等-hzy
type GroupWorkers interface {
	// 更新Group
	UpdateGroup(options *UpdateGroupOptions)
}

type UpdateGroupType int

const (
	GroupCreate UpdateGroupType = iota
	GroupUpdate
	GroupKill
	GroupStop
	GroupRestore
)

// 更新Group中的选项内容
type UpdateGroupOptions struct {
	// Group的开始时间
	StartTime time.Time

	// Group To Update
	Group *apis.Group

	//对Group进行了什么操作-标记一下-hzy
	UpdateType UpdateGroupType
}

type groupWorkers struct {
	//锁
	groupLock sync.RWMutex

	// 存储所有Group的GoRoutines
	groupUpdates map[string]chan *UpdateGroupOptions

	//管理所有Group
	groupManager Manager

	//管理所有的Task
	//taskManager task.Manager

	//以队列的形式管理监控group，实时反馈给aip-server各个group的状态
	queueManager *GroupQueues

	////group-client
	//groupClient core.GroupInterface
	////task-client
	//taskClient core.TaskInterface
	////action-Client
	//actionClient core.ActionInterface
	//// runtime-Client
	//runtimeClient core.RuntimeInterface

	clientsManager *manager.Manager
	// 管理运行所需的Runtime
	// 存储RuntimeManager
	runtimeManager *runtime.RuntimeManager
}

func NewGroupWorkers(groupManager Manager, groupQueues *GroupQueues, runtimeManager *runtime.RuntimeManager, clientsManager *manager.Manager) GroupWorkers {
	//TODO:
	return &groupWorkers{
		runtimeManager: runtimeManager,
		groupManager:   groupManager,
		queueManager:   groupQueues,
		//groupClient:    groupclient,
		//taskClient:     taskclient,
		//actionClient:   actionClient,
		//runtimeClient:  runtimeClient,
		clientsManager: clientsManager,
		groupUpdates:   make(map[string]chan *UpdateGroupOptions),
	}
}

var _ GroupWorkers = &groupWorkers{}

// 这个groupUpdates-Channel是为了同一时刻，任务只能对应一个操作（增加、删除或者更新），管道满的话说明该Group在进行别的操作当中
// 每个Group的操作通过独立的goroutine管理，通过channel通知Group的更新并执行相应的操作。-hzy
func (g *groupWorkers) UpdateGroup(options *UpdateGroupOptions) {
	g.groupLock.Lock()
	defer g.groupLock.Unlock()

	groupName := options.Group.Name
	groupUpdates, exists := g.groupUpdates[groupName] //后期最好将group_workers当中的groupUpdates这个map进行清理（对于已经执行完的group，删除信息）
	if !exists {
		groupUpdates = make(chan *UpdateGroupOptions, 1)
		g.groupUpdates[groupName] = groupUpdates

		go func() {
			g.groupWorkerLoop(groupUpdates) //对每一个任务只开这一个协程
		}()
	}

	// 通知更新
	select {
	case groupUpdates <- options: //往管道中放入options
		logs.Debugf("Group:%s signal sent for %v(0:create;1:update;2:kill)", groupName, options.UpdateType)
	default:
		logs.Warnf("Group:%s update signal skipped (channel busy)", groupName)
	}
}

// 设计管道应该是为了让同一时刻对任务的执行操作（添加、更新、删除）仅能执行一次   --但是还有一个问题就是，for循环接收管道内容后就清空管道内容，有可能这时候执行的操作还没结束，如果这时候别的操作发起请求，则也会导致请求重叠---应该不会，如果再发起请求，前面的操作应该已经结束了
func (g *groupWorkers) groupWorkerLoop(groupUpdates <-chan *UpdateGroupOptions) {
	for update := range groupUpdates { //从管道中读取内容，只要管道中有内容，就往下执行，没有内容就卡在for循环上
		switch update.UpdateType {
		case GroupCreate:
			g.startGroup(update.Group)
		case GroupUpdate:
			g.UpdateGroup(update)
		case GroupKill:
			g.killGroup(update.Group)
		case GroupStop:
			g.stopGroup(update.Group)
		case GroupRestore:
			g.restoreGroup(update.Group)
		default:
			logs.Error("Unhandled default case")
		}
	}
}

func (g *groupWorkers) startGroup(gr *apis.Group) {
	// TODO: 检查Group的运行依赖
	// 当Group bind到当前节点时，开始部署当前Group
	// Group中包含多个Action,Action支持串行和并行执行
	// 任务部署前，需要检查任务的依赖，需要检查的内容包括
	//   资源依赖，任务所需计算、网络、存储或者硬件资源是否就绪
	//   顺序依赖，前序节点是否满足
	//   数据依赖，任务执行所需数据是否准备好
	//   条件依赖，任务执行是否满足条件
	// 首先先将任务放入到checking队列当中  ---也就是对应的
	group, err := g.clientsManager.GetGroup(gr.Name, gr.Namespace) //因为该startGroup方法当中涉及到参数的更新，所以先从etcd中获取一下
	if err != nil {
		logs.Errorf("Get group err:%v", err)
	}
	success := g.queueManager.AddToChecking(group.Name, group)
	if !success {
		// 按理来说不会出现这样的情况，为啥呢，因为如果group_handler.go当中的HandleGroupAdd方法只会执行一次
		logs.Error("Move group into checking queue failed, because groupID has been in checking queue")
		return
	}
	//修改Checking队列当中改group的信息（同时也同步到group_manager当中），状态都改为checking
	g.handleCheckingUpdate(group) //12.31新增：除了修改group的状态，还需要修改上层Task的状态为CheckDeploy
}

//// 对于正常完成的group在更新完group_status之后进行delete操作--目前该方法暂未考虑 1.4
//func (g *groupWorkers) deleteGroup(group *apis.Group) {
//	g.queueManager.DeleteGroup(group) //删除group_manager和queue_manager当中的任务
//}

func (g *groupWorkers) killGroup(group *apis.Group) {
	if g.runtimeManager == nil {
		logs.Error("RuntimeManager is nil")
	}
	for _, actionReference := range group.Status.Actions {
		action, err := g.clientsManager.GetAction(actionReference.Name, actionReference.Namespace)
		if err != nil {
			logs.Errorf("Get action err:%v", err)
		}
		actionStatus := &action.Status
		for _, runtimeReference := range actionStatus.Runtimes {
			runtime, err := g.clientsManager.GetRuntime(runtimeReference.Name, runtimeReference.Namespace)
			if err != nil {
				logs.Errorf("Get runtime err:%v", err)
			}
			runtimeStatus := &runtime.Status
			if runtimeStatus.Phase == apis.Successed || runtimeStatus.Phase == apis.DeployCheck || runtimeStatus.Phase == apis.Failed {
				// runtime已经执行完成，不用再kill了
				continue
			}
			err = g.runtimeManager.Kill(group, action, runtime, action.Spec.Name, runtime.Spec.Name)
			if err != nil {
				logs.Errorf("Kill task err:%v", err)
			}
		}
	}
	//这里需要关闭for循环，因为任务结束了，这个任务对应的管道需要被关闭，否则会一直开着
	groupName := group.Name // 获取groupName
	g.groupLock.Lock()
	defer g.groupLock.Unlock()
	delete(g.groupUpdates, groupName) // 删除对应的key：group，value：channel
	return
}

// 目前和killGroup方法实现的一样
func (g *groupWorkers) stopGroup(group *apis.Group) {
	if g.runtimeManager == nil {
		logs.Error("RuntimeManager is nil")
	}
	for _, actionReference := range group.Status.Actions {
		action, err := g.clientsManager.GetAction(actionReference.Name, actionReference.Namespace)
		if err != nil {
			logs.Errorf("Get action err:%v", err)
		}
		actionStatus := &action.Status
		for _, runtimeReference := range actionStatus.Runtimes {
			runtime, err := g.clientsManager.GetRuntime(runtimeReference.Name, runtimeReference.Namespace)
			if err != nil {
				logs.Errorf("Get runtime err:%v", err)
			}
			runtimeStatus := &runtime.Status
			if runtimeStatus.Phase == apis.Successed || runtimeStatus.Phase == apis.DeployCheck || runtimeStatus.Phase == apis.Failed {
				// runtime已经执行完成，不用再kill了
				continue
			}
			err = g.runtimeManager.Kill(group, action, runtime, action.Spec.Name, runtime.Spec.Name)
			if err != nil {
				logs.Errorf("Kill task err:%v", err)
			}
		}
	}
	//这里需要关闭for循环，因为任务结束了，这个任务对应的管道需要被关闭，否则会一直开着
	groupName := group.Name // 获取groupName
	g.groupLock.Lock()
	defer g.groupLock.Unlock()
	delete(g.groupUpdates, groupName) // 删除对应的key：group，value：channel
	return
}

func (g *groupWorkers) restoreGroup(group *apis.Group) {
	//TODO
	return
}

// 修改group下面的所有状态为Checking  +增加：修改group上层的Task状态为Checking
func (gw *groupWorkers) handleCheckingUpdate(gr *apis.Group) {
	gr.Status.Phase = apis.DeployCheck //首先标记GroupStatus的Phase为DeployCheck
	//gr.Status.LastTime = times  //隐藏
	groupStatus := &gr.Status
	// 更新一下etcd当中的Group的Status
	patchGroup1, err := json.Marshal(map[string]interface{}{
		"status": map[string]interface{}{
			"phase": apis.DeployCheck,
		},
	})
	_, err = gw.clientsManager.PatchGroup(gr.Name, gr.Namespace, patchGroup1)
	if err != nil {
		logs.Errorf("Patch group error222:%v", err)
	}
	// GroupStatus当中的Actions，需要修改下面的（ActionStatus的Phase以及RuntimeStatus的Phase）
	for _, actionReference := range groupStatus.Actions { //Actions
		action, err := gw.clientsManager.GetAction(actionReference.Name, actionReference.Namespace)
		if err != nil {
			logs.Errorf("Get action err:%v", err)
		}
		actionStatus := &action.Status
		// 为了适配迁移，该Action在A设备上已经执行完成了
		if actionStatus.Phase == apis.Successed {
			continue
		}
		actionStatus.Phase = apis.DeployCheck
		// 更新一下etcd当中的Action的Status
		patchAction, err := json.Marshal(map[string]interface{}{
			"status": map[string]interface{}{
				"phase": actionStatus.Phase,
			},
		})
		_, err = gw.clientsManager.PatchAction(action.Name, actionReference.Namespace, patchAction)
		if err != nil {
			logs.Errorf("Patch action err-88:%v", err)
		}
		//groupSpec.Actions[i].Status.LastTime = times //隐藏
		for _, runtimeReference := range actionStatus.Runtimes { // RuntimeStatus
			runtime, err := gw.clientsManager.GetRuntime(runtimeReference.Name, runtimeReference.Namespace)
			if err != nil {
				logs.Errorf("Get runtime err:%v", err)
			}
			runtimeStatus := &runtime.Status
			// 为了适配迁移，该Runtime在A设备上已经执行完成了
			if runtimeStatus.Phase == apis.Successed {
				continue
			}
			runtimeStatus.Phase = apis.DeployCheck
			//actionStatus.RuntimeStatus[j].LastTime = times // 隐藏
			// 更新一下etcd当中的Runtime的Status
			patchRuntime, err := json.Marshal(map[string]interface{}{
				"status": map[string]interface{}{
					"phase": runtimeStatus.Phase,
				},
			})
			_, err = gw.clientsManager.PatchRuntime(runtime.Name, runtimeReference.Namespace, patchRuntime)
			if err != nil {
				logs.Errorf("Patch runtime err-88:%v", err)
			}
		}
		//// 说明Action是第一次启动，这里添加一个操作，将action上传到etcd当中----修改一下改成patch
		//newAction := &apis.Action{
		//	ObjectMeta: metav1.ObjectMeta{Name: groupSpec.Actions[i].Name, Namespace: ""},
		//	TypeMeta:   metav1.TypeMeta{Kind: "Action", APIVersion: "resources/v1"},
		//	Spec:       groupSpec.Actions[i].Spec,
		//	Status:     groupSpec.Actions[i].Status,
		//}
		////action := &groupSpec.Actions[i]
		//_, err := gw.actionClient.Create(context.TODO(), newAction, metav1.CreateOptions{})
		//if err != nil {
		//	logs.Errorf("Create action failed,err-1:%v", err)
		//}
	}

	// 修改Group上层的Task 的Status状态为deploychecking
	// 为了适配迁移，副本group在handleCheckingUpdate方法当中无需再将Task的状态设置为DeployChecking，由源任务进行修改
	if !gr.Spec.IsCopy {
		taskName := gr.Status.Belong.Name // 查找该group所属的Task
		// client-go 查看task-list
		task1, err1 := gw.clientsManager.GetTask(taskName, gr.Status.Belong.Namespace)
		if err1 != nil {
			logs.Errorf("Etcd has group:%v, but not has task:%v, get task err:%v,", gr.Name, taskName, err1)
			return
		}
		logs.Infof("The state of the Task to which the group belongs:%v", task1.Status.Phase)
		if task1.Status.Phase == apis.ReadyToDeploy || task1.Status.Phase == apis.Unknown { // TODO 这里为啥要判断是否DeployCheck--因为group被分配到不同的节点上，遍历到group的时候，都需要修改上层Task的信息的话，是重叠的，没必要  这里逻辑错误，如果第一个group遍历到完并且运行了，这里的Task的状态就行Running
			//task1.Status.Phase = apis.DeployCheck //首先设置Task的状态为DeployCheck
			logs.Trace("=================Task的状态被修改为DeployCheck")
			//task1.Status.LastTime = times //隐藏
			patchTask, err := json.Marshal(map[string]interface{}{
				"status": map[string]interface{}{
					"phase": apis.DeployCheck, //value值不同
				},
			})
			if err != nil {
				logs.Errorf("Json Marshal failed, err:%v", err)
			}
			_, err = gw.clientsManager.PatchTask(task1.Name, task1.Namespace, patchTask)
			if err != nil {
				logs.Errorf("Patch group error:%v", err)
			}
		}
	}
}
