package nodelet

import (
	"context"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof" // 自动注册 pprof 处理器
	"time"

	"hit.edu/framework/pkg/component-base/logs"

	"hit.edu/framework/pkg/apimachinery/runtime"
	"hit.edu/framework/pkg/apimachinery/runtime/schema"
	"hit.edu/framework/pkg/apimachinery/runtime/serializer"
	apis "hit.edu/framework/pkg/apis/cores"
	"hit.edu/framework/pkg/client-go/clients"
	"hit.edu/framework/pkg/client-go/rest"
	"hit.edu/framework/pkg/nodelet/node"
	"hit.edu/framework/pkg/nodelet/node/collector"
	"hit.edu/framework/pkg/nodelet/task"
)

// Nodelet,部署在每个节点上，管理当前节点上的所有资源
// Node中存在以下类型的exporter
//
//	NodeExporter,监控当前节点的资源情况
//	TaskExporter，部署任务，监控当前任务的执行情况和资源使用情况，分配资源
//	AbilityExporter, 拉起能力
//
// Nodelet管理当前节点上的所有Images,Image类型包括Docker镜像，
// Nodelet管理当前节点上的所有Docker，包括Task Exporter直接拉起和通过Pod间接部署的Docker

type Nodelet struct {
	cfg   *Config //全局config，包含下级的exporter config
	cache map[string]collector.Metric
	// Close this to shut down the resourcelet.
	clientSet      *clients.ClientSet
	StopEverything <-chan struct{}
}

func New(ctx context.Context, configPath string) (*Nodelet, error) {
	cfg := NewConfig(configPath)
	if cfg == nil {
		logs.Error("config is nil")
		return nil, fmt.Errorf("配置初始化失败")
	}
	stopEverything := ctx.Done()
	apiserverHost := cfg.apiserverAddr
	logs.Infof("apiserverHost: %s", apiserverHost)
	clientSet, err := InitClient(apiserverHost)
	if err != nil {
		log.Fatalf("init client failed: %v", err)
	}
	nl := &Nodelet{
		cfg:            cfg,
		clientSet:      clientSet,
		StopEverything: stopEverything,
	}

	// TODO: Nodelet注册, 注册自己的Node资源
	return nl, nil
}

func InitClient(apiserverHost string) (*clients.ClientSet, error) {
	//初始化ClientSet客户端
	scheme := runtime.NewScheme()
	apis.AddToScheme(scheme)
	c := &rest.Config{
		Host:    apiserverHost, //http://localhost:10000   http://suda801.wangwanu.com:11006   //连接api-server
		APIPath: "/apis/resources/v1",
		ContentConfig: rest.ContentConfig{
			AcceptContentTypes: "application/json; charset=UTF-8", //text/plain; charset=UTF-8
			ContentType:        "application/json; charset=UTF-8", //application/json; charset=UTF-8
			GroupVersion: &schema.GroupVersion{
				Group:   "resources",
				Version: "v1",
			},
			NegotiatedSerializer: serializer.NewCodecFactory(scheme),
		},
		UserAgent: "defaultUserAgent",
		Transport: &http.Transport{
			MaxIdleConns:        100,              // 最大空闲连接数
			IdleConnTimeout:     90 * time.Second, // 空闲连接超时时间
			TLSHandshakeTimeout: 10 * time.Second, // TLS 握手超时时间
		},
		Timeout: 7 * 24 * 3600 * time.Second, //改成7200s
	}
	clientSet, err := clients.NewForConfig(c)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize clientSet: %v", err)
	}
	return clientSet, nil
}

func (nl *Nodelet) Run(ctx context.Context) {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	// 构造Node Exporter
	ne, err := node.NewNodeExporter(nl.cfg.nc, nl.clientSet)
	if err != nil {
		panic(err)
	}
	go ne.Run(ctx)

	//构造Task Exporter
	te, err := task.NewTaskExporter(nl.cfg.tc, nl.clientSet, ctx)
	if err != nil {
		panic(err)
	}
	go te.Run(ctx)

	// 构造Ability Exporter

	// TODO：配置不同的Channel

	// TODO: 运行不同的模块,多进程？
	<-ctx.Done()
}
