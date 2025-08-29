package collector

// 每次收集的数据及格式
//
//	type Metric interface {
//		//
//		Item() *Item
//	}
type Metric struct {
	//
	Item []*Item
}

// TODO: 定义Metric格式，可以兼容各类资源监控的需求
// TODO: Metrc的定义函数
func NewMetric(item []*Item) Metric {
	// TODO:
	return Metric{
		item,
	}
}

func (m Metric) ToString() string {
	return m.Item[0].toString()
}
