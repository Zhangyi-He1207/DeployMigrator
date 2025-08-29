package group

import (
	apis "hit.edu/framework/pkg/apis/cores"
	"hit.edu/framework/pkg/component-base/logs"
	"sync"
)

// 队列的key目前都填groupName；group.Name
type GroupQueues struct {
	queueLock        sync.RWMutex
	checkingQueue    map[string]*apis.Group
	copyPendingQueue map[string]*apis.Group
	runningQueue     sync.Map
	MigratedQueue    map[string]*apis.Group
	errorQueue       map[string]*apis.Group
	completedQueue   map[string]*apis.Group
	groupManager     Manager
}

func NewGroupQueues(groupManager Manager) *GroupQueues {
	return &GroupQueues{
		checkingQueue:    make(map[string]*apis.Group),
		copyPendingQueue: make(map[string]*apis.Group),
		runningQueue:     sync.Map{},
		MigratedQueue:    make(map[string]*apis.Group),
		errorQueue:       make(map[string]*apis.Group),
		completedQueue:   make(map[string]*apis.Group),
		groupManager:     groupManager,
	}
}

// add
func (gq *GroupQueues) AddToChecking(key string, value *apis.Group) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.checkingQueue[key]; exists {
		logs.Infof("Group:%v has been added to checking queue", value.Name)
		return false
	}
	gq.checkingQueue[key] = value
	return true
}
func (gq *GroupQueues) AddToRunning(key string, value *apis.Group) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.runningQueue.Load(key); exists {
		logs.Debugf("Group:%v has been added to running queue", value.Name)
		return false
	}
	gq.runningQueue.Store(key, value)
	return true
}
func (gq *GroupQueues) AddToError(key string, value *apis.Group) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.errorQueue[key]; exists {
		logs.Infof("Group:%v has been added to error queue", value.Name)
		return false
	}
	gq.errorQueue[key] = value
	return true
}
func (gq *GroupQueues) AddToCompleted(key string, value *apis.Group) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.completedQueue[key]; exists {
		logs.Infof("Group:%v has been added to completed queue", value.Name)
		return false
	}
	gq.completedQueue[key] = value
	return true
}

// delete
func (gq *GroupQueues) DeleteFromChecking(key string) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.checkingQueue[key]; !exists {
		logs.Debugf("Group:%v not in checking queue", key)
		return false
	}
	delete(gq.checkingQueue, key)
	return true
}
func (gq *GroupQueues) DeleteFromCheckingAndAddToError(key string) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.checkingQueue[key]; !exists {
		logs.Errorf("Group:%v not in checking queue, delete failed-1", key)
		return false
	}
	group := gq.checkingQueue[key]
	delete(gq.checkingQueue, key)

	if _, exists := gq.errorQueue[key]; exists {
		logs.Errorf("Group:%v has been added to error queue, it's a error", key)
		return false
	}
	gq.errorQueue[key] = group
	return true
}

func (gq *GroupQueues) DeleteFromRunningAndAddToError(key string) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()

	group, exists := gq.runningQueue.Load(key)
	if !exists {
		logs.Errorf("Group:%v not in running queue, delete failed", key)
		return false
	}
	gq.runningQueue.Delete(key)

	if _, exists := gq.errorQueue[key]; exists {
		logs.Errorf("Group:%v has been added to error queue, it's a error", key)
		return false
	}
	gq.errorQueue[key] = group.(*apis.Group)
	return true
}
func (gq *GroupQueues) DeleteFromRunningAndAddToCompleted(key string) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	group, exists := gq.runningQueue.Load(key)
	if !exists {
		logs.Infof("Group:%v not in running queue, delete failed", key)
		return false
	}
	gq.runningQueue.Delete(key)
	if _, exists := gq.completedQueue[key]; exists {
		logs.Errorf("Group:%v has been added to completed queue, it's a error", key)
		return false
	}
	gq.completedQueue[key] = group.(*apis.Group)
	return true
}

func (gq *GroupQueues) DeleteFromRunningAndAddToMigrated(key string) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	group, exists := gq.runningQueue.Load(key)
	if !exists {
		logs.Infof("Group:%v not in running queue, delete failed", key)
		return false
	}
	gq.runningQueue.Delete(key)
	if _, exists := gq.MigratedQueue[key]; exists {
		logs.Errorf("Group:%v has been added to migrated queue, it's a error", key)
		return false
	}
	gq.MigratedQueue[key] = group.(*apis.Group)
	return true
}

func (gq *GroupQueues) DeleteFromCheckingAndAddToRunning(key string) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.checkingQueue[key]; !exists {
		logs.Errorf("Group:%v not in checking queue, delete failed", key)
		return false
	}
	group := gq.checkingQueue[key]
	delete(gq.checkingQueue, key)

	if _, exists := gq.runningQueue.Load(key); exists {
		logs.Errorf("Group:%v has been added to running queue, it's error", key)
		return false
	}
	gq.runningQueue.Store(key, group)
	return true
}

func (gq *GroupQueues) DeleteFromCheckingAndAddToCopyPending(key string) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.checkingQueue[key]; !exists {
		logs.Infof("Group:%v not in checking queue, delete failed-2", key)
		return false
	}
	group := gq.checkingQueue[key]
	delete(gq.checkingQueue, key)
	if _, exists := gq.copyPendingQueue[key]; exists {
		logs.Infof("Group:%v has been added to copy-pending queue, it's a error", key)
		return false
	}
	gq.copyPendingQueue[key] = group
	return true
}

func (gq *GroupQueues) DeleteFromCopyPendingAndAddToRunning(key string) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.copyPendingQueue[key]; !exists {
		logs.Infof("Group:%v not in copy-pending queue, delete failed-3", key)
		return false
	}
	group := gq.copyPendingQueue[key]
	delete(gq.copyPendingQueue, key)
	if _, exists := gq.runningQueue.Load(key); exists {
		logs.Errorf("Group:%v has been added to running queue, it's a error", key)
		return false
	}
	gq.runningQueue.Store(key, group)
	return true
}

func (gq *GroupQueues) DeleteFromCopyPendingAndAddToCompleted(key string) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.copyPendingQueue[key]; !exists {
		logs.Infof("Group:%v not in copy-pending queue, delete failed-4", key)
		return false
	}
	group := gq.copyPendingQueue[key]
	delete(gq.copyPendingQueue, key)
	if _, exists := gq.completedQueue[key]; exists {
		logs.Infof("Group:%v has been added to completed queue, it's a error", key)
		return false
	}
	gq.completedQueue[key] = group
	return true
}

func (gq *GroupQueues) DeleteFromMigratedAndAddToCompleted(key string) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.MigratedQueue[key]; !exists {
		logs.Infof("Group:%v not in copy-pending queue, delete failed-4", key)
		return false
	}
	group := gq.MigratedQueue[key]
	delete(gq.MigratedQueue, key)
	if _, exists := gq.completedQueue[key]; exists {
		logs.Infof("Group:%v has been added to completed queue, it's a error", key)
		return false
	}
	gq.completedQueue[key] = group
	return true
}

func (gq *GroupQueues) DeleteFromRunning(key string) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.runningQueue.Load(key); !exists {
		logs.Infof("Group:%v not in running queue", key)
		return false
	}
	gq.runningQueue.Delete(key)
	return true
}
func (gq *GroupQueues) DeleteFromError(key string) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.errorQueue[key]; !exists {
		logs.Infof("Group:%v not in error queue", key)
		return false
	}
	delete(gq.errorQueue, key)
	return true
}
func (gq *GroupQueues) DeleteFromCompleted(key string) bool {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	if _, exists := gq.completedQueue[key]; !exists {
		logs.Infof("Group:%v not in completed queue", key)
		return false
	}
	delete(gq.completedQueue, key)
	return true
}

// get
func (gq *GroupQueues) GetFromChecking(key string) (*apis.Group, bool) {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	value, ok := gq.checkingQueue[key]
	return value, ok
}
func (gq *GroupQueues) GetFromRunning(key string) (*apis.Group, bool) {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	value, ok := gq.runningQueue.Load(key)
	if !ok {
		return nil, false
	}
	return value.(*apis.Group), ok
}
func (gq *GroupQueues) GetFromError(key string) (*apis.Group, bool) {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	value, ok := gq.errorQueue[key]
	return value, ok
}

func (gq *GroupQueues) GetFromCompleted(key string) (*apis.Group, bool) {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	value, ok := gq.completedQueue[key]
	return value, ok
}

func (gq *GroupQueues) GetAllRunning() []*apis.Group {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	values := make([]*apis.Group, 0)
	gq.runningQueue.Range(func(key, value interface{}) bool {
		values = append(values, value.(*apis.Group))
		return true
	})
	return values
}

func (gq *GroupQueues) GetAllChecking() []*apis.Group {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	values := make([]*apis.Group, 0)
	for _, value := range gq.checkingQueue {
		values = append(values, value)
	}
	return values
}

func (gq *GroupQueues) GetAllCopyPending() []*apis.Group {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	values := make([]*apis.Group, 0)
	for _, value := range gq.copyPendingQueue {
		values = append(values, value)
	}
	return values
}

func (gq *GroupQueues) GetAllError() []*apis.Group {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	values := make([]*apis.Group, 0)
	for _, value := range gq.errorQueue {
		values = append(values, value)
	}
	return values
}

func (gq *GroupQueues) GetAllCompleted() []*apis.Group {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	values := make([]*apis.Group, 0)
	for _, value := range gq.completedQueue {
		values = append(values, value)
	}
	return values
}
func (gq *GroupQueues) GetAllMigrated() []*apis.Group {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()
	values := make([]*apis.Group, 0)
	for _, value := range gq.MigratedQueue {
		values = append(values, value)
	}
	return values
}

//// 删除任务信息从queue_manager，同时同步group_manager
//func (gq *GroupQueues) DeleteGroup(group *apis.Group) {
//	gq.queueLock.Lock()
//	defer gq.queueLock.Unlock()
//	groupID := group.Status.GroupID
//	var err error
//	if state, ok := gq.getFromQueue(groupID); ok {
//		switch state {
//		case "checking":
//			gq.DeleteFromChecking(groupID)
//		case "running":
//			gq.DeleteFromRunning(groupID)
//		case "error":
//			gq.DeleteFromChecking(groupID)
//		case "completed":
//			gq.DeleteFromCompleted(groupID)
//		default:
//			err = fmt.Errorf("Unfind group ID %s in queue_manager", groupID)
//		}
//	}
//	if err == nil {
//		gq.groupManager.DeleteGroup(group)
//	}
//}
//
//// 更新任务状态并同步
//func (gq *GroupQueues) UpdateGroup(groupID string, group *apis.Group) error {
//	//更新队列中的任务状态
//	var err error
//	if state, ok := gq.getFromQueue(groupID); ok {
//		switch state {
//		case "checking":
//			logs.Info("Checking group update")
//			gq.checkingQueue[groupID] = group
//		case "running":
//			logs.Info("Running group update")
//			gq.runningQueue.Store(groupID, group)
//		case "error":
//			logs.Info("Error group update")
//			gq.errorQueue[groupID] = group
//		case "completed":
//			logs.Info("Completed group update")
//			gq.completedQueue[groupID] = group
//		default:
//			err = fmt.Errorf("Unfind group named %s", groupID)
//		}
//	}
//	//这一步好像不用了，因为是指针，修改一处即可 TODO 后期优化，groupManager当中不用引入queue_manager
//	if err == nil {
//		gq.groupManager.UpdateGroup(group)
//	}
//	return err
//}
//
//// 从queue队列中找到group所属的队列
//func (gq *GroupQueues) getFromQueue(groupID string) (string, bool) {
//	if _, ok := gq.checkingQueue[groupID]; ok {
//		return "checking", true
//	}
//	if _, ok := gq.runningQueue.Load(groupID); ok {
//		return "running", true
//	}
//	if _, ok := gq.completedQueue[groupID]; ok {
//		return "completed", true
//	}
//	if _, ok := gq.errorQueue[groupID]; ok {
//		return "error", true
//	}
//	return "", false
//}
