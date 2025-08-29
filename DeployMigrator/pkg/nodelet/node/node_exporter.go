package node

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"hit.edu/framework/pkg/apimachinery/types"
	apis "hit.edu/framework/pkg/apis/cores"
	metav1 "hit.edu/framework/pkg/apis/meta"
	"hit.edu/framework/pkg/client-go/clients"
	"hit.edu/framework/pkg/client-go/clients/typed/core"
	"hit.edu/framework/pkg/component-base/logs"
	"hit.edu/framework/pkg/nodelet/node/collector"
)

// TODO 11.14 node-exporter后续需要实现的功能，数据处理并填入nodestatus字段，通过client-go定期写入api-server中 --完成
// TODO: 接口格式调整
type Exporter interface {
	// TODO: 定义接口
	Run(ctx context.Context) error
}

var _ Exporter = &NodeExporter{}

type NodeExporter struct {
	// Register
	// TODO: 扫描节点信息，并注册到API-Server中
	// TODO: 定义组件
	nodeCollector    *collector.NodeCollector
	staticCache      map[string]collector.Metric
	dynamicCache     map[string]collector.Metric
	staticCacheLock  sync.RWMutex
	dynamicCacheLock sync.RWMutex
	nodesClient      core.NodeInterface
	NodeName         string
	ClusterCategory  string
	LocalClusterID   string
	IsmasterNode     bool
}

func NewNodeExporter(cfg *Config, clientset *clients.ClientSet) (*NodeExporter, error) {
	// TODO：参数配置
	// 创建NodeCollector 读取配置信息
	logs.Info("Init nodeExporter module")
	nc, err := collector.NewNodeCollector(cfg.EnabledCollectors)
	if err != nil {
		return nil, err
	}
	// Client-Go配置
	nodeClient := clientset.Core().Nodes("test")
	// 配置const常量
	return &NodeExporter{
		nodeCollector:   nc,
		staticCache:     make(map[string]collector.Metric),
		dynamicCache:    make(map[string]collector.Metric),
		nodesClient:     nodeClient,
		NodeName:        cfg.NodeName,
		LocalClusterID:  cfg.LocalClusterID,
		ClusterCategory: cfg.ClusterCategory,
		IsmasterNode:    cfg.IsMasterNode,
	}, nil
}
func getHostName() string {
	// 如果运行在Kubernetes中，可以通过Downward API获取节点名称
	if nodeName := os.Getenv("HOST_NAME"); nodeName != "" {
		return nodeName
	}

	// 否则获取本地主机名
	if hostName, err := os.Hostname(); err == nil {
		return hostName
	}
	return "unknown-host"
}

// 想改成每隔60秒收集一次静态信息，每隔1s收集一次动态信息
func (n *NodeExporter) Run(ctx context.Context) error {
	// 如果etcd没有node信息的话，就往etcd当中写入node信息----目前不同node上需要确定不同的node名字
	list, err := n.nodesClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logs.Error("Get node list error", err)
	}
	var nodeInfoIsCreated = false
	for _, d := range list.Items {
		if d.Name == n.NodeName {
			nodeInfoIsCreated = true
			break
		}
	}
	if !nodeInfoIsCreated {
		//etcd当中没有本节点的信息，下进行创建node信息
		node := &apis.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   n.NodeName,
				Labels: n.getIsMasterForLabels(),
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Node",
				APIVersion: "resources/v1",
			},
			Spec: apis.NodeSpec{
				NodeName:        n.NodeName,
				ClusterCategory: n.ClusterCategory,
				Resource:        make(map[string][]apis.Item),
				HostName:        getHostName(),
			},
			Status: apis.NodeStatus{
				Usage: make(map[string][]apis.Item),
			},
		}
		_, err := n.nodesClient.Create(context.TODO(), node, metav1.CreateOptions{})
		if err != nil {
			logs.Errorf("Create node error:%v", err)
		}
	}
	// TODO 执行一个Patch操作，将ClusterCategory属性设置为clusterCategory
	patchNode, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"clusterCategory": n.ClusterCategory,
			"cluster_id":      n.LocalClusterID,
		},
	})
	_, err = n.nodesClient.Patch(context.TODO(), n.NodeName, types.StrategicMergePatchType, patchNode, metav1.PatchOptions{})
	if err != nil {
		logs.Errorf("Patch node error:%v", err)
		return err
	}
	// 先收集一次静态和动态数据
	if err := n.collectAndUploadData("static"); err != nil {
		return err
	}
	if err := n.collectAndUploadData("dynamic"); err != nil {
		return err
	}

	staticTicker := time.NewTicker(time.Second * 60)
	defer staticTicker.Stop()
	dynamicTicker := time.NewTicker(time.Second * 5)
	defer dynamicTicker.Stop()

	// 定期收集数据并上传
	for {
		select {
		case <-staticTicker.C:
			if err := n.collectAndUploadData("static"); err != nil {
				return err
			}
		case <-dynamicTicker.C:
			if err := n.collectAndUploadData("dynamic"); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
func (n *NodeExporter) getIsMasterForLabels() map[string]string {
	if n.IsmasterNode {
		return map[string]string{
			"isCenter": "true",
		}
	} else {
		return nil
	}
}

// collectAndUploadData 执行数据收集和上传操作
func (n *NodeExporter) collectAndUploadData(dataType string) error {
	var err error
	if dataType == "static" {
		err = n.nodeCollector.GatherStaticData(n.processMetri)
	} else if dataType == "dynamic" {
		err = n.nodeCollector.GatherDynamicData(n.processMetri)
	}
	if err != nil {
		return err
	}

	// 获取Node结构体指针
	node, getErr := n.nodesClient.Get(context.TODO(), n.NodeName, metav1.GetOptions{})
	if getErr != nil {
		return fmt.Errorf("Failed to get node: %v", getErr)
	}

	// 上传本地缓存当中的信息到etcd
	n.UploadCache(node, dataType)

	return nil
}

// 读取cache当中的数据，写入到node引用内
func (n *NodeExporter) UploadCache(node *apis.Node, cacheType string) {
	// TODO: 数据预处理，然后放入Node中，然后写入API-Server中----应该是将信息放入到NodeStatus当中，然后通过client-go，定期写入到api-server当中
	var cache map[string]collector.Metric
	var lock *sync.RWMutex

	if cacheType == "static" {
		cache = n.staticCache
		lock = &n.staticCacheLock
	} else {
		cache = n.dynamicCache
		lock = &n.dynamicCacheLock
	}
	lock.Lock()
	defer lock.Unlock()

	if len(cache) == 0 {
		logs.Info("Cache is empty, nothing to upload")
		return
	}
	// 读取内存cache当中的metric，转换为NodeStatus当中的Usage  或者是NodeSpec当中的Resource
	for _, metric := range cache { //静态的话，对应三个Metric，CPU-info-Metric、Memory-info-Metric、Storage-info-Metric
		n.processMetriToNode(node, metric, cacheType) //node:node节点信息
	}

	//TODO 实现上传逻辑到API-server{  ----先放入到NodeStatus当中，然后通过client-go写入到api-server当中
	//直接patch修改node的Spec和Status
	nodePatch1, err := json.Marshal(map[string]interface{}{
		"spec": node.Spec,
	})
	nodePatch2, err2 := json.Marshal(map[string]interface{}{
		"status": node.Status,
	})
	_, err = n.nodesClient.Patch(context.TODO(), node.Name, types.StrategicMergePatchType, nodePatch1, metav1.PatchOptions{})
	if err != nil {
		logs.Errorf("Patch node error-1:%v", err)
	}
	_, err2 = n.nodesClient.Patch(context.TODO(), node.Name, types.StrategicMergePatchType, nodePatch2, metav1.PatchOptions{})
	if err2 != nil {
		logs.Errorf("Patch node error-2:%v", err2)
	}
	// 方法二：patch方法更新node信息--目前这条路有点问题，因为此处获取不到更新完的map
	//if cacheType == "static" {
	//	patchNode, err := json.Marshal(map[string]interface{}{
	//		"spec": map[string]interface{}{
	//			"resource": ,
	//		},
	//	})
	//	_, err = n.nodesClient.Patch(context.TODO(), NodeName, types.StrategicMergePatchType, patchNode, metav1.PatchOptions{})
	//	if err != nil {
	//		logs.Errorf("patch node static info error:%v", err)
	//	}
	//} else {
	//	patchNode, err := json.Marshal(map[string]interface{}{
	//		"status": map[string]interface{}{
	//			"usage": ,
	//		},
	//	})
	//	_, err = n.nodesClient.Patch(context.TODO(), NodeName, types.StrategicMergePatchType, patchNode, metav1.PatchOptions{})
	//	if err != nil {
	//		logs.Errorf("patch node static dynamic error:%v", err)
	//	}
	//}
	// 清空缓存cache
	if cacheType == "static" {
		n.staticCache = make(map[string]collector.Metric)
	} else {
		n.dynamicCache = make(map[string]collector.Metric)
	}
	//logs.Info("Cache uploaded and cleared")
}

// 将Metric存到本地的cache当中
func (n *NodeExporter) processMetri(types string, metric collector.Metric) {
	// TODO：处理Metric
	// TODO: 存入本地Cache或同步到manager中？待定
	// TODO: 将Cache放到nodelet.go中   --应该是放到Nodeexporter当中
	//暂时先实现写入本地Cache
	key := fmt.Sprintf("%s-%v", metric.Item[0].GetName(), metric.Item[0].GetLabels())
	if types == "static" {
		n.staticCacheLock.Lock()
		defer n.staticCacheLock.Unlock()
		n.staticCache[key] = metric // 将 metric 存储到缓存中
	} else {
		n.dynamicCacheLock.Lock()
		defer n.dynamicCacheLock.Unlock()
		n.dynamicCache[key] = metric // 将 metric 存储到缓存中
	}
	//logs.Debugf("Processed metric:%v-%v successfully", key, metric.ToString())
}

// 将metric写入到node引用当中
func (n *NodeExporter) processMetriToNode(node *apis.Node, metric collector.Metric, cacheType string) {
	var itemList []apis.Item
	for _, item := range metric.Item {
		apisItem := convertToApisItem(*item)
		itemList = append(itemList, apisItem)
	}
	part := strings.Split(metric.Item[0].GetName(), ".")[1] // CPU、Storage、Memory

	if cacheType == "static" {
		n.addStaticDataToNodeSpec(node, part, itemList)
	} else {
		n.addDynamicDataToNodeStatus(node, part, itemList)
	}
}

func (n *NodeExporter) addStaticDataToNodeSpec(node *apis.Node, part string, itemList []apis.Item) {
	if node.Spec.Resource == nil {
		node.Spec.Resource = make(map[string][]apis.Item)
	}
	switch part {
	case collector.CpuCollectorName:
		node.Spec.Resource["cpu"] = itemList
	case collector.MemoryCollectorName:
		node.Spec.Resource["memory"] = itemList
	case collector.StorageCollectorName:
		node.Spec.Resource["storage"] = itemList
	}
}

func (n *NodeExporter) addDynamicDataToNodeStatus(node *apis.Node, part string, itemList []apis.Item) {
	if node.Status.Usage == nil {
		node.Status.Usage = make(map[string][]apis.Item)
	}
	switch part {
	case collector.CpuCollectorName:
		node.Status.Usage["cpu"] = itemList
	case collector.MemoryCollectorName:
		node.Status.Usage["memory"] = itemList
	case collector.StorageCollectorName:
		node.Status.Usage["storage"] = itemList
	}
}
func convertToApisItem(item collector.Item) apis.Item {
	return apis.Item{
		Name:   item.GetName(),
		Desc:   item.GetDesc(),
		Labels: item.GetLabels(),
		Values: item.GetValues(),
	}
}
