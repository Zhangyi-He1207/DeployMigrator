package controller

import (
	"fmt"
	"hit.edu/framework/pkg/apimachinery/fields"
	"hit.edu/framework/pkg/apimachinery/util/wait"
	apis "hit.edu/framework/pkg/apis/cores"
	"hit.edu/framework/pkg/client-go/clients"
	"hit.edu/framework/pkg/client-go/tools/cache"
	"hit.edu/framework/pkg/client-go/util/manager"
	"hit.edu/framework/pkg/client-go/util/workqueue"
	"hit.edu/framework/pkg/component-base/logs"
	"hit.edu/framework/pkg/nodelet/events"
	"strconv"
	"sync"
	"time"
)

const (
	thresholdbattery float64 = 5
	thresholdCPU     float64 = 90
	thresholdGPU     float64 = 90
	thresholdMemory  float64 = 90
	thresholdNetWork float64 = 50
	thresholdStorage float64 = 90
	eventCooldown            = 5 * time.Minute
)

type NodeMonitor struct {
	//nodeClient    core.NodeInterface
	clientsManager *manager.Manager
	//recorder      recorder.EventRecorder
	nodeIndexer   cache.Indexer    //// 本地缓存，提供关于资源的快速查询（索引查询）。 informer会调用Indexer的Add、update、delete方法来实现资源的同步于更新
	nodeInformer  cache.Controller //// cache.Controller 是 k8s中用于控制器模式的核心组件，它封装了资源的监听和事件处理机制，通常用于协调控制循环。，作用：监听资源变化、缓存资源、触发处理逻辑
	queue         workqueue.TypedRateLimitingInterface[string]
	mu            sync.Mutex
	lastEventTime time.Time // 记录节点最后事件时间
}

func NewNodeMonitor(clientSet *clients.ClientSet, clientsManager *manager.Manager, nodeName string) *NodeMonitor {
	//创建资源的List Watcher
	nodeListWatcher := cache.NewListWatchFromClient(clientSet.Core().RESTClient(), "nodes", "test", fields.Everything())
	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
	nodeOptions := cache.InformerOptions{
		ListerWatcher: nodeListWatcher,
		ObjectType:    &apis.Node{}, // 要监听的资源类型
		Handler: cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(oldObj, newObj interface{}) {
				newNode, okNew := newObj.(*apis.Node)
				if !okNew || newNode.Name != nodeName {
					return
				}
				newExceeded := checkNodeThreshold(newNode)
				if newExceeded { // 只要新状态超限就触发
					key, err := cache.MetaNamespaceKeyFunc(newObj)
					if err != nil {
						logs.Errorf("get node key failed: %v", err)
						return
					}
					queue.Add(key)
				}
			},
		},
		ResyncPeriod: 0, // ResyncPeriod，0表示不定期重新同步
		Indexers:     cache.Indexers{},
	}
	nodeIndexer, nodeInformer := cache.NewInformerWithOptions(nodeOptions)

	return &NodeMonitor{
		//nodeClient:   nodeClient,
		//recorder:     recorder,
		clientsManager: clientsManager,
		nodeIndexer:    nodeIndexer,
		nodeInformer:   nodeInformer,
		queue:          queue,
	}
}

func checkNodeThreshold(node *apis.Node) bool {
	// 防御性判空：节点对象及关键路径
	if node == nil || node.Status.Usage == nil {
		return false
	}
	cpuAveUtil := getFloatValue(node.Status.Usage["cpu"][0].Values["AveUtil"])
	memoryUsage := getFloatValue(node.Status.Usage["memory"][0].Values["Usage"])
	storageUsage := getFloatValue(node.Status.Usage["storage"][0].Values["Usage"])
	logs.Infof("检查任务状态----CPU利用率：%v,内存利用率：%v，存储利用率：%v", cpuAveUtil, memoryUsage, storageUsage)
	time.Sleep(20 * time.Second)
	//logs.Info("20秒结束-=-------------------------------------------=")
	return true
	//return cpuAveUtil > thresholdCPU || memoryUsage > thresholdMemory || storageUsage > thresholdStorage
}

func (nm *NodeMonitor) Run(workers int, stopCh <-chan struct{}) {
	defer nm.queue.ShutDown()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		nm.nodeInformer.Run(stopCh)
	}()
	// 等待缓存同步
	if !cache.WaitForCacheSync(stopCh, nm.nodeInformer.HasSynced) {
		logs.Errorf("Timed out waiting for caches to sync")
		return
	}
	// 启动 Worker 协程
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			wait.Until(nm.runWorker, time.Second, stopCh) //周期性执行f方法，间隔period（这里是time.Second）。如果stopCh关闭，则终止循环。
		}()
	}
	<-stopCh
	wg.Wait()
}
func (nm *NodeMonitor) runWorker() {
	for nm.processNextItem() {
	}
}
func (nm *NodeMonitor) processNextItem() bool {
	key, quit := nm.queue.Get() // 调用一次Get必须再调用一次Done，告诉队列这个key的处理已经完成
	defer nm.queue.Done(key)    // 不会移除key，只是配合Get方法表示处理完成而已
	if quit {                   //只有queue.ShutDown()之后，且所有秘钥获取完成时，queue.Get()才会返回quit == true。
		return false
	}

	item, exists, err := nm.nodeIndexer.GetByKey(key)
	if err != nil {
		logs.Errorf("get node %s by key failed: %v", key, err)
		return false
	}
	if !exists {
		logs.Errorf("node %s not exists", key)
		return false
	}
	node := item.(*apis.Node)
	err = nm.generateMigrationEvent(node)
	if err != nil { // event事件生成失败，调用重新机制，往队列加入key
		handleError(nm.queue, key, err)
	} else { // event生成失败，
		nm.queue.Forget(key)
	}
	return true
}

// 错误处理（带速率限制重试 ）
func handleError(queue workqueue.TypedRateLimitingInterface[string], key string, err error) {
	queue.AddRateLimited(key) // 将key重新入队。但是内部使用指数退避（Exponential Backoff），会阶梯延长重试间隔，相当于过一段时间再加入
}

// 产生事件  有个问题，就是这个事件需要不要一直产生
func (nm *NodeMonitor) generateMigrationEvent(n *apis.Node) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nowTime := time.Now()
	if nowTime.Sub(nm.lastEventTime) < eventCooldown { // 为了避免一段时间能重复迁移，设置了一个迁移事件的产生间隔，在这个间隔内，只要生成了这个迁移事件，这段时间内就不会重复再生成
		logs.Infof("Node %s is still in the cooling period (last event time: %s)", n.Name, nm.lastEventTime.Format(time.RFC3339))
		return nil
	}
	nm.clientsManager.LogEventForMigration(n, apis.EventTypeNormal, events.TriggerLocalMigration, fmt.Sprintf("Node Name:\t %s is shortage", n.Name), "", n.Namespace) //
	logs.Info("send Trigger Migration event=====================")
	nm.lastEventTime = nowTime
	return nil
}
func getFloatValue(s string) float64 {
	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		logs.Errorf("Convert value failed, err:%v", err)
		return 0
	}
	return value
}
