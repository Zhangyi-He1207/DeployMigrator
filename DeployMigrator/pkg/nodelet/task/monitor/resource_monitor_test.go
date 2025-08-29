package monitor_test

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"hit.edu/framework/pkg/apimachinery/runtime"
	"hit.edu/framework/pkg/apimachinery/runtime/schema"
	"hit.edu/framework/pkg/apimachinery/runtime/serializer"
	apis "hit.edu/framework/pkg/apis/cores"
	metav1 "hit.edu/framework/pkg/apis/meta"
	"hit.edu/framework/pkg/client-go/clients"
	"hit.edu/framework/pkg/client-go/clients/typed/core"
	"hit.edu/framework/pkg/client-go/rest"
	"hit.edu/framework/pkg/client-go/tools/recorder"
	"hit.edu/framework/pkg/component-base/logs"
	"hit.edu/framework/pkg/nodelet/task/group"
	"hit.edu/framework/pkg/nodelet/task/monitor"
	// "hit.edu/framework/pkg/nodelet/task/runtime"
)

func TestForResourceLimit(t *testing.T) {
	moduleName := "TestForBroadcaster"
	logs.Init(moduleName)
	// ctx := context.Background()
	scheme := runtime.NewScheme()
	clientset := initClientSet(scheme)

	// Client-Go配置
	groupClient := clientset.Core().Groups("test")
	eventClient := clientset.Core().Events("test")
	// 全局事件组件的配置
	eventBroadcaster := recorder.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(context.Background(), &core.EventSinkImpl{Interface: eventClient})

	recorder := eventBroadcaster.NewRecorder(scheme, "TaskExporter")

	// Manager配置 group
	groupManager := group.NewGroupManager()
	// queue_manager
	groupQueues := group.NewGroupQueues(groupManager)

	resource_monitor := monitor.NewResourceMonitor(groupQueues, recorder, groupClient)

	// 模拟进程
	cmd := exec.Command("stress", "--cpu", "1", "--vm", "1", "--vm-bytes", "200M")
	if err := cmd.Start(); err != nil {
		panic(err)
	}

	resource_monitor.ResourceLimit(group1, &group1.Spec.Actions[0], &group1.Spec.Actions[0].Spec.Runtimes[0], cmd.Process.Pid)
	defer resource_monitor.StopResourceLimit(group1, &group1.Spec.Actions[0], &group1.Spec.Actions[0].Spec.Runtimes[0], cmd.Process.Pid)
	prompt()
}

func TestForCGroupUnit(t *testing.T) {
	// 1. 创建cgroup
	cg, err := monitor.NewCGroupV2("myapp")
	if err != nil {
		panic(err)
	}
	defer cg.Cleanup()

	// 2. 设置资源限制
	if err := cg.SetCPULimit(50); err != nil { // 限制50% CPU
		panic(err)
	}
	if err := cg.SetMemoryLimit(100); err != nil { // 限制100MB内存
		panic(err)
	}

	// 3. 模拟进程
	cmd := exec.Command("stress", "--cpu", "1", "--vm", "1", "--vm-bytes", "200M")
	if err := cmd.Start(); err != nil {
		panic(err)
	}

	// 4. 将进程加入cgroup
	if err := cg.AddProcess(cmd.Process.Pid); err != nil {
		panic(err)
	}

	// 5. 等待进程执行
	if err := cmd.Wait(); err != nil {
		fmt.Println("进程执行完成:", err)
	}
}

var runtimeCommand = apis.Runtime{
	Name:                     "yolo-cmd",
	Type:                     apis.ByCommand,
	Command:                  []string{"python3"},
	Args:                     []string{"/home/kcm/workplace/migration-demo-0116/yolo-runner.py"},
	EnableFineGrainedControl: false,
	// EnableFineGrainedControl:     true,
	// EnableFineGrainedControlPort: "5123",
}

var group1 = &apis.Group{
	ObjectMeta: metav1.ObjectMeta{Name: "TestGroup-wasm", Namespace: "test"},
	TypeMeta:   metav1.TypeMeta{Kind: "Group", APIVersion: "resources/v1"},
	Spec: apis.GroupSpec{
		Name:    "TestGroup-wasm",
		Parents: make([]string, 0),
		Actions: []apis.Action{
			apis.Action{
				ObjectMeta: metav1.ObjectMeta{Name: "wasm_action"},
				Spec: apis.ActionSpec{
					Name: "wasm_action",
					Runtimes: []apis.Runtime{
						runtimeCommand,
					},
				},
				Status: apis.ActionStatus{
					ActionID: "wasm_action:TestGroup-wasm:test-task", // ActionID =ActionName + GroupID
					Phase:    apis.Unknown,
					RuntimeStatus: []apis.RuntimeStatus{
						apis.RuntimeStatus{
							RuntimeID: "wasm-cmd:wasm_action:TestGroup-wasm:test-task", // RuntimeID = RuntimeName + ActionID
							Phase:     apis.Unknown,
						},
					},
				},
			},
		},
		Replicas: []int32{0, 0},
	},
	Status: apis.GroupStatus{
		GroupID: "TestGroup-wasm:test-task",
		ActionStatus: []apis.ActionStatus{
			apis.ActionStatus{
				ActionID: "wasm_action:TestGroup-wasm:test-task",
				RuntimeStatus: []apis.RuntimeStatus{
					apis.RuntimeStatus{
						RuntimeID: "wasm-cmd:wasm_action:TestGroup-wasm:test-task",
						Phase:     apis.Unknown,
					},
				},
				Phase: apis.Unknown,
			},
		},
		Belongs: apis.IDRef{TaskID: "test-task"},
	},
}

func initClientSet(scheme *runtime.Scheme) *clients.ClientSet {
	apis.AddToScheme(scheme)
	// 创建ClientSet
	c := &rest.Config{
		Host:    "http://localhost:10000",
		APIPath: "/apis/resources/v1",
		ContentConfig: rest.ContentConfig{
			AcceptContentTypes: "application/json; charset=UTF-8", //text/plain; charset=UTF-8
			ContentType:        "application/json; charset=UTF-8", //application/json; charset=UTF-8
			GroupVersion: &schema.GroupVersion{
				Group:   "resources",
				Version: "v1",
			},
			NegotiatedSerializer: serializer.NewCodecFactory(scheme),
		},
		UserAgent: "defaultUserAgent",
		Transport: &http.Transport{
			MaxIdleConns:        100,              // 最大空闲连接数
			IdleConnTimeout:     90 * time.Second, // 空闲连接超时时间
			TLSHandshakeTimeout: 10 * time.Second, // TLS 握手超时时间
		},
		Timeout: 10 * time.Second,
	}
	clientSet, err := clients.NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return clientSet
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
	logs.Info()
}
