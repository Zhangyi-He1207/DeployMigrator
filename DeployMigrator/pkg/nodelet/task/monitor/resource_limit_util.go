package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type CGroupUnit struct {
	Path string // cgroup路径 如：/sys/fs/cgroup/mycgroup
	name string
	pids []int
}

func NewCGroupV2(name string) (*CGroupUnit, error) {
	path := filepath.Join("/sys/fs/cgroup", name)
	if err := os.Mkdir(path, 0755); err != nil {
		return nil, fmt.Errorf("创建cgroup失败: %v", err)
	}
	return &CGroupUnit{Path: path, name: name}, nil
}

// 设置CPU限制，单位为%
func (c *CGroupUnit) SetCPULimit(percent int) error {
	// 格式："MAX 100000" 表示100% CPU
	value := fmt.Sprintf("%d %d", percent*1000, 100000)
	return os.WriteFile(filepath.Join(c.Path, "cpu.max"), []byte(value), 0644)
}

// 设置内存限制，单位MB
func (c *CGroupUnit) SetMemoryLimit(mb int) error {
	// 先禁用swap
	if err := os.WriteFile(filepath.Join(c.Path, "memory.swap.max"), []byte("0"), 0644); err != nil {
		return err
	}
	// 设置内存上限
	bytes := strconv.Itoa(mb * 1024 * 1024)
	return os.WriteFile(filepath.Join(c.Path, "memory.max"), []byte(bytes), 0644)
}

// 添加进程到cgroup
func (c *CGroupUnit) AddProcess(pid int) error {
	c.pids = append(c.pids, pid)
	return os.WriteFile(filepath.Join(c.Path, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0644)
}

// 清理cgroup
func (c *CGroupUnit) Cleanup() error {
	return os.RemoveAll(c.Path)
}
