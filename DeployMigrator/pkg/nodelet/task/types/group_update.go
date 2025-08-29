package types

import apis "hit.edu/framework/pkg/apis/cores"

// 对Group的操作
// Group中的Action可能会更新
// 可能会添加新的Group
type Operation int

const (
	ADD Operation = iota
	Stop
	UPDATE
	KILL
	Restore
)

// 从API Server处获取的Group更新信息
// FIXME: Group可能有多个写入者，如何保证资源的一致性？引入Patch机制? 字段部分更新
type GroupUpdate struct {
	Group *apis.Group
	Op    Operation
}
