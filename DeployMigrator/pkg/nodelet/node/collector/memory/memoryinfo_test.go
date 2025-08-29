package memory

import (
	"fmt"
	"log"
	"testing"
)

func TestMemoryForWindows(t *testing.T) {
	done := make(chan struct{})

	go func() {
		provider := NewWindowsMemInfoProvider()
		info, err := provider.GetMemoryInfo()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Total Memory: %.2f GB\t Available Memory: %.2f GB\n",
			float64(info.Total)/1024.0/1024/1024, float64(info.Available)/1024.0/1024/1024)

		usage, err := provider.GetMemoryUsage()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Used Memory percent: %.2f%% \n", usage.Usage)

		done <- struct{}{}
	}()

	<-done
	close(done)
}

func TestMemoryForLinux(t *testing.T) {
	done := make(chan struct{})

	go func() {
		provider := NewLinuxMemInfoProvider()
		info, err := provider.GetMemoryInfo()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Total Memory: %.2f GB\t Available Memory: %.2f GB\n",
			float64(info.Total)/1024.0/1024/1024, float64(info.Available)/1024.0/1024/1024)

		usage, err := provider.GetMemoryUsage()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Used Memory percent: %.2f%% \n", usage.Usage)

		done <- struct{}{}
	}()

	<-done
	close(done)
}
