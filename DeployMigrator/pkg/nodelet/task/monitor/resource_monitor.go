package monitor

/*
type ResourceMonitor struct {
	ctx context.Context
	// 考虑runtimeID作为key
	// specQueue map[string]ProcessStats
	specQueue    sync.Map
	monitorQueue sync.Map

	cGroupQueue sync.Map

	frequency int

	// running任务队列就是监控目标
	groupQueues *group.GroupQueues
	// eventRecorder 记录事件
	recorder recorder.EventRecorder

	groupClient core.GroupInterface
	// taskClient   core.TaskInterface
	// actionClient core.ActionInterface
}

func NewResourceMonitor(groupQueues *group.GroupQueues, recorder recorder.EventRecorder, groupClient core.GroupInterface) *ResourceMonitor {

	return &ResourceMonitor{
		// processQueue: make(map[string]ProcessStats),
		specQueue:    sync.Map{},
		monitorQueue: sync.Map{},
		cGroupQueue:  sync.Map{},
		frequency:    5,
		groupQueues:  groupQueues,
		recorder:     recorder,
		groupClient:  groupClient,
		// stopCh:            make(chan struct{}),
	}
}

func (rm *ResourceMonitor) Start(ctx context.Context) {
	rm.ctx = ctx
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		rm.monitoring()
	}()

	maintainMonitoringQueueDone := make(chan struct{})
	go func() {
		defer close(maintainMonitoringQueueDone)
		select {
		case <-rm.ctx.Done():
			return
		case <-time.After(time.Millisecond * 100):
			// 维护监控队列
			rm.maintainMonitoringQueue()
		}
	}()
	// 等待 Context 取消或所有协程退出
	select {
	case <-rm.ctx.Done():
		logs.Info("ResourceMonitor exiting due to context cancel")
	case <-monitorDone:
	case <-maintainMonitoringQueueDone:
	}
	// 等待所有子协程退出
	// <-depenUpdateDone
	<-monitorDone
	<-maintainMonitoringQueueDone
}

func (rm *ResourceMonitor) maintainMonitoringQueue() {
	runningGroups := rm.groupQueues.GetAllRunning()
	for i := range runningGroups {
		gro := runningGroups[i] //不用再加&&
		// 对于resource monitor ，应当从etcd同步任务信息，还是使用本地缓存的任务信息？
		group, err := rm.groupClient.Get(rm.ctx, gro.Name, meta.GetOptions{})
		if err != nil {
			logs.Errorf("Etcd get group error :%v", err)
		}
		for actionIndex := range group.Spec.Actions { // 遍历group当中的Action
			// action := &group.Spec.Actions[actionIndex]
			actionStatus := &group.Status.ActionStatus[actionIndex]
			for runtimeIndex := range actionStatus.RuntimeStatus {
				runtimeStatus := &actionStatus.RuntimeStatus[runtimeIndex]
				// runtime := &action.Spec.Runtimes[runtimeIndex]
				if runtimeStatus.Phase == "Running" {
					// 如果该任务没有在监控队列中，则添加
					index := ResourceIndex{
						groupName:    gro.Name,
						actionIndex:  actionIndex,
						runtimeIndex: runtimeIndex,
						PID:          runtimeStatus.ProcessId,
					}
					_, loaded := rm.specQueue.LoadOrStore(runtimeStatus.RuntimeID, index)
					if !loaded {
						logs.Infof("Monitoring add : runtimeID:%v processID:%v ", runtimeStatus.RuntimeID, runtimeStatus.ProcessId)
					}
				}
			}
		}
	}

}

func (rm *ResourceMonitor) monitoring() {
	// 对running中的每一个任务，开启一个协程来监控（注意协程的关闭信号）
	var wg sync.WaitGroup
	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-time.After(time.Millisecond * 100):
			rm.specQueue.Range(func(key, value interface{}) bool {
				// 需要类型判定？
				logs.Info(key, value)
				if value, ok := value.(ResourceIndex); ok {
					_, loaded := rm.monitorQueue.LoadOrStore(key, NewProcessMonitor(value))
					if !loaded {
						wg.Add(1)
						go func() {
							defer wg.Done()
							if value, ok := rm.monitorQueue.Load(key); ok {
								processMonitor, _ := value.(ProcessMonitor)
								for {
									stats, err := processMonitor.Collect()
									// stats.CPUPercent
									if err != nil {
										logs.Errorf("监控错误: 进程已不存在%v\n", err)
										break
									}

									resource_data := []apis.ResourceStatus{
										{
											Name:      "cpu",
											Usage:     stats.CPUPercent,
											UsageUnit: apis.CPUPercentage,
										},
										{
											Name:      "memory",
											Usage:     stats.MemoryUsageMB,
											UsageUnit: apis.StorageMB,
										},
									}
									// 数据更新到总线
									rm.record(processMonitor.Index, resource_data)

									logs.Infof("PID: %v  CPU: %.2f%%  Memory: %.2f MB\n",
										stats.PID,
										stats.CPUPercent,
										stats.MemoryUsageMB)

									time.Sleep(time.Duration(rm.frequency) * time.Second)
								}
							} else {
								logs.Errorf("resource monitor : monitorQueue 不存在该任务的进程 %v", key.(string))
							}
						}()
					}
				} else {
					logs.Error("convert fail: ResourceIndex ")
				}
				return true // 返回true继续遍历
			})

		}
	}
	wg.Wait()
}

func (rm *ResourceMonitor) record(index ResourceIndex, resource_data []apis.ResourceStatus) {
	patchGroup, err := json.Marshal([]map[string]interface{}{
		{
			"op":    "replace",
			"path":  "/status/action_status/" + strconv.Itoa(index.actionIndex) + "/status/" + strconv.Itoa(index.runtimeIndex) + "/resources",
			"value": resource_data,
		},
	})
	if err != nil {
		logs.Errorf("Marshal patch group err:%v", err)
	}
	_, err = rm.groupClient.Patch(context.TODO(), index.groupName, types.JSONPatchType, patchGroup, metav1.PatchOptions{})
	if err != nil {
		logs.Errorf("ResourceMonitor : Patch group err :%v", err)
	}

}

func (rm *ResourceMonitor) ResourceLimit(group *apis.Group, action *apis.Action, runtime *apis.Runtime, pid int) error {
	// limitUnit应当从任务信息中解析,作为参数传递
	testLimitUnit := &ResourceLimitUnit{
		cpuLimit:    50,
		memoryLimit: 1024,
	}

	cGroupUnit, err := NewCGroupV2(group.Name)
	if err != nil {
		logs.Error("LimitResource fail", group.Name, action.Name, runtime.Name)
		return err
	}
	rm.cGroupQueue.Store(group.Name, cGroupUnit)
	err = cGroupUnit.AddProcess(pid)
	if err != nil {
		logs.Error(err)
		return err
	}
	err = cGroupUnit.SetCPULimit(testLimitUnit.cpuLimit)
	if err != nil {
		logs.Error(err)
		return err
	}
	err = cGroupUnit.SetMemoryLimit(testLimitUnit.memoryLimit)
	if err != nil {
		logs.Error(err)
		return err
	}
	return nil
}

func (rm *ResourceMonitor) StopResourceLimit(group *apis.Group, action *apis.Action, runtime *apis.Runtime, pid int) error {
	if value, ok := rm.cGroupQueue.Load(group.Name); ok {
		if value, ok := value.(CGroupUnit); ok {
			err := value.Cleanup()
			return err
		}
	}
	err := fmt.Errorf("不存在对应cGroup:%v", group.Name)
	logs.Error(err)
	return err
}

type ResourceLimitUnit struct {
	cpuLimit    int
	memoryLimit int
}

type ResourceIndex struct {
	groupName    string
	actionIndex  int
	runtimeIndex int
	PID          string
}

type ProcessMonitor struct {
	Index       ResourceIndex
	lastCPUTime uint64
	lastSysCPU  uint64
	lastSysIdle uint64
}

type ProcessStats struct {
	PID           string
	CPUPercent    float64
	MemoryUsageMB float64
}

func NewProcessMonitor(index ResourceIndex) *ProcessMonitor {
	// index := ResourceIndex{
	// 	groupName:    groupName,
	// 	actionIndex:  actionIndex,
	// 	runtimeIndex: runtimeIndex,
	// }
	return &ProcessMonitor{
		Index: index,
		// PID:   pid,
	}
}

// 读取进程的 CPU 时间
func (m *ProcessMonitor) getProcessCPUTime() (uint64, error) {
	statPath := fmt.Sprintf("/proc/%v/stat", m.Index.PID)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 17 {
		return 0, fmt.Errorf("invalid stat format")
	}

	utime, _ := strconv.ParseUint(fields[13], 10, 64)
	stime, _ := strconv.ParseUint(fields[14], 10, 64)
	return utime + stime, nil
}

// 读取系统级 CPU 统计
func getSystemCPU() (total, idle uint64, err error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, err
	}

	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("cpu ")) {
			fields := bytes.Fields(line)
			for i := 1; i < len(fields); i++ {
				val, _ := strconv.ParseUint(string(fields[i]), 10, 64)
				if i == 4 { // idle 字段
					idle = val
				}
				total += val
			}
			return
		}
	}
	return 0, 0, fmt.Errorf("cpu stats not found")
}

// 获取进程内存使用 (RSS)
func (m *ProcessMonitor) getProcessMemory() (float64, error) {
	statusPath := fmt.Sprintf("/proc/%v/status", m.Index.PID)
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseFloat(fields[1], 64)
				return kb / 1024, nil // 转换为 MB
			}
		}
	}
	return 0, fmt.Errorf("VmRSS not found")
}

func (m *ProcessMonitor) Collect() (*ProcessStats, error) {
	// 获取进程 CPU 时间
	procTime, err := m.getProcessCPUTime()
	if err != nil {
		return nil, fmt.Errorf("process not found")
	}

	// 获取系统 CPU 统计
	sysTotal, sysIdle, err := getSystemCPU()
	if err != nil {
		return nil, err
	}

	// 计算 CPU 使用率
	var cpuPercent float64
	if m.lastCPUTime > 0 {
		procDelta := procTime - m.lastCPUTime
		totalDelta := sysTotal - m.lastSysCPU
		idleDelta := sysIdle - m.lastSysIdle

		if totalDelta > 0 {
			cpuPercent = 100.0 * float64(procDelta) / float64(totalDelta-idleDelta)
		}
	}

	// 获取内存使用
	memoryMB, err := m.getProcessMemory()
	if err != nil {
		return nil, err
	}

	// 更新状态
	m.lastCPUTime = procTime
	m.lastSysCPU = sysTotal
	m.lastSysIdle = sysIdle

	return &ProcessStats{
		PID:           m.Index.PID,
		CPUPercent:    cpuPercent,
		MemoryUsageMB: memoryMB,
	}, nil
}
*/
