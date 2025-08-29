package process

import (
	"hit.edu/framework/pkg/component-base/logs"
	"os/exec"
	"sync"
)

type ProcessManager struct {
	lock           sync.Mutex
	processes      map[string]*exec.Cmd // 任务名称映射到 exec.Cmd 对象
	successProcess map[string]*exec.Cmd
}

func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		processes:      make(map[string]*exec.Cmd),
		successProcess: make(map[string]*exec.Cmd),
	}
}

// 管理进程的添加
func (pm *ProcessManager) AddProcess(name string, cmd *exec.Cmd) bool {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	if _, exists := pm.processes[name]; exists {
		logs.Errorf("process name:%s already exists", name)
		return false
	}
	pm.processes[name] = cmd
	return true
}

// 管理进程的删除
func (pm *ProcessManager) RemoveProcess(name string) bool {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	if _, exists := pm.processes[name]; !exists {
		logs.Errorf("process name:%s not found-1", name)
		return false
	}
	delete(pm.processes, name)
	return true
}

func (pm *ProcessManager) RemoveProcessFromSuccess(name string) bool {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	if _, exists := pm.successProcess[name]; !exists {
		logs.Errorf("process name:%s not found-2", name)
		return false
	}
	delete(pm.successProcess, name)
	return true
}

func (pm *ProcessManager) GetProcess(name string) (*exec.Cmd, bool) {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	cmd, ok := pm.processes[name]
	return cmd, ok
}

// 这个方法原来的写法错误，会造成死锁
func (pm *ProcessManager) MoveProcessToSucess(name string) bool {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	// 检查是否存在该进程
	cmd, exists := pm.processes[name]
	if !exists {
		logs.Errorf("process name:%s not found-3", name)
		return false
	}
	// 将进程移到到成功列表
	pm.successProcess[name] = cmd
	// 同时删除Process当中的进程
	delete(pm.processes, name)
	return true
}
