package cpu

import (
	"fmt"
	"github.com/shirou/gopsutil/cpu"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TODO: 可以使用第三方工具 -已完成
// TODO: 不同平台，不同架构CPU下的数据收集 -已完成
// TODO: 可以使用Adapter模式
// 平台包括 Windows,Linux
// 架构包括 x86,arm等 --CPU获取对于架构不一样，获取方式没有影响
// 需要收集的数据包括 CPU处理器型号，CPU核心数，CPU频率

// CPU静态信息
type CPUInfo struct {
	ModelName  string  //CPU型号
	Cores      int     //CPU核心数
	BaseFreq   float64 //存储CPU的频率（基准速度）
	PhysicalID string  // 新增字段标识物理 CPU
}

// CPU动态利用率
type CPUUtil struct {
	//Frequencies [][]float64
	//Utilization    []float64 //存储所有核心的利用率,服务器上一共有多少个核心，就返回多少个值，不分区CPU
	AveUtil []float64 //存储所有核心的利用率的平均值，即一个平均值  这里为了统一cpu.Percent的统一返回值，使用float数组 扩展：cpu.Percent(,true)是返回每个CPU的利率用，故为数组  cpu.Percent(,false)返回所用CPU的平均率用率一个数，这个方法做了统一故返回一个数组
}

func init() {
	if _, err := os.Stat("/host/proc"); err == nil {
		os.Setenv("HOST_PROC", "/host/proc")
		os.Setenv("HOST_SYS", "/host/sys")
	}
}

// （pass-目前均使用的是gopsutil） 尝试在Linux平台下使用gopsutil，windows平台下使用wmi
// CPU信息提供者接口
type CPUInfoProvider interface {
	GetCPUInfo() ([]*CPUInfo, error)
	GetCPUFreq() (*CPUUtil, error)
}

// linux 平台的实现
type LinuxCPUIInfoProvider struct {
}

func NewLinuxCPUIInfoProvider() *LinuxCPUIInfoProvider {
	return &LinuxCPUIInfoProvider{}
}

// 获取CPU静态信息
//
//	func (l LinuxCPUIInfoProvider) GetCPUInfo() ([]*CPUInfo, error) {
//		// 使用 gopsutil 或 /proc/cpuinfo 读取 CPU 信息   其实gopsutil底层就是去查 /proc/
//		// TODO: 完成获取 CPU 信息的逻辑-已完成
//		cpuinfos, err := cpu.Info()
//		if err != nil {
//			return nil, err
//		}
//		// 获取所有CPU的信息
//		var allCPUInfo []*CPUInfo
//		for _, info := range cpuinfos {
//			allCPUInfo = append(allCPUInfo, &CPUInfo{
//				ModelName: info.ModelName,
//				Cores:     int(info.Cores),
//				BaseFreq:  info.Mhz / 1000,
//			})
//		}
//		return allCPUInfo, nil
//	}

// 获取宿主机 /proc 的实际路径（适配容器环境）
func getHostProcPath() string {
	// 优先使用环境变量 HOST_PROC
	if hostProc := os.Getenv("HOST_PROC"); hostProc != "" {
		return hostProc
	}
	// 默认路径
	return "/proc"
}
func (l LinuxCPUIInfoProvider) GetCPUInfo() ([]*CPUInfo, error) {
	// 动态获取宿主机 /proc 路径
	procPath := filepath.Join(getHostProcPath(), "cpuinfo")
	data, err := ioutil.ReadFile(procPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cpuinfo: %v", err)
	}

	blocks := strings.Split(string(data), "\n\n")
	seen := make(map[string]struct{})
	var allCPUInfo []*CPUInfo

	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}

		var modelName, physicalID string
		var cores int
		var mhz float64

		lines := strings.Split(block, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "model name") {
				modelName = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			} else if strings.HasPrefix(line, "cpu cores") {
				coresStr := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				fmt.Sscanf(coresStr, "%d", &cores)
			} else if strings.HasPrefix(line, "cpu MHz") {
				mhzStr := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				fmt.Sscanf(mhzStr, "%f", &mhz)
			} else if strings.HasPrefix(line, "physical id") {
				physicalID = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
		}

		if modelName != "" && physicalID != "" {
			key := physicalID
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				allCPUInfo = append(allCPUInfo, &CPUInfo{
					ModelName:  modelName,
					Cores:      cores,
					BaseFreq:   mhz / 1000,
					PhysicalID: physicalID,
				})
			}
		}
	}

	return allCPUInfo, nil
}

// 获取CPU动态平均利用率
func (l LinuxCPUIInfoProvider) GetCPUFreq() (*CPUUtil, error) {
	// TODO: 获取 CPU 利用率 -使用 gopsutil-已完成
	avePercent, err := cpu.Percent(1000*time.Millisecond, false) //(每 100 毫秒获取一次)
	if err != nil {
		return nil, err
	}
	return &CPUUtil{
		AveUtil: avePercent,
	}, nil
}

// Windows 平台的实现
type WindowsCPUInfoProvider struct{}

func NewWindowsCPUInfoProvider() *WindowsCPUInfoProvider {
	return &WindowsCPUInfoProvider{}
}

func (w *WindowsCPUInfoProvider) GetCPUInfo() ([]*CPUInfo, error) {
	// 使用 gopsutil 或 /proc/cpuinfo 读取 CPU 信息
	// TODO: 完成获取 CPU 信息的逻辑
	cpuinfos, err := cpu.Info()
	if err != nil {
		return nil, err
	}
	// 获取所有CPU的信息
	var allCPUInfo []*CPUInfo
	for _, info := range cpuinfos {
		allCPUInfo = append(allCPUInfo, &CPUInfo{
			ModelName: info.ModelName,
			Cores:     int(info.Cores),
			BaseFreq:  info.Mhz / 1000,
		})
	}
	return allCPUInfo, nil
}

func (w *WindowsCPUInfoProvider) GetCPUFreq() (*CPUUtil, error) {
	// TODO: 获取 CPU 频率信息 使用 gopsutil
	//avePercent, err := cpu.Percent(1000*time.Millisecond, false)
	//if err != nil {
	//	return nil, err
	//}
	//return &CPUUtil{
	//	//Utilization:    percent,
	//	AveUtil: avePercent,
	//}, nil

	// 每1秒获取一次所有核心的CPU利用率
	avePercent, err := cpu.Percent(5*time.Second, true)
	if err != nil {
		return nil, err
	}

	// 过滤掉利用率为0的核心
	var totalUtil float64
	var count int
	for _, percent := range avePercent {
		if percent > 0 { // 忽略利用率为0的核心
			totalUtil += percent
			count++
		}
	}

	// 计算非零核心的平均利用率
	if count == 0 {
		// 如果所有核心的利用率都为0，返回0
		return &CPUUtil{AveUtil: []float64{0}}, nil
	}

	averageUtil := totalUtil / float64(count)

	return &CPUUtil{
		AveUtil: []float64{averageUtil}, // 返回总的平均值
	}, nil

	// 获取每个核心的利用率
	//avePercent, err := cpu.Percent(1*time.Second, true)
	//if err != nil {
	//	fmt.Println("Error:", err)
	//	return nil, err
	//}
	//
	//// 打印每个核心的 CPU 利用率
	//fmt.Println("每个核心的 CPU 利用率：")
	//total := 0.0
	//for i, percent := range avePercent {
	//	fmt.Printf("Core-%d: %.2f%%\n", i, percent)
	//	total += percent
	//}
	//
	//// 计算加权平均 CPU 利用率（按核心个数计算）
	//averageUtilization := total / float64(len(avePercent))
	//fmt.Printf("系统总 CPU 利用率: %.2f%%\n", averageUtilization)
	//return &CPUUtil{
	//	AveUtil: []float64{averageUtilization}, // 返回总的平均值
	//}, nil
}

// 暂时没用到，获取cpu的数量
//func GetCpuNum() int {
//	info, err := cpu.Info()
//	if err != nil {
//		logs.Error("get cpu info failed, err:", err)
//		return 0
//	}
//	return len(info)
//}

// 暂时没用到，获取cpu的数量
//func getPhysicalCPUsLinux() (int, error) {
//	file, err := os.Open("/proc/cpuinfo")
//	if err != nil {
//		return 0, err
//	}
//	defer file.Close()
//
//	physicalCPUs := make(map[string]struct{})
//	scanner := bufio.NewScanner(file)
//	for scanner.Scan() {
//		line := scanner.Text()
//		if strings.HasPrefix(line, "physical id") {
//			fields := strings.Fields(line)
//			if len(fields) == 3 {
//				physicalCPUs[fields[2]] = struct{}{}
//			}
//		}
//	}
//	return len(physicalCPUs), scanner.Err()
//}
