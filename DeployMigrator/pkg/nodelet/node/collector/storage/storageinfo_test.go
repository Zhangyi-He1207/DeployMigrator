package storage

import (
	"fmt"
	"log"
	"testing"
)

func TestStorageForWindows(t *testing.T) {
	done := make(chan struct{})

	go func() {
		provider := NewWindowsStoInfoProvider()
		info, err := provider.GetStorageInfo()
		if err != nil {
			log.Fatal(err)
		}
		for _, storage := range info {
			fmt.Printf("Device: %s\t MountPoint %s\t Total: %.2f GB\t Free: %.2f GB\t Used: %.2f GB\n",
				storage.Device, storage.MountPoint, float64(storage.Total)/1024.0/1024/1024, float64(storage.Free)/1024.0/1024/1024, float64(storage.Used)/1024.0/1024/1024)
		}
		usages, err := provider.GetStorageUsage()
		if err != nil {
			log.Fatal(err)
		}
		for _, usage := range usages {
			fmt.Printf("Usage:%.2f%%\n", usage.Usage)
		}

		done <- struct{}{}
	}()

	<-done
	close(done)
}

func TestStorageForLinux(t *testing.T) {
	done := make(chan struct{})

	go func() {
		provider := NewLinuxStoInfoProvider()
		info, err := provider.GetStorageInfo()
		if err != nil {
			log.Fatal(err)
		}
		for _, storage := range info {
			fmt.Printf("Device: %s\t MountPoint %s\t Total: %.2f GB\t Free: %.2f GB\t Used: %.2f GB\n",
				storage.Device, storage.MountPoint, float64(storage.Total)/1024.0/1024/1024, float64(storage.Free)/1024.0/1024/1024, float64(storage.Used)/1024.0/1024/1024)
		}
		usages, err := provider.GetStorageUsage()
		if err != nil {
			log.Fatal(err)
		}
		for _, usage := range usages {
			fmt.Printf("Usage:%.2f%%\n", usage.Usage)
		}

		done <- struct{}{}
	}()

	<-done
	close(done)
}
