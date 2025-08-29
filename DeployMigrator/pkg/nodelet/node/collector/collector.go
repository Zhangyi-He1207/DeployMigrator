package collector

import (
	"hit.edu/framework/pkg/component-base/logs"
	"sync"
)

const namespace = "node"

const (
	defaultEnable  = true
	defaultDisable = false
	STATIC         = "static"
	DYNAMIC        = "dynamic"
)

const (
	// 通道内最大的Metric缓存数量
	capMetricChan = 1000
)

type NewCollectorFactory func() (Collector, error)

var (
	factories = make(map[string]NewCollectorFactory) //factories这个变量必须是全局静态变量，因为cpu.go\memory.go\storage.go在的init方法当中，注册New函数到factories当中，主要是因为init方法在包导入的时候会优先调用，即在NewNodeCollector方法之间init方法就已经被调用
)

// XXCollector模块在注册时，调用该函数，将XXCollector（cpu、storage、memory）的New函数 注册到factories中
func RegisterCollector(name string, isDefaultEnable bool, factory NewCollectorFactory) {
	if isDefaultEnable { //这个参数好像可以删除，已经在NewNodeCollector进行了XXCollector的选择性开启
		factories[name] = factory
	}
}

// XXCollect的接口 例如CPUCollector都得实现这个接口
type Collector interface {
	// 获取新的资源信息
	UpdateStaticInfo(ch chan<- Metric) error
	UpdateDynamicInfo(ch chan<- Metric) error
}

type NodeCollector struct {
	// 所有注册的Collector
	Collectors map[string]Collector
}

func NewNodeCollector(enabledCollectors []string) (*NodeCollector, error) {
	logs.Info("New nodeCollector")
	collectors := make(map[string]Collector)
	var err error
	var failedCollectors []string //如果感觉没有可以删除，记录无法初始化的XXCollector
	// TODO: 创建新的Collector
	// TODO: 实现Collector的注册逻辑
	// 遍历所有的构造函数，添加Collector
	for name, f := range factories {
		if contains(enabledCollectors, name) {
			collectors[name], err = f()
			if err != nil {
				logs.Infof("Warning: failed to initialize collector %s: %v", name, err)
				failedCollectors = append(failedCollectors, name)
				continue
			}
			if collectors[name] == nil {
				logs.Infof("Warning: failed to initialize %s collector: collector does not exist", name)
			}
		}
	}
	if len(failedCollectors) > 0 {
		logs.Infof("Warning: failed to initialize the following collectors: %v", failedCollectors)
	}
	return &NodeCollector{Collectors: collectors}, nil
}
func contains(slice []string, item string) bool {
	for _, value := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// 收集静态信息数据
func (n *NodeCollector) GatherStaticData(processFunc func(types string, metric Metric)) error {

	var metricChan = make(chan Metric, capMetricChan)
	var wg sync.WaitGroup

	//wg.Add(len(n.Collectors))

	for name, c := range n.Collectors { //遍历开启的XXCollector
		if c == nil {
			logs.Error("collector %s does not exist", name)
			continue
		}
		wg.Add(1)
		go func(name string, c Collector) {
			if err := c.UpdateStaticInfo(metricChan); err != nil { //将Metric格式的数据写入到metricChan管道当中
				logs.Error("failed to update static metrics: %v", err)
			}
			wg.Done()
		}(name, c)
	}

	// 所有的数据收集完后，关闭Chan
	wg.Wait()
	close(metricChan) //用于通知接收方发送方已经完成了所有数据的发送，再不会向 Channel 发送更多数据

	// 在主线程中调用 processMetric方法从metricChan管道中读取Metric格式的数据
	for metric := range metricChan { //循环会阻塞，直到有数据进入 metricChan
		processFunc("static", metric)
	}
	return nil // 处理成功完成
}
func (n *NodeCollector) GatherDynamicData(processFunc func(types string, metric Metric)) error {

	var metricChan = make(chan Metric, capMetricChan)
	var wg sync.WaitGroup

	//wg.Add(len(n.Collectors))

	for name, c := range n.Collectors {
		wg.Add(1)
		go func(name string, c Collector) {
			if err := c.UpdateDynamicInfo(metricChan); err != nil {
				logs.Error("failed to update static metrics: %v", err)
			}
			wg.Done()
		}(name, c)
	}

	// 所有的数据收集完后，清除Chan
	wg.Wait()
	close(metricChan)

	// 在主线程中接收 metricChan 中的数据
	for metric := range metricChan { //循环会阻塞，直到有数据进入 metricChan
		processFunc("dynamic", metric)
	}
	return nil // 处理成功完成
}
