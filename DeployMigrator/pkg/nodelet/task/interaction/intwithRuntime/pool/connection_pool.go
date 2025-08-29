package pool

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"hit.edu/framework/pkg/component-base/logs"
	"sync"
	"time"
)

type ConnectionPool struct {
	pool   sync.Map // key=address, value=*grpc.ClientConn
	locker sync.Mutex
}

func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		pool:   sync.Map{},
		locker: sync.Mutex{},
	}
}

//	func (cp *ConnectionPool) GetConn(address string) (*grpc.ClientConn, error) {
//		// 双重检查锁定模式
//		if conn, ok := cp.pool.Load(address); ok {
//			return conn.(*grpc.ClientConn), nil
//		}
//
//		cp.locker.Lock()
//		defer cp.locker.Unlock()
//
//		if conn, ok := cp.pool.Load(address); ok {
//			return conn.(*grpc.ClientConn), nil
//		}
//
//		conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
//		if err != nil {
//			return nil, err
//		}
//
//		cp.pool.Store(address, conn)
//		return conn, nil
//	}
func (cp *ConnectionPool) GetConn(address string) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond) // 连接超时   100 20
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		address,
		grpc.WithBlock(), // 阻塞直到连接就绪
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %v", address, err)
	}
	return conn, nil
}

// connection_pool.go 增加带重试的GetConnWithRetry方法
//
//	func (cp *ConnectionPool) GetConnWithRetry(address string, maxRetries int, backoff time.Duration) (*grpc.ClientConn, error) {
//		var conn *grpc.ClientConn
//		var err error
//
//		for i := 0; i < maxRetries; i++ {
//			conn, err = cp.GetConn(address)
//			if err == nil {
//				return conn, nil
//			}
//			logs.Errorf("连接尝试 %d 失败: %v", i+1, err) // 明确错误日志
//			time.Sleep(backoff)
//			//backoff = time.Duration(float64(backoff) * 1.5)
//		}
//		return nil, fmt.Errorf("after %d retries: %v", maxRetries, err)
//	}
func (cp *ConnectionPool) GetConnWithRetry(address string, maxRetries int) (*grpc.ClientConn, error) {
	var conn *grpc.ClientConn
	var err error

	for i := 0; i < maxRetries; i++ {
		startTime := time.Now()
		conn, err = cp.GetConn(address)
		if err == nil {
			logs.Infof("Connected to %s in %v", address, time.Since(startTime))
			return conn, nil
		}

		logs.Tracef("Connection attempt %d failed (耗时 %v): %v", i+1, time.Since(startTime), err)
		if i < maxRetries-1 {
			time.Sleep(15 * time.Millisecond) // 递增间隔：50ms, 100ms... time.Sleep(time.Duration(i+1) * 50 * time.Millisecond)   10mm
		}
	}
	return nil, fmt.Errorf("after %d retries: %v", maxRetries, err)
}

// 修改 GetConn 实现健康检查
//func (cp *ConnectionPool) GetConn(address string) (*grpc.ClientConn, error) {
//	if conn, ok := cp.pool.Load(address); ok {
//		if conn.(*grpc.ClientConn).GetState() == connectivity.Ready {
//			return conn.(*grpc.ClientConn), nil
//		}
//	}
//
//	cp.mu.Lock()
//	defer cp.mu.Unlock()
//
//	// 双重检查
//	if conn, ok := cp.pool.Load(address); ok {
//		return conn.(*grpc.ClientConn), nil
//	}
//
//	conn, err := grpc.Dial(address,
//		grpc.WithTransportCredentials(insecure.NewCredentials()),
//		grpc.WithKeepaliveParams(keepalive.ClientParameters{
//			Time:    30 * time.Second,
//			Timeout: 10 * time.Second,
//		}))
//	if err != nil {
//		return nil, err
//	}
//
//	cp.pool.Store(address, conn)
//	return conn, nil
//}
