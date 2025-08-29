package nodelet

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	run "hit.edu/framework/pkg/apimachinery/runtime"
	"hit.edu/framework/pkg/apimachinery/runtime/schema"
	"hit.edu/framework/pkg/apimachinery/runtime/serializer"
	apis "hit.edu/framework/pkg/apis/cores"
	"hit.edu/framework/pkg/client-go/rest"
	"hit.edu/framework/pkg/component-base/logs"
	"hit.edu/framework/pkg/nodelet/node"
	"hit.edu/framework/pkg/nodelet/task"
	"hit.edu/framework/test/etcd_sync/active/clients"
	cross_core "hit.edu/framework/test/etcd_sync/active/clients/typed/core"
)

type Config struct {
	// TODO：整合子模块
	nc            *node.Config
	tc            *task.Config
	apiserverAddr string
}

// IsRunningInPod 判断是否运行在Pod中
func IsRunningInPod() bool {
	// 通过检查Kubernetes特有的环境变量来判断
	isPod := os.Getenv("KUBERNETES_SERVICE_HOST") != ""
	logs.Infof("isPod的值为：%v", isPod)
	return isPod
	//方法二：
	//检查默认的 ServiceAccount 路径
	//_, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token")
	//return !os.IsNotExist(err)
}

func NewConfig(configPath string) *Config {
	var config *FrameworkConfig
	var err error

	if IsRunningInPod() { // 为pod环境
		config = &FrameworkConfig{
			NodeName:        os.Getenv("NODE_NAME"),
			ClusterCategory: os.Getenv("CLUSTER_CATEGORY"),
			LocalClusterID:  os.Getenv("LOCAL_CLUSTER_ID"),
			ApiServerAddr:   os.Getenv("API_SERVER_HOST"),
		}
		//if addr := os.Getenv("API_SERVER_ADDR"); addr != "" {
		//	if port := os.Getenv("API_SERVER_PORT"); port != "" {
		//		config.ApiServerAddr = addr
		//		if p, err := strconv.Atoi(port); err != nil {
		//			config.ApiServerPort = p
		//		}
		//	}
		//}
	} else { // 为二进制环境,从本地读取配置文件
		// logs.Info("framework-conf ", configPath)
		//没有指定配置文件位置，则去默认位置加载
		if configPath == "" {
			logs.Info("ConfigPath is empty, using default")
			fileName := "frameworkConf.yaml"
			// 获取当前文件绝对路径
			_, currentFilePath, _, _ := runtime.Caller(0)
			// 计算项目根目录路径
			projectRoot := filepath.Join(filepath.Dir(currentFilePath), "..", "..")
			// 构建配置文件的绝对路径
			configPath = filepath.Join(projectRoot, fileName)
		}
		logs.Infof("configPath:%v", configPath)
		// 验证路径有效性
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			logs.Errorf("配置文件不存在于：%s", configPath)
		}
		config, err = LoadConfig(configPath)
		if err != nil {
			logs.Errorf("frameworkConf.yaml load failed: %e", err)
			config = &FrameworkConfig{} // 使用空配置
		}
	}
	// 获取必要配置项（环境变量优先于配置文件）
	nodeName := GetNodeName(config)
	LocalClusterID := GetLocalClusterID(config)
	clusterCategory := GetClusterCategory(config)
	address := GetAPIServerHost(config)
	isMaster := GetIsMaster(config)
	fileRegisrty := GetFileRegistry(config)
	logs.Infof("address:%v==============", address)
	logs.Infof("fileRegisrty:%v==============", fileRegisrty)
	taskTargetMap, groupTargetMap, actionTargetMap, runtimeTargetMap, err := BuildTargetMap(config)
	if err != nil {
		logs.Errorf("targetMap build failed")
		return nil
	}
	dir, port := GetWasmConfig(config)
	return &Config{
		//需要修改成从配置文件中读取内容 例如：config.json
		nc:            node.NewConfig([]string{"CPU", "Memory", "Storage"}, "", nodeName, clusterCategory, LocalClusterID, isMaster),
		tc:            task.NewConfig(nodeName, taskTargetMap, groupTargetMap, actionTargetMap, runtimeTargetMap, dir, port, fileRegisrty),
		apiserverAddr: address,
	}
}
func GetIsMaster(config *FrameworkConfig) bool {
	if config.IsMasterNode == true {
		return config.IsMasterNode
	}
	return false
}

func GetNodeName(config *FrameworkConfig) string {
	//if nodeName := os.Getenv("NODE_NAME"); nodeName != "" {
	//	return nodeName
	//}
	if config.NodeName != "" {
		return config.NodeName
	}
	return "CloudNode1"
}
func GetLocalClusterID(config *FrameworkConfig) string {
	//if nodeName := os.Getenv("NODE_NAME"); nodeName != "" {
	//	return nodeName
	//}
	if config.LocalClusterID != "" {
		return config.LocalClusterID
	}
	return ""
}
func GetClusterCategory(config *FrameworkConfig) string {
	//if clusterCategory := os.Getenv("CLUSTER_CATEGORY"); clusterCategory != "" {
	//	return clusterCategory
	//}
	if config.ClusterCategory != "" {
		return config.ClusterCategory
	}
	return "Cloud"
}
func GetAPIServerHost(config *FrameworkConfig) string {
	//if addr := os.Getenv("API_SERVER_HOST"); addr != "" {
	//	return addr
	//}
	if config.ApiServerAddr != "" {
		return config.ApiServerAddr
	}
	return "http://localhost:10000"
}
func GetFileRegistry(config *FrameworkConfig) string {
	if config.FileRegistryAddr != "" {
		return config.FileRegistryAddr
	}
	return "http://localhost:8919"
}

//func GetNameSpace(config *FrameworkConfig) string {
//	//if addr := os.Getenv("API_SERVER_HOST"); addr != "" {
//	//	return addr
//	//}
//	if config.ApiServerAddr != "" {
//		return config.ApiServerAddr
//	}
//	return "http://localhost:10000"
//}

func GetWasmConfig(config *FrameworkConfig) (string, string) {
	dir := "/home/public/tmp/wasm"
	port := "8080"
	if config.WasmConfig.WasmToolchainDir != "" {
		dir = config.WasmConfig.WasmToolchainDir
	}
	if config.WasmConfig.WasmRuntimePort != "" {
		port = config.WasmConfig.WasmRuntimePort
	}
	return dir, port
}

// 定义完整的配置结构体
type FrameworkConfig struct {
	EtcdPort         int    `yaml:"EtcdPort"`
	ApiServerAddr    string `yaml:"ApiServerAddr"`
	FileRegistryAddr string `yaml:"FileRegistryAddr"`
	NodeName         string `yaml:"NodeName"`
	ClusterCategory  string `yaml:"ClusterCategory"`
	LocalClusterID   string `yaml:"LocalClusterID"`
	IsMasterNode     bool   `yaml:"IsMasterNode"`
	OtherCluster     map[string]struct {
		ClusterID string `yaml:"ClusterID"`
		ClusterIP string `yaml:"ClusterIP"`
	} `yaml:"OtherCluster"`
	WasmConfig struct {
		WasmToolchainDir string `yaml:"WasmToolchainDir"`
		WasmRuntimePort  string `yaml:"WasmRuntimePort"`
	} `yaml:"WasmConfig"`
	Namespace string `yaml:"Namespace"`
	// 注意YAML字段名与结构体的映射
}

// 创建目标映射的函数
func BuildTargetMap(config *FrameworkConfig) (map[string]cross_core.TaskInterface, map[string]cross_core.GroupInterface, map[string]cross_core.ActionInterface, map[string]cross_core.RuntimeInterface, error) {
	taskTargetMap := make(map[string]cross_core.TaskInterface)
	groupTargetMap := make(map[string]cross_core.GroupInterface)
	actionTargetMap := make(map[string]cross_core.ActionInterface)
	runtimeTargetMap := make(map[string]cross_core.RuntimeInterface)
	// 参数校验
	if config == nil || len(config.OtherCluster) == 0 {
		return taskTargetMap, groupTargetMap, actionTargetMap, runtimeTargetMap, nil
	}
	// 获取本地集群ID（环境变量优先）
	localID := getEnvWithFallback("LOCAL_CLUSTER_ID", config.LocalClusterID)
	if localID == "" {
		return nil, nil, nil, nil, fmt.Errorf("missing local cluster ID")
	}
	// 优先从环境变量获取集群配置
	envClusters := parseClusterEnv()
	var clusters map[string]struct {
		ClusterID string
		ClusterIP string
	}
	if len(envClusters) > 0 { // 使用环境变量配置
		clusters = envClusters
	} else { // 使用本地读取yaml文件
		// 安全转换文件配置的集群数据
		clusters = make(map[string]struct {
			ClusterID string
			ClusterIP string
		})
		for key, c := range config.OtherCluster {
			clusters[key] = struct {
				ClusterID string
				ClusterIP string
			}{
				ClusterID: c.ClusterID,
				ClusterIP: c.ClusterIP,
			}
		}
	}
	// 服务发现参数校验
	for key, cluster := range clusters {
		// 参数有效性检查
		if cluster.ClusterID == "" || cluster.ClusterIP == "" {
			return nil, nil, nil, nil, fmt.Errorf("无效的集群配置: %s", key)
		}
		// 这里演示参数组合，请根据实际需求调整
		logs.Infof("localID:%v,otherDomain.ClusterID:%v,otherDomain.ClusterIP:%v", localID, cluster.ClusterID, cluster.ClusterIP)
		clientSet, err := InitCrossClient(localID, cluster.ClusterID, cluster.ClusterIP)
		if err != nil {
			logs.Errorf("init client failed: %v", err)
		}
		taskTarget := clientSet.Core().Tasks("test")
		groupTarget := clientSet.Core().Groups("test")
		actionTarget := clientSet.Core().Actions("test")
		runtimeTarget := clientSet.Core().Runtimes("test")

		taskTargetMap[cluster.ClusterID] = taskTarget
		groupTargetMap[cluster.ClusterID] = groupTarget
		actionTargetMap[cluster.ClusterID] = actionTarget
		runtimeTargetMap[cluster.ClusterID] = runtimeTarget
	}
	return taskTargetMap, groupTargetMap, actionTargetMap, runtimeTargetMap, nil
}

// 配置加载函数
func LoadConfig(path string) (*FrameworkConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config FrameworkConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析YAML失败: %v", err)
	}

	return &config, nil
}

// 环境变量解析函数
func parseClusterEnv() map[string]struct {
	ClusterID string
	ClusterIP string
} {
	clusters := make(map[string]struct {
		ClusterID string
		ClusterIP string
	})

	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "CLUSTER") {
			continue
		}

		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		// 解析形如 CLUSTER1_CLUSTERID 的变量名
		keyParts := strings.Split(parts[0], "_")
		if len(keyParts) != 3 || (keyParts[2] != "CLUSTERID" && keyParts[2] != "CLUSTERIP") {
			continue
		}

		clusterNum := strings.TrimPrefix(keyParts[0], "CLUSTER")
		clusterKey := "cluster" + clusterNum

		// 初始化或更新集群配置
		cfg := clusters[clusterKey]
		switch keyParts[2] {
		case "CLUSTERID":
			cfg.ClusterID = parts[1]
		case "CLUSTERIP":
			cfg.ClusterIP = parts[1]
		}
		clusters[clusterKey] = cfg
	}

	return clusters
}

// 辅助函数：带默认值的环境变量读取
func getEnvWithFallback(key string, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
func InitCrossClient(localID, clusterID, clusterIP string) (*clients.ClientSet, error) {
	scheme := run.NewScheme()
	apis.AddToScheme(scheme)
	c := &rest.Config{
		//Host: "http://broker.registry-svc.test.svc.clusterset.local:3001/forward?target=",
		//Host:    "http://localhost:10000",
		Host:    "http://" + localID + ".registry-svc.test.svc.clusterset.local:3001",
		APIPath: "/apis/resources/v1",
		ContentConfig: rest.ContentConfig{
			AcceptContentTypes: "application/json; charset=UTF-8", //text/plain; charset=UTF-8
			ContentType:        "application/json; charset=UTF-8", //application/json; charset=UTF-8
			GroupVersion: &schema.GroupVersion{
				Group:   "resources",
				Version: "v1",
			},
			NegotiatedSerializer: serializer.NewCodecFactory(scheme),
			TargetURL:            "http://" + clusterIP + ":10000",
			FlowType:             "etcd",
			ClusterID:            clusterID,
		},
		UserAgent: "defaultUserAgent",
		Transport: &http.Transport{
			MaxIdleConns:        10000,            // 最大空闲连接数
			IdleConnTimeout:     90 * time.Second, // 空闲连接超时时间
			TLSHandshakeTimeout: 10 * time.Second, // TLS 握手超时时间
		},
		Timeout: 1000 * time.Second,
	}
	//创建ClientSet
	clientSet, err := clients.NewForConfig(c)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize clientSet: %v", err)
	}
	return clientSet, nil
}
