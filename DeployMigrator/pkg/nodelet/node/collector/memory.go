package collector

import (
	"fmt"
	"hit.edu/framework/pkg/component-base/logs"
	meminfo "hit.edu/framework/pkg/nodelet/node/collector/memory"
	"runtime"
)

// TODO: 根据Config配置该选项 ，是否启用MemoryCollector
var enableMemInfo = true //改成冲配置文件当中读取
const MemoryCollectorName = "Memory"

// 收集内存数据  扩展Item有什么用，是为了统一XXCollector中的数据收集的指标，如果说每个Collector中的静态和动态属性都使用的是各自的结构体，不做统一，那么上传数据给上层的话，会有很多类的数据，这里都统一成Item上传给上层，更加简洁
type MemoryCollector struct {
	//静态内存信息
	memoryInfo *Item
	//动态内存占用量
	memoryUsage *Item
	provider    meminfo.MemoryInfoProvider
}

var (
	// 是否启用Memory收集
	// TODO: 根据Config配置该选项
	enableMemoryInfo *bool
)

func init() { //它在包级别的变量初始化之后，自动调用，不需要显式调用 ---也就是说main入口函数导入了collector包，他就会被调用
	// 向NodeCollector注册自身
	RegisterCollector(MemoryCollectorName, enableMemInfo, NewMemoryCollector)
	logs.Info("init memoryCollector==========")
}

func NewMemoryCollector() (Collector, error) {
	// TODO: 完善初始化逻辑
	var provider meminfo.MemoryInfoProvider //注意接口变量不要使用指针，接口类型的指针和具体类型指针是不同的类型，不能直接互换使用
	switch runtime.GOOS {
	case "linux":
		provider = meminfo.NewLinuxMemInfoProvider()
	case "windows":
		provider = meminfo.NewWindowsMemInfoProvider()
	default:
		return nil, fmt.Errorf("unsupported platform")
	}
	// TODO: 适配不同架构，针对 x86 和 ARM 做相应处理 --好像针对不同的架构，暂无区别，故目前先不区分
	m := &MemoryCollector{
		memoryInfo: NewItem(
			NewName(namespace, MemoryCollectorName, "Info"),
			"Free and total memory space",
			// TODO: 设置Labels
			[]string{"Total", "Available"},
		),
		memoryUsage: NewItem(
			NewName(namespace, MemoryCollectorName, "Percent"),
			"dynamic usage",
			[]string{"Usage"},
		),
		provider: provider, // 设置适配器
	}

	return m, nil
}
func (m *MemoryCollector) UpdateStaticInfo(ch chan<- Metric) error {
	if err := m.updateInfo(STATIC); err != nil {
		return err
	}
	// TODO: 收集到数据之后，通过Channel发到主线程
	// 将memory 信息逐个发送到通道
	var memoryInfo []*Item
	memoryInfo = append(memoryInfo, m.memoryInfo)
	ch <- NewMetric(memoryInfo)
	return nil
}

func (m *MemoryCollector) UpdateDynamicInfo(ch chan<- Metric) error {
	if err := m.updateInfo(DYNAMIC); err != nil {
		return err
	}
	// TODO: 收集到数据之后，通过Channel发到主线程
	// 将memory 信息逐个发送到通道
	var memoryUsage []*Item
	memoryUsage = append(memoryUsage, m.memoryUsage)
	ch <- NewMetric(memoryUsage)
	return nil
}

func (m *MemoryCollector) updateInfo(infoType string) error {
	// 实际收集数据
	// 获取Memory
	if infoType == STATIC {
		meminfo, err := m.provider.GetMemoryInfo()
		if err != nil {
			return err
		}
		// TODO: 更新静态Memory信息
		m.memoryInfo.values["Total"] = fmt.Sprintf("%d", meminfo.Total)
		m.memoryInfo.values["Available"] = fmt.Sprintf("%d", meminfo.Available)
	} else {
		memUsage, err := m.provider.GetMemoryUsage()
		if err != nil {
			return err
		}
		m.memoryUsage.values["Usage"] = fmt.Sprintf("%.2f", memUsage.Usage)
	}
	return nil
}
