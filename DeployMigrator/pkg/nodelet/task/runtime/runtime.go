package runtime

import (
	"context"
	"fmt"
	"sync"

	apis "hit.edu/framework/pkg/apis/cores"
	"hit.edu/framework/pkg/client-go/util/manager"
	"hit.edu/framework/pkg/component-base/logs"
	"hit.edu/framework/pkg/nodelet/events/eventbus"
	"hit.edu/framework/pkg/nodelet/task/interaction/intwithRuntime/pool"
	"hit.edu/framework/pkg/nodelet/task/runtime/binary"
	"hit.edu/framework/pkg/nodelet/task/runtime/command"
	"hit.edu/framework/pkg/nodelet/task/runtime/net"
)

type Runtime interface {
	Run(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error
	Kill(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error
	Stop(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error
	Restore(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error
	CheckRuntimeStatus(group *apis.Group, action *apis.Action, runtime *apis.Runtime) (string, error)
	StoreData(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) string
	RestoreData(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error
	StartRuntime(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error
	InitRuntime(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error
	StopRuntime(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error
}

type RuntimeManager struct {
	runtimes         map[apis.RuntimeType]Runtime
	eventbus         *eventbus.EventBus
	clientsManager   *manager.Manager
	mu               sync.Mutex
	pool             *pool.ConnectionPool
	NodeName         string
	wasmToolchainDir string
	wasmRuntimePort  string
	// 为了合理关闭wasm运行时进程
	ctx context.Context
}

func NewRuntimeManager(ctx context.Context, bus *eventbus.EventBus, clientsManager *manager.Manager, nodeName string, wasmToolchainDir string, wasmRuntimePort string) *RuntimeManager {
	return &RuntimeManager{
		runtimes: make(map[apis.RuntimeType]Runtime),
		eventbus: bus,
		//recorder: recorder,
		//deviceClient: deviceClient,
		//actionClient: actionClient,
		//groupClient:  groupClient,
		clientsManager:   clientsManager,
		pool:             pool.NewConnectionPool(),
		NodeName:         nodeName,
		wasmToolchainDir: wasmToolchainDir,
		wasmRuntimePort:  wasmRuntimePort,
		ctx:              ctx,
	}
}

var _ Runtime = &RuntimeManager{}

func (rm *RuntimeManager) GetRuntime(rt apis.RuntimeType) Runtime {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if rt == apis.ByService || rt == apis.ByPod || rt == apis.ByDeployment {
		rt = apis.ByK8s
	}

	runtime, exists := rm.runtimes[rt]
	if !exists {
		switch rt {
		case apis.ByBinary: //将预编译的二进制文件部署
			//TODO
			runtime = binary.NewBinaryRuntime()
			break
		case apis.ByCommand: //任务作为系统命令执行
			runtime = command.NewCommandRuntime(rm.clientsManager, rm.eventbus, rm.pool)
			break
		case apis.ByNet: //基于网络的部署
			runtime = net.NewNetRuntime()
			break
		default:
			logs.Info("unknown runtime type:%s", rt)
			break
		}
		rm.runtimes[rt] = runtime
	}
	return runtime
}

func (rm *RuntimeManager) Run(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error {
	return rm.GetRuntime(runtime.Spec.Type).Run(group, action, runtime, actionSpecName, runtimeSpecName)
}
func (rm *RuntimeManager) Kill(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error {
	if rm == nil {
		logs.Error("runtime manager is nil")
		return fmt.Errorf("RuntimeManager is not initialized")
	}
	return rm.GetRuntime(runtime.Spec.Type).Kill(group, action, runtime, actionSpecName, runtimeSpecName)
}
func (rm *RuntimeManager) Stop(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error {
	return rm.GetRuntime(runtime.Spec.Type).Stop(group, action, runtime, actionSpecName, runtimeSpecName)
}
func (rm *RuntimeManager) Restore(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error {
	return rm.GetRuntime(runtime.Spec.Type).Restore(group, action, runtime, actionSpecName, runtimeSpecName)
}

func (rm *RuntimeManager) CheckRuntimeStatus(group *apis.Group, action *apis.Action, runtime *apis.Runtime) (string, error) {
	//TODO
	return "", nil
}
func (rm *RuntimeManager) StoreData(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) string {
	return rm.GetRuntime(runtime.Spec.Type).StoreData(group, action, runtime, actionSpecName, runtimeSpecName)
}
func (rm *RuntimeManager) RestoreData(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error {
	return rm.GetRuntime(runtime.Spec.Type).RestoreData(group, action, runtime, actionSpecName, runtimeSpecName)
}
func (rm *RuntimeManager) StartRuntime(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error {
	return rm.GetRuntime(runtime.Spec.Type).StartRuntime(group, action, runtime, actionSpecName, runtimeSpecName)
}
func (rm *RuntimeManager) InitRuntime(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error {
	return rm.GetRuntime(runtime.Spec.Type).InitRuntime(group, action, runtime, actionSpecName, runtimeSpecName)
}
func (rm *RuntimeManager) StopRuntime(group *apis.Group, action *apis.Action, runtime *apis.Runtime, actionSpecName, runtimeSpecName string) error {
	return rm.GetRuntime(runtime.Spec.Type).StopRuntime(group, action, runtime, actionSpecName, runtimeSpecName)
}
