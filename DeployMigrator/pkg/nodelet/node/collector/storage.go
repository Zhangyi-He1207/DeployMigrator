package collector

import (
	"fmt"
	"hit.edu/framework/pkg/component-base/logs"
	stoinfo "hit.edu/framework/pkg/nodelet/node/collector/storage"
	"runtime"
)

// TODO: 根据Config配置该选项 ，是否启用StorageCollector
var enableStoInfo = true //改成冲配置文件当中读取，这个参数可以去掉，采用Controller当中根据配置文件注册XXController即可
const StorageCollectorName = "Storage"

// 收集存储数据  扩展Item有什么用，是为了统一XXCollector中的数据收集的指标，如果说每个Collector中的静态和动态属性都使用的是各自的结构体，不做统一，那么上传数据给上层的话，会有很多类的数据，这里都统一成Item上传给上层，更加简洁
type storageCollector struct {
	// 静态storage信息
	// 磁盘名、挂载点、总容器、已使用量、空闲量等
	storageInfo []*Item
	// 动态Storage信息
	// 运行时每个磁盘的占用量
	storageUsage []*Item
	provider     stoinfo.StorageInfoProvider
}

var (
// 是否启用CPUInfo收集
// TODO: 根据Config配置该选项
// enableStoInfo *bool
)

func init() { //它在包级别的变量初始化之后，自动调用，不需要显式调用 ---也就是说main入口函数导入了collector包，他就会被调用
	// 向NodeCollector注册自身
	RegisterCollector(StorageCollectorName, enableStoInfo, NewStorageCollector)
	logs.Info("Init StorageCollector")
}
func NewStorageCollector() (Collector, error) {
	var provider stoinfo.StorageInfoProvider
	var system = runtime.GOOS
	switch system {
	case "linux":
		provider = stoinfo.NewLinuxStoInfoProvider()
	case "windows":
		provider = stoinfo.NewWindowsStoInfoProvider()
	default:
		return nil, fmt.Errorf("unsupported platform")
	}
	s := &storageCollector{
		provider: provider,
	}

	for _, deviceName := range stoinfo.GetPartitionDeviceName(system) {
		if deviceName == "" {
			continue
		}
		s.storageInfo = append(s.storageInfo, NewItem(
			NewName(namespace, StorageCollectorName, "Info"),
			fmt.Sprintf("%v-Info", deviceName),
			[]string{"Device", "MountPoint", "Total", "Used", "Free"},
		))
		s.storageUsage = append(s.storageUsage, NewItem(
			NewName(namespace, StorageCollectorName, "Percent"),
			fmt.Sprintf("%v-Usage", deviceName),
			[]string{"Usage"},
		))
	}
	return s, nil
}
func (s *storageCollector) UpdateStaticInfo(ch chan<- Metric) error {
	if err := s.updateInfo(STATIC); err != nil {
		return err
	}
	//for _, info := range s.storageInfo {
	//	ch <- NewMetric(info)
	//}
	ch <- NewMetric(s.storageInfo)
	return nil
}

func (s *storageCollector) UpdateDynamicInfo(ch chan<- Metric) error {
	if err := s.updateInfo(DYNAMIC); err != nil {
		return err
	}
	//for _, info := range s.storageUsage {
	//	ch <- NewMetric(info)
	//}
	ch <- NewMetric(s.storageUsage)
	return nil
}

func (s *storageCollector) updateInfo(infoType string) error {
	if infoType == STATIC {
		storageInfos, err := s.provider.GetStorageInfo()
		if err != nil {
			return err
		}
		for i, info := range storageInfos {
			s.storageInfo[i].values["Device"] = info.Device
			s.storageInfo[i].values["MountPoint"] = info.MountPoint
			s.storageInfo[i].values["Total"] = fmt.Sprintf("%d", info.Total)
			s.storageInfo[i].values["Used"] = fmt.Sprintf("%d", info.Used)
			s.storageInfo[i].values["Free"] = fmt.Sprintf("%d", info.Free)
		}
	} else {
		storageUsage, err := s.provider.GetStorageUsage()
		if err != nil {
			return err
		}
		for i, info := range storageUsage {
			s.storageUsage[i].values["Usage"] = fmt.Sprintf("%.2f", info.Usage)
		}
	}
	return nil
}
