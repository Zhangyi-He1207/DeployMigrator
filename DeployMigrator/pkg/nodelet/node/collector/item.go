package collector

import "strings"

// 数据存储的基本单元
type Item struct {
	// 收集数据的名称
	name string //格式：name = namespace + '.' + subsystem + '.' + name    node.CPU.Info   node.CPU.Percent

	// 收集数据的描述信息
	desc string //例如：CPU info from /proc/cpuinfo

	// 需要收集的数据的key列表
	labels []string

	// TODO:需要收集的数据的Value
	values map[string]string //每个key对应的value列表

	err error
}

// labels []string --> map[string]string
func NewItem(name string, desc string, labels []string) *Item {
	i := &Item{
		name: name,
		desc: desc,
		// TODO: Labels
		labels: labels, // 初始化 key
		// TODO: Values
		values: make(map[string]string), // 初始化空的 values map
	}
	return i
}

// UpdateValues 用于动态更新值
func (i *Item) UpdateValues(newValues map[string]string) {
	for k, v := range newValues {
		i.values[k] = v
	}
}

// 构造收集数据的名称
// name = namespace + '.' + subsystem + '.' + name
func NewName(namespace string, subsystem string, name string) string {
	if name == "" {
		return ""
	}
	// TODO：格式检查
	return strings.Join([]string{namespace, subsystem, name}, ".")
}

func (i *Item) toString() string {
	str := ""
	for k, v := range i.values {
		str += k + "=" + v + ","
	}
	return str
}
func (i *Item) GetName() string {
	return i.name
}
func (i *Item) GetLabels() []string {
	return i.labels
}
func (i *Item) GetDesc() string {
	return i.desc
}
func (i *Item) GetValues() map[string]string {
	return i.values
}
