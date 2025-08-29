package grpc_client

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"hit.edu/framework/pkg/component-base/logs"
)

func TestClient(t *testing.T) {
	moduleName := "testModule"
	logs.Init(moduleName)
	runtimeId := "runtime-1"
	logs.Infof("runtime for task:%s", runtimeId)
	go TestPullService(t)

	port := "5123"
	client := NewRuntimeClient(port, runtimeId)

	// rpc调用init()
	_, error := client.RunAppInit()
	if error != nil {
		logs.Infof("任务init失败: %e", error)
	}

	// rpc调用start()
	_, error = client.RunAppStart()
	if error != nil {
		logs.Infof("任务启动失败: %e", error)
	}

	// 延迟一定时间
	time.Sleep(2 * time.Second)

	// rpc调用store()
	_, error = client.RunAppStore()
	if error != nil {
		logs.Errorf("任务保存状态失败: %e", error)
	}

	// 延迟一定时间
	time.Sleep(1 * time.Second)

	// rpc调用restore()
	_, error = client.RunAppRestore("{restore data}")
	if error != nil {
		logs.Errorf("任务恢复状态失败: %e", error)
	}

}

func prompt() {
	fmt.Printf("-> Press Return key to continue.")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		break
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

func TestPullService(t *testing.T) {
	logs.Infof("拉起任务")
	cmd := exec.Command("python3", "/home/kcm/py_examples/migration-demo-0116/yolo-runner.py")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	e := cmd.Start()
	CheckError(e)
	cmd.Wait()
}

func CheckError(e error) {
	if e != nil {
		fmt.Println(e)
	}
}
