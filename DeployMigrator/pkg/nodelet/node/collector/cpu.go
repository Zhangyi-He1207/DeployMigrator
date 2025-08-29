package collector

import (
	"fmt"
	"hit.edu/framework/pkg/component-base/logs"
	cpuinfo "hit.edu/framework/pkg/nodelet/node/collector/cpu"

	"runtime"
)

// TODO: 根据Config配置该选项 ，是否启用CPUCollector
var enableCPUInfo = true //改成冲配置文件当中读取
const CpuCollectorName = "CPU"

// 收集CPU数据  扩展Item有什么用，是为了统一XXCollector中的数据收集的指标，如果说每个Collector中的静态和动态属性都使用的是各自的结构体，不做统一，那么上传数据给上层的话，会有很多类的数据，这里都统一成Item上传给上层，更加简洁
type CpuCollector struct {
	// 静态CPU信息
	// CPU的型号、核心数、基准频率等 这里存服务器上每个CPU的静态信息，故为数组
	cpuInfos []*Item
	// 动态CPU信息
	// 运行时CPU的动态频率，这里觉得收集每个CPU的频率可能不是很高效，收集服务器上所有CPU的率用率的平均值（单个值）就够了，但是后期有收集各个CPU动态利用率的需求，可以修改
	cpuFreq  *Item
	provider cpuinfo.CPUInfoProvider
}

func init() { //它在包级别的变量初始化之后，自动调用，不需要显式调用 ---也就是说main入口函数导入了collector包，他就会被调用
	// 向NodeCollector注册自身  ---这个enableCPUInfo参数可以不要，目前的代码逻辑：在collector.go当中的NodeCollector拿到配置文件当中需要注册XXCollector的New函数，并执行New方法调用  对于cpu.go\memory.go\storage.go都有init方法，都会自动将New方法注册到NodeColector当中，但是调不调用还是看NodeCollector
	RegisterCollector(CpuCollectorName, enableCPUInfo, NewCPUCollector)
	logs.Info("Registe CPUCollector==========")
}

func NewCPUCollector() (Collector, error) {
	// TODO: 完善初始化逻辑
	var provider cpuinfo.CPUInfoProvider //注意接口变量不要使用指针，接口类型的指针和具体类型指针是不同的类型，不能直接互换使用
	switch runtime.GOOS {
	case "linux":
		provider = cpuinfo.NewLinuxCPUIInfoProvider()
	case "windows":
		provider = cpuinfo.NewWindowsCPUInfoProvider()
	default:
		return nil, fmt.Errorf("unsupported platform")
	}
	// TODO: 适配不同架构，针对 x86 和 ARM 做相应处理 --好像针对不同的架构，暂无区别，故目前先不区分
	c := &CpuCollector{ //初始化CpuCollector
		cpuFreq: NewItem( //1、首先初始化CPU的动态Item信息
			NewName(namespace, CpuCollectorName, "Percent"),
			"CPU dynamic utilization",
			// TODO: 设置Labels
			[]string{"AveUtil"},
		),
		provider: provider, // 2、初始化适配器
	}
	cpuInfos, err := provider.GetCPUInfo() //调用收集方法先看一下有几个CPU，这样好初始化CPU数组来存放每个CPU的信息
	if err != nil {
		return nil, err
	}
	for i, _ := range cpuInfos {
		c.cpuInfos = append(c.cpuInfos, NewItem( // 3、接着初始化每个 CPU 核心的静态信息 Item
			NewName(namespace, CpuCollectorName, "Info"), //名称
			fmt.Sprintf("CPU-%v-Info", i),                //描述信息
			[]string{"ModelName", "Core", "BaseFreq"},    //Labels
		))
	}
	return c, nil
}

// 收集静态信息
func (c *CpuCollector) UpdateStaticInfo(ch chan<- Metric) error {
	if err := c.updateInfo(STATIC); err != nil { // 收集完信息，并已放入到了CpuCollector中的数组（Item格式）
		return err
	}
	// TODO: 收集到数据之后，通过Channel发到主线程
	//ch <- NewMetric(nil, 0.0, "a")
	// 将静态 CPU 信息逐个发送到通道
	//for _, info := range c.cpuInfos {
	//	ch <- NewMetric(info) //将Item格式的数据转换成Metric格式，并传入到管道中
	//}
	ch <- NewMetric(c.cpuInfos) //将Item格式的数据转换成Metric格式，并传入到管道中
	return nil

}

func (c *CpuCollector) UpdateDynamicInfo(ch chan<- Metric) error {
	if err := c.updateInfo(DYNAMIC); err != nil { // 收集完信息，并已放入到了CpuCollector中的数组（Item格式）
		return err
	}

	// TODO: 收集到数据之后，通过Channel发到主线程
	var cpuFreq []*Item
	cpuFreq = append(cpuFreq, c.cpuFreq)
	ch <- NewMetric(cpuFreq) //将Item格式的数据转换成Metric格式，并传入到管道中
	return nil

}

// 收集信息，并放入CpuCollector中的数组当中
func (c *CpuCollector) updateInfo(infoType string) error {
	// 实际收集数据
	// 获取CPU数据
	if infoType == STATIC {
		cpuInfos, err := c.provider.GetCPUInfo() //返回手机到的静态CPU数据【数据结构为：[]*CPUInfo】   c.provider主要是选择是linux还是Windows
		if err != nil {
			return err
		}
		// TODO: 更新静态CPU信息
		for i, info := range cpuInfos { //把[]*CPUInfo数据结构当中的信息转移到CpuCollector中的Item当中（为了方便向上层传数据）
			c.cpuInfos[i].values["ModelName"] = info.ModelName
			c.cpuInfos[i].values["Core"] = fmt.Sprintf("%d", info.Cores)
			c.cpuInfos[i].values["BaseFreq"] = fmt.Sprintf("%.2f GHz", info.BaseFreq)
		}
	} else {
		cpuFreq, err := c.provider.GetCPUFreq()
		if err != nil {
			return err
		}
		// TODO: 更新动态CPU信息
		//for i, freq := range cpuFreq.Utilization {
		//	c.cpuFreq.values[fmt.Sprintf("core_%d", i)] = fmt.Sprintf("%.2f GHz", freq)
		//}
		c.cpuFreq.values["AveUtil"] = fmt.Sprintf("%.2f", cpuFreq.AveUtil[0]) //是所有CPU核心的平均值，故只有一个值
	}
	return nil
}
