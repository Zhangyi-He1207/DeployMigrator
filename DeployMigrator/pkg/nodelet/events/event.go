package events

import apis "hit.edu/framework/pkg/apis/cores"

// 定义Nodelet中所有的事件

// Image相关

// Container相关

// Ability相关

// Wasm相关

// command相关
//type ActionStatusUpdateEvent struct {
//	//ActionID      string // Action的ID，用于标识该Action
//	//Belongs       apis.IDRef
//	Phase         apis.Phase            // 可选字段，表示Action的生命周期阶段
//	Resources     []apis.ResourceStatus // 可选字段，表示当前Action的资源状态
//	Devices       []apis.DeviceStatus   // 可选字段，表示当前Action的设备状态
//	Data          []apis.DataStatus     // 可选字段，表示当前Action的数据状态
//	Scenes        []apis.SceneStatus    // 可选字段，表示当前Action的场景状态
//	RuntimeStatus []apis.RuntimeStatus  // 可选字段，表示当前Action的Runtime状态
//	Results       []apis.Result         // 可选字段，表示任务执行结果
//	StartTime     apis.Time
//	FinishTime    apis.Time
//	LastTime      apis.Time // 可选字段，表示最后更新时间
//}

// RuntimeStatusUpdateEvent 用于表示Runtime状态更新的事件
//type RuntimeStartPhaseEvent struct {
//	ProcessId string
//	Phase     apis.Phase
//	StartAt   apis.Time
//	LastTime  apis.Time  // 可选字段，表示最后更新时间
//}

type ActionResourceEvent struct {
	Resources []apis.ResourceStatus
	Devices   []apis.DeviceStatus
	Data      []apis.DataStatus
	Scenes    []apis.SceneStatus
	LastTime  apis.Time // 可选字段，表示最后更新时间
}

//// RuntimeStatusUpdateEvent 用于表示Runtime状态更新的事件
//type RuntimeStartPhaseEvent struct {
//	Group     *apis.Group
//	Action    *apis.Action
//	Runtime   *apis.Runtime
//	ProcessId string
//	Phase     apis.Phase
//	StartAt   apis.Time
//	LastTime  apis.Time // 可选字段，表示最后更新时间
//}
//
//// RuntimeStatusUpdateEvent 用于表示Runtime状态更新的事件
//type RuntimeEndPhaseEvent struct {
//	Group    *apis.Group
//	Action   *apis.Action
//	Runtime  *apis.Runtime
//	Phase    apis.Phase
//	FinishAt apis.Time
//	LastTime apis.Time // 可选字段，表示最后更新时间
//}

// RuntimeStatusUpdateEvent 用于表示Runtime状态更新的事件
type RuntimeStartPhaseEvent1 struct {
	GroupName       string
	GroupNamespace  string
	ActionSpecName  string
	RuntimeSpecName string
	ProcessId       string
	Phase           apis.Phase
	StartAt         apis.Time
	LastTime        apis.Time // 可选字段，表示最后更新时间
}

// RuntimeStatusUpdateEvent 用于表示Runtime状态更新的事件
type RuntimeEndPhaseEvent1 struct {
	GroupName       string
	GroupNamespace  string
	ActionSpecName  string
	RuntimeSpecName string
	Phase           apis.Phase
	FinishAt        apis.Time
	LastTime        apis.Time // 可选字段，表示最后更新时间
}
type ActionEndPhaseEvent1 struct {
	GroupName      string
	GroupNamespace string
	ActionSpecName string
	Phase          apis.Phase
	FinishAt       apis.Time
	LastTime       apis.Time
}

const (
	ReadyToMigrate = "ReadyToMigrate"
)

// Wasm event reason list
const (
	CreatedWasm         = "Created"
	StartedWasm         = "Started"
	FailedToCreateWasm  = "Failed"
	FailedToStartWasm   = "Failed"
	KillingWasm         = "Killing"
	PreemptWasm         = "Preempting"
	BackOffStartWasm    = "BackOff"
	ExceededGracePeriod = "ExceededGracePeriod"
)

// Command event reason list
const (
	DeployCheck            = "DeployCheck"
	DependencyError        = "DependencyError"
	Running                = "Running"
	StartedCommand         = "Started"
	StoredCommand          = "Stored"
	RestoredCommand        = "Restored"
	FailedToStartCommand   = "Failed"
	KillingCommand         = "Killing"
	KilledCommand          = "Killed"
	PreemptCommand         = "Preempting"
	BackOffStartCommand    = "BackOff"
	TriggerLocalMigration  = "TriggerLocalMigration"
	TriggerCrossMigration  = "TriggerCrossMigration"
	ExecuteSuccessfully    = "ExecuteSuccessfully"
	ExecuteFailed          = "ExecuteFailed"
	SelectOtherDomain      = "SelectDomain"
	GroupRunError          = "GroupRunError"
	ScheduledToOtherDomain = "ScheduledToOtherDomain"
	ExecuteDiscard         = "ExecuteDiscard"
	Migrating              = "Migrating"
	Migrated               = "Migrated"
)

// type EventCode string

// 事件代码常量定义（使用4位数字编码）
const (
	EvtCodeDeployCheck            apis.EventCode = "31"
	EvtCodeDependencyError        apis.EventCode = "311"
	EvtCodeRunning                apis.EventCode = "32"
	EvtCodeStartedCommand         apis.EventCode = "321"
	EvtCodeStoredCommand          apis.EventCode = "323"
	EvtCodeRestoredCommand        apis.EventCode = "324"
	EvtCodeFailedToStartCommand   apis.EventCode = "322"
	EvtCodeKillingCommand         apis.EventCode = "35"
	EvtCodeKilledCommand          apis.EventCode = "351"
	EvtCodeTriggerLocalMigration  apis.EventCode = "362"
	EvtCodeTriggerCrossMigration  apis.EventCode = "363"
	EvtCodeExecuteSuccessfully    apis.EventCode = "33"
	EvtCodeExecuteFailed          apis.EventCode = "34"
	EvtCodeSelectOtherDomain      apis.EventCode = "38"
	EvtCodeGroupRunError          apis.EventCode = "38"
	EvtCodeScheduledToOtherDomain apis.EventCode = "38"
	EvtCodeExecuteDiscard         apis.EventCode = "37"
	EvtCodeMigrating              apis.EventCode = "36"
	EvtCodeMigrated               apis.EventCode = "361"
)

var ReasonToCodeForDeploy = map[string]apis.EventCode{
	DeployCheck:            EvtCodeDeployCheck,
	DependencyError:        EvtCodeDependencyError,
	Running:                EvtCodeRunning,
	StartedCommand:         EvtCodeStartedCommand,
	FailedToStartCommand:   EvtCodeFailedToStartCommand,
	StoredCommand:          EvtCodeStoredCommand,
	RestoredCommand:        EvtCodeRestoredCommand,
	ExecuteSuccessfully:    EvtCodeExecuteSuccessfully,
	ExecuteFailed:          EvtCodeExecuteFailed,
	KillingCommand:         EvtCodeKillingCommand,
	KilledCommand:          EvtCodeKilledCommand,
	Migrating:              EvtCodeMigrating,
	Migrated:               EvtCodeMigrated,
	TriggerLocalMigration:  EvtCodeTriggerLocalMigration,
	TriggerCrossMigration:  EvtCodeTriggerCrossMigration,
	ExecuteDiscard:         EvtCodeExecuteDiscard,
	SelectOtherDomain:      EvtCodeSelectOtherDomain,
	GroupRunError:          EvtCodeGroupRunError,
	ScheduledToOtherDomain: EvtCodeScheduledToOtherDomain,
}
