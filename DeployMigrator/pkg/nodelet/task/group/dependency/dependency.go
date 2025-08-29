// Dependency作用是检查程序的包依赖是否满足（目前针对python任务），具体功能有：
// check:检查设备上的虚拟环境与提供的requirememts.txt是否能够满足
// setUpEnv:根据requirememts.txt创建新的虚拟环境，一般在不满足依赖的情况下调用
// inputEnv:注入环境变量，将系统自身的Env加上指定虚拟环境的python的环境变量加入到PATH之后，注入到runtime的Env中，在command.go中执行的时候直接加入到cmd.Env即可
package dependency

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	apis "hit.edu/framework/pkg/apis/cores"
	"hit.edu/framework/pkg/component-base/logs"
)

type DependencyManager struct {
	lock        sync.Mutex
	allEnv      map[string]string
	envsPackage map[string][]apis.Requirement
}

func NewDependencyManager() *DependencyManager {
	dm := &DependencyManager{
		allEnv:      make(map[string]string),
		envsPackage: make(map[string][]apis.Requirement),
	}
	return dm
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logs.Trace("Command output: %s", out.String())
		logs.Trace("Command error: %s", stderr.String())
		return fmt.Errorf("command execution failed: %v", err)
	}

	logs.Info("Command output: %s", out.String())
	return nil
}

func GetAllCondaEnv() (map[string]string, error) {
	// startTime := time.Now()
	cmd := exec.Command("conda", "info", "--envs")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		logs.Info("Error executing conda command: %v", err)
		return nil, err
	}
	// 解析输出
	output := out.String()
	lines := strings.Split(output, "\n")
	// 存储环境名称
	envs := make(map[string]string)

	// 遍历每一行，提取虚拟环境名称
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 1 && !strings.HasPrefix(fields[0], "#") {
			// envNames = append(envNames, fields[0]) // 添加环境名称
			envs[fields[0]] = fields[len(fields)-1] // 添加环境名称及其路径
		}
	}
	// timeCost := time.Since(startTime)
	// fmt.Println("GetAllCondaEnv cost %s time", timeCost)
	return envs, nil
}

// 使用pip list获取pip freeze格式（numpy==1.23.1这种）的字符数组
func (dm *DependencyManager) GetInstalledPackages(envName string) ([]apis.Requirement, error) {
	// startTime := time.Now()
	cmd := exec.Command("conda", "run", "-n", envName, "pip", "list", "--format=freeze")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	installed := make(map[string]string)
	re := regexp.MustCompile(`(?P<Name>[a-zA-Z0-9_-]+)==(?P<Version>[0-9\.]+)`)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		match := re.FindStringSubmatch(line)
		if match != nil {
			installed[match[1]] = match[2]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// timeCost := time.Since(startTime)
	// fmt.Println("GetInstalledPackages cost %s time", timeCost)
	InstalledRequirements := make([]apis.Requirement, 0)
	for name, version := range installed {
		InstalledRequirements = append(InstalledRequirements, apis.Requirement{Name: name, Version: version})
	}
	return InstalledRequirements, nil
}

// GetCondaEnvPath 根据传入的虚拟环境名称，返回虚拟环境的路径
func GetCondaEnvPath(envName string) (string, error) {
	// 执行 "conda info --envs" 命令，获取所有虚拟环境的信息  命令conda env list 也可以
	cmd := exec.Command("conda", "info", "--envs")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		logs.Error("failed to execute conda command!")
		return "", errors.New(string("failed to execute conda command"))
	}
	// 解析输出
	output := out.String()
	lines := strings.Split(output, "\n")
	// 遍历每一行，查找目标环境
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 1 && fields[0] == envName {
			return fields[len(fields)-1], nil // 返回路径
		}
	}
	// 如果没有找到环境
	return "", fmt.Errorf("environment '%s' not found", envName)
}

func (dm *DependencyManager) UpdateEnvs() error {
	startTime := time.Now()
	envs, err := GetAllCondaEnv()
	if err != nil {
		logs.Errorf("Get All Conda Environments Err. Maybe pip is not installed")
		return fmt.Errorf("UpdateEnvs err!")
	}
	dm.lock.Lock()
	defer dm.lock.Unlock()
	dm.allEnv = envs

	timeCost := time.Since(startTime)
	logs.Trace("UpdateEnvs success! cost %v time", timeCost)
	// logs.Trace("UpdateEnvs success")
	return nil
}

func (dm *DependencyManager) UpdateEnvPackages() error {
	startTime := time.Now()
	dm.lock.Lock()
	defer dm.lock.Unlock()
	for envname := range dm.allEnv {
		installed, err := dm.GetInstalledPackages(envname)
		if err != nil {
			logs.Tracef("Error retrieving installed packages: %v", err)
			return fmt.Errorf("UpdateEnvPackages err!")
		}
		dm.envsPackage[envname] = installed
	}

	timeCost := time.Since(startTime)
	logs.Trace("GetInstalledPackages cost %v time", timeCost)
	logs.Trace("UpdateEnvPackages success")
	return nil
}

// 查找当前conda的  所有虚拟环境，判断是否有虚拟环境满足这个requirements.txt的依赖,如果有则返回name与路径
func (dm *DependencyManager) CheckEnvironmentSatisfy(requirements []apis.Requirement) (string, string, bool) {
	//for遍历所有虚拟环境
	envs := dm.allEnv
	for envname, envpath := range envs {
		installedRequire := dm.envsPackage[envname]
		if CheckRequirements(requirements, installedRequire, envname) {
			logs.Trace("All requirements are satisfied. EnvName:%v", envname)
			envpath = envpath + "/bin/python"
			return envname, envpath, true
		} else {
			// logs.Info("Some requirements are not satisfied. EnvName:%v", envname)
		}
	}
	return "", "", false
}

// 解析requirements.txt文件，返回包的切片
func (dm *DependencyManager) ParseRequirements(filePath string) ([]apis.Requirement, error) {
	startTime := time.Now()
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var requirements []apis.Requirement
	re := regexp.MustCompile(`(?P<Name>[a-zA-Z0-9_-]+)(==(?P<Version>[0-9\.]+))?`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		match := re.FindStringSubmatch(line)
		if match != nil {
			req := apis.Requirement{
				Name:    match[1],
				Version: match[3],
			}
			requirements = append(requirements, req)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	timeCost := time.Since(startTime)
	logs.Trace("ParseRequirements cost %s time", timeCost)
	return requirements, nil
}

// 将获取到的requirements.txt的内容与已有的installed的内容比较
func CheckRequirements(requirements []apis.Requirement, installed []apis.Requirement, envName string) bool {
	startTime := time.Now()
	allSatisfied := true
	for _, req := range requirements {
		found := false
		name := req.Name
		// reqVersion := req.Version
		var installedVersion string
		for _, installed := range installed {
			if installed.Name == name {
				found = true
				installedVersion = installed.Version
			}
		}
		if !found {
			// logs.Info("Package %v is not installed.", req.Name)
			allSatisfied = false
			return allSatisfied
		} else if req.Version != "" && installedVersion < req.Version {
			//找到对应的package但是版本落后
			logs.Info("Package %v version mismatch: required %v, installed %v.", req.Name, req.Version, installedVersion)
			allSatisfied = false
			return allSatisfied
		} else {
			// logs.Info("Package %s is satisfied.", req.Name)
		}
	}
	// logs.Info("requirements satisfied envName: %v", envName)
	timeCost := time.Since(startTime)
	logs.Trace("CheckRequirements cost %s time", timeCost)
	return allSatisfied
}

// 根据已有的requirements.txt创建新的conda环境
// 有些包可能使用conda create -n --file requirements.txt无法从conda官网安装，这里采取先新创建空的
// conda虚拟环境，然后再执行pip install，这是手动配置环境的正常流程
// 可能存在一个问题，创建新的虚拟环境的试时候需要指定python版本，目前默认3.9版本，后续需要调整
func SetupEnvironment(reqFile string, runtiemName string) bool {
	pythonVersion := "python=3.9"
	nowtime := time.Now().Format("2006-01-02 15:04:05")
	newEnvName := runtiemName + nowtime
	newEnvName = strings.Replace(newEnvName, ":", "-", -1)
	newEnvName = strings.Replace(newEnvName, " ", "-", -1)
	// reqFile := "requirements.txt"
	if _, err := os.Stat(reqFile); os.IsNotExist(err) {
		logs.Error("requirements.txt not found: %v", err)
		return false
	}
	createEnvCmd := "conda"
	createEnvArgs := []string{"create", "-n", newEnvName, pythonVersion}
	CMDCreate := exec.Command(createEnvCmd, createEnvArgs...)
	logs.Info("Creating a new Conda environment...")
	if err := CMDCreate.Start(); err != nil {
		logs.Error("Failed to start create Conda environment: %v", err)
		return false
	}
	if err := CMDCreate.Wait(); err != nil {
		logs.Info("Failed to finish create Conda environment: %v", err)
		return false
	}
	//需要指定run -n的名称
	installEnvCmd := "conda"
	installEnvArgs := []string{"run", "-n", newEnvName, "pip", "install", "-r", reqFile}
	CMDInstall := exec.Command(installEnvCmd, installEnvArgs...)
	logs.Info("Installing for new Conda environment...")
	if err := CMDInstall.Start(); err != nil {
		logs.Error("Failed to start install dependency requirements.txt: %v", err)
		return false
	}
	if err := CMDInstall.Wait(); err != nil {
		logs.Info("Failed to finish install dependency requirements.txt: %v", err)
		return false
	}
	logs.Info("Environment '%v' created successfully with requirements from %v", newEnvName, reqFile)
	return true
}

// conda remove --name ENV_NAME --all
// TODO 存在一种场景，有其他程序正在使用这个虚拟环境，这时候删除会出现未知的结果，可能是删除失败
func RemoveEnvironment(envName string) bool {
	removeEnvCmd := "conda"
	removeEnvArgs := []string{"remove", "-n", envName, "--all"}
	logs.Info("Removing Conda environment:%v", envName)
	if err := runCommand(removeEnvCmd, removeEnvArgs...); err != nil {
		logs.Error("Failed to remove Conda environment:%v. err info: %v", envName, err)
		return false
	}
	logs.Info("Environment '%v' removed successfully", envName)
	return true
}

// 根据传入的虚拟环境名称，生成一个用于runtime的ENV属性的Value并返回
func EnvForInput(envName string) (string, bool) {
	// pythonPath := "/home/ysk/miniconda3/envs/py38/bin" // 替换为Python 安装路径
	// pythonPath := "" // 替换为Python 安装路径
	// cmd := exec.Command("/bin/bash","-c","source /home/ysk/miniconda3/etc/profile.d/conda.sh;conda activate py38;" + "which python;python " + "predict.py")   // 启动 bash 终端
	// cmd := exec.Command("/bin/bash","-c","echo $PATH;which python;python " + "predict.py")   // 启动 bash 终端
	// 获取当前环境变量
	//先找到这个名字的虚拟环境的path
	pythonPath, err := GetCondaEnvPath(envName)
	if err != nil {
		logs.Errorf("get conda Env err! envName: %v", envName)
		return "", false
	}
	//填充虚拟环境路径后的/bin，python解释器在这里
	pythonPath = pythonPath + "/bin/python"
	// env := os.Environ()
	// // 初始化新的环境变量列表,两种方法，删除base加入新的虚拟环境；
	// //第二种：在PATH最前面拼接新的虚拟环境，这样就不会被base替代---目前是这种
	// var updatedEnv string

	// for _, e := range env {
	// 	if strings.HasPrefix(e, "PATH=") {
	// 		// 获取当前 PATH 的值
	// 		currentPath := strings.TrimPrefix(e, "PATH=")
	// 		// fmt.Printf("e: %v", e)
	// 		// var newPath = "PATH=" + pythonPath + currentPath
	// 		//下面的写法是只保留PATH=之后的value的内容，后续可以根据需要增加其他环境变量
	// 		//export PATH=$(echo "$PATH" | awk -v RS=: '!a[$0]++' | tr '\n' :)  去除PATH中重复的路径，暂时认为重复会不会有影响
	// 		var newPath = pythonPath + ":" + currentPath
	// 		// updatedEnv = append(updatedEnv, newPath)
	// 		updatedEnv = newPath
	// 	} else {
	// 		// 其他环境变量保持不变
	// 		//下面注释掉的话，updatedEnv只会保留PATH这一条环境变量
	// 		// updatedEnv = append(updatedEnv, e)
	// 	}
	// }
	// return updatedEnv
	return pythonPath, true
}

// TODO 正则表达式解析，格式：
//
//	     Results的访问格式对应
//				TODO: 正则表达式
//				Action{ID}.Results.{Name}， 缺省访问本Group对应的Action
//				Group{ID}.Action{ID}.Results.{Name}， 访问对应Group的对应Action
//				Task{ID}.Group{ID}.Action{ID}.Runtime{}.Results.{Name}, 访问对应Task的对应Group的对应Action
//	         目前不支持跨Workflow获取数据
//	  对于Local类型，Value对应从本地资源或者数据节点获取的数据，ValueType对应从资源或者数据中获取的数据类型
//	  	Local的访问格式对应
//	         Resources.{Name}: 从本地资源中获取
//				Devices.{Name}: 从本地设备里列表中获取
//				Scenes.{Name}： 从本地场景中获取
//				Data.{Name}： 从本地数据中获取
func ParseFrom(input string) (TaskID, GroupID, ActionID, RuntimeID, Field string, ParseTypeID int, err error) {
	// 定义部分名称的顺序
	partNames := []string{"Task", "Group", "Action", "Runtime"}

	// 初始化结果
	result := make(map[string]string)
	for _, name := range partNames {
		result[name] = ""
	}
	result["Field"] = ""

	// 将输入字符串按 '.' 分割
	parts := strings.Split(input, ".")

	// 依次处理每个部分
	for _, part := range parts {
		if part == "" {
			continue
		}
		// 检查是否是 Field
		if part == "Field" {
			result["Field"] = part
			continue
		}
		// 分割 Part{ID}
		partSplit := strings.SplitN(part, "{", 2)
		if len(partSplit) != 2 {
			continue
		}
		partName := partSplit[0]
		partID := strings.TrimSuffix(partSplit[1], "}")
		if _, exists := result[partName]; exists {
			result[partName] = partID
		}
	}

	// 提取结果
	TaskID = result["Task"]
	GroupID = result["Group"]
	ActionID = result["Action"]
	RuntimeID = result["Runtime"]
	Field = result["Field"]

	ParseTypeID = 0
	// 如果某部分没有匹配到，则将ParseTypeID加上对应值，判断是那一种情况
	//Task{ID}.Group{ID}.Action{ID}.Runtime{ID}
	//     0000 ~ 1111，某些情况去除
	if TaskID == "" {
		ParseTypeID += 1 << 3
	}
	if GroupID == "" {
		ParseTypeID += 1 << 2
	}
	if ActionID == "" {
		ParseTypeID += 1 << 1
	}
	if RuntimeID == "" {
		ParseTypeID += 1
	}
	//ID就是Name
	return TaskID, GroupID, ActionID, RuntimeID, Field, ParseTypeID, err
}

func ParseField(Field interface{}, path string) (interface{}, string, error) {
	//两类数据类型对应两类正则解析式
	// 定义正则表达式
	//遇到一个问题，这里的每一项的类型都可能不一样，比如使用列表、结构体、map，这三种类型的话，{}里面应该怎么填比较合适？通过reflect应该已经解决了，等待测试
	//对于列表，里面填下标可能是不合适的，例如目前的RuntimeStatus
	//格式为 Status/Spec.RuntimeStatus{}
	// 按照 '.' 分割路径
	parts := strings.Split(path, ".")

	// 逐步解析路径
	current := reflect.ValueOf(Field)
	for _, part := range parts {
		// 解析字段名和索引
		field := strings.Split(part, "{")
		fieldName := field[0]
		var index string
		if len(field) > 1 {
			index = strings.TrimSuffix(field[1], "}")
		} else {
			index = ""
		}

		// 获取字段值
		fieldValue := current.FieldByName(fieldName)
		if !fieldValue.IsValid() {
			return nil, "", errors.New("")
		}

		// 如果有索引，处理索引
		if index != "" {
			if fieldValue.Kind() == reflect.Map {
				mapKey := reflect.ValueOf(index)
				fieldValue = fieldValue.MapIndex(mapKey)
			} else if fieldValue.Kind() == reflect.Slice {
				indexInt, err := strconv.Atoi(index)
				if err != nil || indexInt < 0 || indexInt >= fieldValue.Len() {
					return nil, "", errors.New("")
				}
				fieldValue = fieldValue.Index(indexInt)
			}
		}

		// 更新 current 为当前字段值
		current = fieldValue
	}

	// 返回最终的值
	return current.Interface(), current.Type().String(), nil
}

//依次提取，可能会简单一点
// func ParseConditionFrom(input string) (TaskID, GroupID, ActionID, RuntimeID string, ParseTypeID int, err error) {
// 	// 定义正则表达式，匹配 Part{ID} 格式
// 	re := regexp.MustCompile(`^([^{]+)\{([^}]*)\}$`)

// 	// 将输入字符串按 '.' 分割
// 	parts := strings.Split(input, ".")

// 	// 定义部分名称的顺序
// 	partNames := []string{"Task", "Group", "Action", "Runtime", "Field"}

// 	// 初始化结果
// 	result := make(map[string]string)
// 	for _, name := range partNames {
// 		result[name] = ""
// 	}

// 	// 依次处理每个部分
// 	for _, part := range parts {
// 		if part == "" {
// 			continue
// 		}
// 		matches := re.FindStringSubmatch(part)
// 		if matches != nil {
// 			partName := matches[1]
// 			partID := matches[2]
// 			if _, exists := result[partName]; exists {
// 				result[partName] = partID
// 			}
// 		} else {
// 			// 如果没有匹配到 Part{ID} 格式，检查是否是 Field
// 			if part == "Field" {
// 				result["Field"] = part
// 			}
// 		}
// 	}

// 	// 提取结果
// 	taskID = result["Task"]
// 	groupID = result["Group"]
// 	actionID = result["Action"]
// 	runtimeID = result["Runtime"]
// 	field = result["Field"]

// 	return
// }
