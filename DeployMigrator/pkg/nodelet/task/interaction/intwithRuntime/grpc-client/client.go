package grpc_client

import (
	"context"
	"errors"
	"hit.edu/framework/pkg/nodelet/task/interaction/intwithRuntime/pool"
	"sync"
	"time"

	"hit.edu/framework/pkg/component-base/logs"
	pb "hit.edu/framework/pkg/nodelet/task/interaction/intwithRuntime/proto"
)

type RuntimeClient struct {
	ServerIPAndPort string
	connPool        *pool.ConnectionPool
	grpcClient      pb.RuntimeIntentClient
	mu              sync.Mutex // 保护连接状态
}

func NewRuntimeClient(port string, pool *pool.ConnectionPool) *RuntimeClient {
	client := &RuntimeClient{ServerIPAndPort: "127.0.0.1:" + port, connPool: pool}
	// 首次创建时尝试预连接
	if ok := client.checkConnection1(); !ok {
		logs.Error("初次连接 gRPC 服务端失败")
	}
	//for {
	//	success := client.checkConnection()
	//	if success {
	//		break
	//	}
	//	logs.Debug("try to connect grpc server")
	//	time.Sleep(time.Millisecond * 500)
	//}
	return client
}
func NewK8sRuntimeClient(service, port string, pool *pool.ConnectionPool) *RuntimeClient {
	client := &RuntimeClient{ServerIPAndPort: service + ":" + port, connPool: pool}
	if ok := client.checkConnection1(); !ok {
		logs.Error("初次连接 gRPC 服务端失败")
	}
	//for {
	//	success := client.checkConnection()
	//	if success {
	//		break
	//	}
	//	logs.Debug("try to connect grpc server")
	//	time.Sleep(time.Millisecond * 500)
	//}
	return client
}

//	func (c *RuntimeClient) checkConnection() bool {
//		connState := true
//		if c.conn == nil {
//			conn, err := grpc.Dial(c.ServerIPAndPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
//			if err != nil {
//				logs.Debugf("failed to connect to grpc server:%v", err)
//				connState = false
//			} else {
//				c.conn = conn
//				c.grpcClient = pb.NewRuntimeIntentClient(conn)
//			}
//		}
//		return connState
//	}
func (c *RuntimeClient) checkConnection1() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 直接连接（无需退避）
	conn, err := c.connPool.GetConnWithRetry(c.ServerIPAndPort, 500) // 最多重试3次
	if err != nil {
		logs.Infof("Failed to connect to gRPC server: %v", err)
		return false
	}
	if c.grpcClient == nil {
		c.grpcClient = pb.NewRuntimeIntentClient(conn)
		logs.Info("gRPC client initialized")
	}
	return true
}

// rpc远程调用init
func (c *RuntimeClient) RunAppInit() (result *pb.Result, err error) {
	logs.Infof("RunAppInit()")
	if !c.checkConnection1() {
		return &pb.Result{}, errors.New("runAppInit: grpc connection failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logs.Info("==============Init=========")
	result, err = c.grpcClient.Init(ctx, &pb.InitIntent{})
	logs.Trace("==============Init=========")
	for err != nil {
		time.Sleep(time.Millisecond * 100) //kcm:这里的延时会影响迁移指标，建议删除
		logs.Debug("retry to runAppInit")
		result, err = c.grpcClient.Init(ctx, &pb.InitIntent{})
	}
	logs.Infof("runAppInit: result:%v", result)
	return result, nil
}

// rpc远程调用服务端启动应用
func (c *RuntimeClient) RunAppStart() (result *pb.Result, err error) {
	logs.Infof("RunAppStart()==========")
	if !c.checkConnection1() {
		logs.Error("grpc connection fail===================")
		return &pb.Result{}, errors.New("runAppStart: grpc connection failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logs.Info("==============Start=========")
	result, err = c.grpcClient.Start(ctx, &pb.StartIntent{})
	logs.Trace("==============Start=========")
	for err != nil {
		time.Sleep(time.Millisecond * 100) //kcm:这里的延时会影响迁移指标，建议删除
		logs.Info("retry to runAppStart")
		result, err = c.grpcClient.Start(ctx, &pb.StartIntent{})
	}
	logs.Infof("runAppStart: result:%v", result)
	return result, nil
}

// rpc远程调用服务端保存应用状态
func (c *RuntimeClient) RunAppStore() (answer string, err error) {
	logs.Infof("RunAppStore()")
	if !c.checkConnection1() {
		return "", errors.New("runAppStore: grpc connection failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logs.Info("==============Store=========")
	result, err := c.grpcClient.Store(ctx, &pb.StoreIntent{})
	logs.Trace("==============Store=========")
	for err != nil {
		logs.Debug("retry to runAppStore")
		result, err = c.grpcClient.Store(ctx, &pb.StoreIntent{})
	}
	logs.Infof("runAppStore: result:%v", result)
	return result.Data, nil
}

// rpc远程调用服务端保存应用状态
func (c *RuntimeClient) RunAppRestore(keyStatus string) (result *pb.Result, err error) {
	logs.Infof("RunAppRestore()")
	if !c.checkConnection1() {
		return &pb.Result{}, errors.New("runAppRestore: grpc connection failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	data := &pb.Data{
		Name: "restoreData",
		Data: keyStatus,
	}
	logs.Info("==============Restore=========")
	result, err = c.grpcClient.Restore(ctx, &pb.RestoreIntent{Data: []*pb.Data{data}})
	logs.Trace("==============Restore=========")
	for err != nil {
		logs.Debug("retry to runAppRestore")
		result, err = c.grpcClient.Restore(ctx, &pb.RestoreIntent{Data: []*pb.Data{data}})
	}
	logs.Infof("runAppRestore: result:%v", result)
	return result, nil
}

// rpc远程调用服务端启动应用
func (c *RuntimeClient) RunAppStop() (result *pb.Result, err error) {
	logs.Infof("StopAppStart()")
	if !c.checkConnection1() {
		return &pb.Result{}, errors.New("runAppStop: grpc connection failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err = c.grpcClient.Stop(ctx, &pb.StopIntent{})
	for err != nil {
		time.Sleep(time.Millisecond * 100)
		logs.Debug("retry to runAppStop")
		result, err = c.grpcClient.Start(ctx, &pb.StartIntent{})
	}
	logs.Infof("runAppStop: result:%v", result)
	return result, nil
}

//func (c *RuntimeClient) close() {
//	defer c.conn.Close()
//}
