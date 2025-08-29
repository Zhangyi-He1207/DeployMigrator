package storage

import (
	"github.com/shirou/gopsutil/disk"
	"hit.edu/framework/pkg/component-base/logs"
	"os"
)

// 静态存储信息
type StorageInfo struct {
	Device     string //  /dev/sda1（Linux）或 C:（Windows）
	MountPoint string // / 或 /mnt/data（Linux）或 C:\（Windows）。
	Total      uint64 //总存储容量
	Used       uint64 //已使用的存储容量
	Free       uint64 //未使用的存储容量
}

// 动态存储信息
type StorageUsage struct {
	Usage float64
}

type StorageInfoProvider interface {
	GetStorageInfo() ([]*StorageInfo, error)
	GetStorageUsage() ([]*StorageUsage, error)
}
type LinuxStoInfoProvider struct {
}

// 初始化：检测容器环境并设置宿主机路径
func init() {
	if _, err := os.Stat("/host/proc"); err == nil {
		os.Setenv("HOST_PROC", "/host/proc")
		os.Setenv("HOST_SYS", "/host/sys")
	}
}
func NewLinuxStoInfoProvider() *LinuxStoInfoProvider {
	return &LinuxStoInfoProvider{}
}
func (p *LinuxStoInfoProvider) GetStorageInfo() ([]*StorageInfo, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}
	var allStorageInfo []*StorageInfo
	for _, partition := range partitions {
		// 仅监控根目录 "/"和主要存储分区
		if partition.Mountpoint == "/" || partition.Mountpoint == "/home" {
			usage, err := disk.Usage(partition.Mountpoint)
			if err != nil {
				return nil, err
				continue // 跳过出错的分区
			}
			allStorageInfo = append(allStorageInfo, &StorageInfo{
				Device:     partition.Device,
				MountPoint: partition.Mountpoint,
				Total:      usage.Total,
				Used:       usage.Used,
				Free:       usage.Free,
			})
		}
	}
	return allStorageInfo, nil
}

func (p *LinuxStoInfoProvider) GetStorageUsage() ([]*StorageUsage, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}
	var allStorageUsage []*StorageUsage
	for _, partition := range partitions {
		// 仅监控根目录 "/"和主要存储分区
		if partition.Mountpoint == "/" || partition.Mountpoint == "/home" {
			usage, err := disk.Usage(partition.Mountpoint)
			if err != nil {
				return nil, err
				continue // 跳过出错的分区
			}
			allStorageUsage = append(allStorageUsage, &StorageUsage{
				Usage: usage.UsedPercent,
			})
		}
	}
	return allStorageUsage, nil
}

type WindowsStoInfoProvider struct {
}

func NewWindowsStoInfoProvider() *WindowsStoInfoProvider {
	return &WindowsStoInfoProvider{}
}
func (p *WindowsStoInfoProvider) GetStorageInfo() ([]*StorageInfo, error) {
	partitions, err := disk.Partitions(false) //获取系统中的所有磁盘分区,只返回挂载的分区
	if err != nil {
		return nil, err
	}
	var allStorageInfo []*StorageInfo
	for _, partition := range partitions {
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			logs.Info("Warning: failed to get usage for partition %s: %v", partition.Mountpoint, err)
			continue // 跳过出错的分区
		}
		allStorageInfo = append(allStorageInfo, &StorageInfo{
			Device:     partition.Device,
			MountPoint: partition.Mountpoint,
			Total:      usage.Total,
			Used:       usage.Used,
			Free:       usage.Free,
		})
	}
	return allStorageInfo, nil
}

func (p *WindowsStoInfoProvider) GetStorageUsage() ([]*StorageUsage, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}
	var allStorageUsage []*StorageUsage
	for _, partition := range partitions {
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			return nil, err
			continue // 跳过出错的分区
		}
		allStorageUsage = append(allStorageUsage, &StorageUsage{
			Usage: usage.UsedPercent,
		})
	}
	return allStorageUsage, nil
}

func GetPartitionCount() int {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return 0
	}
	return len(partitions)
}
func GetPartitionDeviceName(system string) []string {
	//这里得区分linux和windows，如果是linux，需要获取的是/ 或者/home下面的磁盘名字
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil
	}
	var deviceNames []string
	for _, partition := range partitions {
		if system == "linux" {
			if partition.Mountpoint == "/" || partition.Mountpoint == "/home" {
				deviceNames = append(deviceNames, partition.Device)
			} else {
				deviceNames = append(deviceNames, "")
			}
		} else if system == "windows" {
			deviceNames = append(deviceNames, partition.Device)
		}
	}
	return deviceNames
}
