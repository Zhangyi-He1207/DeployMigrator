package command

import (
	"os/exec"
	"testing"
)

func TestCMD(t *testing.T) {
	exec.Command("/bin/sh", "-c", "echo hello world").Run()
}

//func TestStartCMD(t *testing.T) {
//	//构建cmd
//	//cmds := []string{"/bin/bash", "ls"}
//	cmd := "ls"
//
//	// 构建args
//	args := []string{"-al"}
//
//	err := startCMD(cmd, args)
//	if err != nil {
//		t.Errorf("start cmd failed: %v", err)
//	}
//}

func TestStartCMDForWindows(t *testing.T) {
	//segment_train := "D:\\Programming\\GoLand\\goProject\\new2-task\\resourcelet\\test\\nodelet\\task_exporter\\cmd_yolo\\yolo_task\\segment_train.yaml"
	//premodule := "D:\\Programming\\GoLand\\goProject\\new2-task\\resourcelet\\test\\nodelet\\task_exporter\\cmd_yolo\\yolo_task\\yolo11x-seg.pt"
	//train := "D:\\Programming\\GoLand\\goProject\\new2-task\\resourcelet\\test\\nodelet\\task_exporter\\cmd_yolo\\yolo_task\\train.py" // 训练任务 Python 脚本路径
	//predict := "D:\\Programming\\GoLand\\goProject\\new2-task\\resourcelet\\test\\nodelet\\task_exporter\\cmd_yolo\\yolo_task\\predict.py"
	//fmt.Println("Segment Train Path:", segment_train)
	//fmt.Println("ScriptPath:" + train) // 打印文件路径
	//fmt.Println("PreModule Path:" + premodule)
	//fmt.Println("Predict Path:" + predict)
	////构建cmd
	////cmds := []string{"/bin/bash", "ls"}
	//runtime := NewCommandRuntime()
	//cmd := "D:\\Programming\\Anaconda\\envs\\yolo\\python.exe"
	//
	//// 构建args
	////args := []string{"-m", "yolo", "segment", "train", "data=" + segment_train, "model=" + premodule, "epoch=100", "imgsz=640", "batch=8", "workers=1", "device=cuda"}
	//args := []string{predict}
	//
	//err := runtime.startCMD("train-task", cmd, args)
	//if err != nil {
	//	t.Errorf("start cmd failed: %v", err)
	//}
}

//临时加的，在Windows上测试用的，可以删除
//if runtime.GOOS == "windows" {
//	//cmd = []string{"cmd"}          // 启动 cmd.exe
//	//args = []string{"/c", "cd .."} // 执行 'dir' 命令，并在执行后退出
//	//pythonPath := "/home/your-username/anaconda3/envs/yolo/bin/python" // Linux/macOS
//	pythonPath := "D:\\Programming\\Anaconda\\envs\\yolo\\python.exe" // Windows
//	module := "D:\\Programming\\GoLand\\goProject\\new2-task\\resourcelet\\test\\nodelet\\task_exporter\\cmd_yolo\\yolo_task\\yolo11x-seg.pt"
//	scriptPath := "D:\\Programming\\GoLand\\goProject\\new2-task\\resourcelet\\test\\nodelet\\task_exporter\\cmd_yolo\\yolo_task\\train.py" // 训练任务 Python 脚本路径
//	//comm := exec.Command(pythonPath, scriptPath, "--data", "segment_train.yaml", "--model", module, "--epochs", "500")
//	cmd[0] = pythonPath
//	args = []string{scriptPath, "--data", "segment_train.yaml", "--model", module, "--epochs", "500"}
//}
