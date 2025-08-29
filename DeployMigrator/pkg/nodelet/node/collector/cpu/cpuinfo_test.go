package cpu

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"testing"
	"time"
)

func TestForWindows(t *testing.T) {
	done := make(chan struct{})

	go func() {
		provider := NewWindowsCPUInfoProvider()
		infos, err := provider.GetCPUInfo()
		if err != nil {
			log.Fatal(err)
		}
		for _, info := range infos {
			fmt.Printf("CPU Model: %s \t CPU cores:%v \t CPU BaseFreq:%vGHz\n", info.ModelName, info.Cores, info.BaseFreq)
		}

		for i := 0; i < 5; i++ {
			freq, err := provider.GetCPUFreq()
			if err != nil {
				log.Fatal(err)
			}
			//for i, util := range freq.Utilization { //遍历所有核心，打印利用率
			//	fmt.Printf("CPU-%d Utilization: %.2f%%\n", i, util)
			//}
			for _, aveUtil := range freq.AveUtil { //遍历所有核心的平均利用率
				fmt.Printf("--%d--CPU AveUtil: %.2f%%-----\n", i, aveUtil)
			}
		}
		// 模拟程序结束后通过 channel 发送通知
		done <- struct{}{}
	}()

	// 模拟一些 CPU 密集型任务
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 10000000; i++ {
		_ = math.Sqrt(rand.Float64())
	}

	<-done
	close(done)
}

// 和Windows大差不差
func TestForLinux(t *testing.T) {
	done := make(chan struct{})

	go func() {
		provider := NewLinuxCPUIInfoProvider() //difference
		infos, err := provider.GetCPUInfo()
		if err != nil {
			log.Fatal(err)
		}
		for _, info := range infos {
			fmt.Printf("CPU Model: %s \t CPU cores:%v \t CPU BaseFreq:%vGHz\n", info.ModelName, info.Cores, info.BaseFreq)
		}

		for i := 0; i < 5; i++ {
			freq, err := provider.GetCPUFreq()
			if err != nil {
				log.Fatal(err)
			}
			//for i, util := range freq.Utilization { //遍历所有核心，打印利用率
			//	fmt.Printf("CPU-%d Utilization: %.2f%%\n", i, util)
			//}
			for _, aveUtil := range freq.AveUtil { //遍历所有核心的平均利用率
				fmt.Printf("--%d--CPU AveUtil: %.2f%%-----\n", i, aveUtil)
			}
		}
		// 模拟程序结束后通过 channel 发送通知
		done <- struct{}{}
	}()

	// 模拟一些 CPU 密集型任务
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 10000000; i++ {
		_ = math.Sqrt(rand.Float64())
	}

	<-done
	close(done)
}
