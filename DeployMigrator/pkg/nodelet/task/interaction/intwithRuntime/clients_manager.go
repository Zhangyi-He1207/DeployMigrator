package intwithRuntime

//
//import (
//	grpc_client "hit.edu/framework/pkg/nodelet/task/interaction/intwithRuntime/grpc-client"
//	"sync"
//
//	"hit.edu/framework/pkg/component-base/logs"
//)
//
//// 管理Taskexporter与所有任务的grpc连接  （grpc客户端）
//type ClientsManager struct {
//	grpcClients map[string]*grpc_client.RuntimeClient
//	mu          sync.RWMutex
//}
//
//func NewClientsManager() *ClientsManager {
//	return &ClientsManager{
//		grpcClients: make(map[string]*grpc_client.RuntimeClient),
//	}
//}
//
//func (cm *ClientsManager) AddRuntimeClientConnection(client *grpc_client.RuntimeClient) bool {
//	cm.mu.Lock()
//	defer cm.mu.Unlock()
//	cm.grpcClients[client.RuntimeID] = client
//	return true
//}
//func (cm *ClientsManager) RemoveRuntimeConnection(runtimeID string) {
//	cm.mu.Lock()
//	defer cm.mu.Unlock()
//	if _, ok := cm.grpcClients[runtimeID]; ok {
//		delete(cm.grpcClients, runtimeID)
//	} else {
//		logs.Error("grpc client not exist, remove runtime client failed")
//	}
//}
//func (cm *ClientsManager) GetRuntimeConnection(runtimeID string) (*grpc_client.RuntimeClient, bool) {
//	cm.mu.RLock()
//	defer cm.mu.RUnlock()
//	if _, ok := cm.grpcClients[runtimeID]; ok {
//		return cm.grpcClients[runtimeID], true
//	}
//	logs.Error("grpc client not exist")
//	return nil, false
//}
