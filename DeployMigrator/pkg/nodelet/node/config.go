package node

import "runtime"

// Node Exporter配置
type Config struct {
	// 需要配置的选项包括
	//  系统平台 （例如："linux" 、 "windows"）
	Platform string //这个参数好像可以没有，可以直接在NewXXCollector，直接获取系统平台是Linux还是Windows
	//  需要监控的内容，需要启用哪些Collector  (例如：[]string{"cpu", "memory"})
	EnabledCollectors []string
	//  资源访问方式 （例如：资源访问协议或 API 的 URL）
	ResourceAccessMethod string
	//  节点名称
	NodeName string
	//  节点所属类别
	ClusterCategory string
	// 节点所在集群的集群ID
	LocalClusterID string
	// 节点是否是主节点
	IsMasterNode bool
}

func NewConfig(enabledCollectors []string, resourceAccessMethod, nodeName, clusterCategory string, localClusterID string, isMaster bool) *Config {
	//监测当前系统平台
	platform := runtime.GOOS
	// 如果资源访问方式为空，设置默认值
	if resourceAccessMethod == "" {
		resourceAccessMethod = "local" // 默认资源访问方式
	}
	return &Config{
		Platform:             platform,
		EnabledCollectors:    enabledCollectors,
		ResourceAccessMethod: resourceAccessMethod,
		NodeName:             nodeName,
		LocalClusterID:       localClusterID,
		ClusterCategory:      clusterCategory,
		IsMasterNode:         isMaster,
	}
}
