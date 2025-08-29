package task

import (
	cross_core "hit.edu/framework/test/etcd_sync/active/clients/typed/core"
)

type Config struct {
	// Node Name
	NodeName         string
	taskTargetMap    map[string]cross_core.TaskInterface
	groupTargetMap   map[string]cross_core.GroupInterface
	actionTargetMap  map[string]cross_core.ActionInterface
	runtimeTargetMap map[string]cross_core.RuntimeInterface
	wasmToolchainDir string
	wasmRuntimePort  string
	FileRegistryAddr string
}

func NewConfig(name string, tasktargetMap map[string]cross_core.TaskInterface, grouptargetMap map[string]cross_core.GroupInterface, actiontargetMap map[string]cross_core.ActionInterface, runtimetargetMap map[string]cross_core.RuntimeInterface, wasmToolchainDir string, wasmRuntimePort string, fileRegistry string) *Config {
	return &Config{
		NodeName:         name,
		taskTargetMap:    tasktargetMap,
		groupTargetMap:   grouptargetMap,
		actionTargetMap:  actiontargetMap,
		runtimeTargetMap: runtimetargetMap,
		wasmToolchainDir: wasmToolchainDir,
		wasmRuntimePort:  wasmRuntimePort,
		FileRegistryAddr: fileRegistry,
	}
}
