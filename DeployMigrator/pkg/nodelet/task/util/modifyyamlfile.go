package util

import (
	"gopkg.in/yaml.v3"
	"hit.edu/framework/pkg/component-base/logs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func NewCopyYmal(srcPath, destDir, destFilename string, LocalDomainName string) error { //源yaml文件的地址，目标yaml文件放置的目录，目标yaml的名字,再传一个本域的域ID
	// 1. 读取源文件
	data, err := ioutil.ReadFile(srcPath)
	if err != nil {
		logs.Errorf("Failed to read source YAML: %v", err)
		return err
	}

	// 2. 创建目标目录（如果不存在）
	if err := os.MkdirAll(destDir, 0755); err != nil {
		logs.Errorf("Failed to create directory %s: %v", destDir, err)
		return err
	}

	// 3. 处理 YAML 内容
	modified, err := processYAMLContent(string(data), LocalDomainName)
	if err != nil {
		return err
	}

	// 4. 生成完整目标路径
	destPath := filepath.Join(destDir, destFilename)
	if err := ioutil.WriteFile(destPath, []byte(modified), 0644); err != nil {
		logs.Errorf("Failed to write modified YAML: %v", err)
		return err
	}
	return nil
}

// 处理 YAML 内容的核心逻辑
func processYAMLContent(original string, localDomainName string) (string, error) {
	var modifiedDocs []string

	// 分割文档时兼容不同分隔符格式
	docs := strings.Split(original, "---\n") // 首先尝试用 "---\n" 分割文档
	if len(docs) == 1 {
		docs = strings.Split(original, "---") // 如果分割后只有1个文档（说明没找到"---\n"分隔符）
	}

	for _, doc := range docs {
		doc = strings.TrimSpace(doc) //清理字符串首尾空白字符，开头的 \n 和空格被移除，结尾的 \t 被移除，中间用于缩进的空格保留
		if doc == "" {
			continue
		}

		var obj map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			logs.Errorf("YAML unmarshal error: %v", err)
			return "", err
		}
		// 修改目标字段
		modifyResources(obj, localDomainName) // 注意map是引用类型

		modified, err := yaml.Marshal(obj)
		if err != nil {
			logs.Errorf("YAML marshal error: %v", err)
			return "", err
		}
		modifiedDocs = append(modifiedDocs, string(modified))
	}
	return strings.Join(modifiedDocs, "---\n"), nil
}

// 资源类型判断与修改
func modifyResources(obj map[string]interface{}, localDomainName string) {
	switch obj["kind"] {
	case "Pod":
		modifyField(obj, "metadata.name")
		modifyField(obj, "metadata.labels.app")
		modifyPodEnv(obj, localDomainName) // 新增：处理Pod的env字段
	case "Service":
		modifyField(obj, "metadata.name")
		modifyField(obj, "spec.selector.app")
	}
}

// 改进版字段修改函数
func modifyField(obj map[string]interface{}, path string) {
	fields := strings.Split(path, ".")
	current := obj

	for _, field := range fields[:len(fields)-1] { // 排除最后一级字段，对于path：metadata.name，则排除的是是name，遍历到metadata就跳出for循环了
		val, ok := current[field].(map[string]interface{}) // 是一种类型断言的操作，用于将接口类型转换为具体的类型
		if !ok {
			return
		}
		current = val
	}

	lastField := fields[len(fields)-1] //这里就是“name”了
	if val, ok := current[lastField].(string); ok {
		current[lastField] = val + "-copy"
	}
}

// 处理Pod的env字段
func modifyPodEnv(obj map[string]interface{}, localDomainName string) {
	if localDomainName == "" {
		logs.Infof("非跨域需求的修改")
		return
	}
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return
	}

	containers, ok := spec["containers"].([]interface{})
	if !ok || len(containers) == 0 {
		return
	}

	// 只处理第一个container
	container, ok := containers[0].(map[string]interface{})
	if !ok {
		return
	}

	env, ok := container["env"].([]interface{})
	if !ok {
		return
	}

	// 遍历env变量
	for _, e := range env {
		envVar, ok := e.(map[string]interface{})
		if !ok {
			continue
		}

		if name, ok := envVar["name"].(string); ok && name == "MY_IP" {
			if value, ok := envVar["value"].(string); ok {
				// 修改MY_IP的value值
				envVar["value"] = localDomainName + "." + value
				envVar["value"] = localDomainName + "." + value
			}
		}
	}
}

/*
obj := map[string]interface{}{
    "apiVersion": "v1",
    "kind":       "Pod",
    "metadata": map[string]interface{}{
        "name":      "grpc-client-pod-copy",
        "namespace": "switch",
        "labels": map[string]interface{}{
            "app": "grpc-client-v2",
        },
    },
    "spec": map[string]interface{}{
        "nodeSelector": map[string]interface{}{
            "kubernetes.io/hostname": "debian1",
        },
        "containers": []interface{}{
            map[string]interface{}{
                "name":  "grpc-client",
                "image": "registry.cn-hangzhou.aliyuncs.com/sudasuzhou/grpc-client-pod:grpc-client-pod-v4.0",
                "env": []interface{}{
                    map[string]interface{}{
                        "name":  "MY_IP",
                        "value": "grpc-server-service.switch.svc.cluster.local", // pve2.grpc-server-service.switch.svc.cluster.local
                    },
                    map[string]interface{}{
                        "name":  "MY_PORT",
                        "value": "50051",
                    },
                },
                "ports": []interface{}{
                    map[string]interface{}{
                        "containerPort": 50052,
                    },
                },
            },
        },
    },
}


*/
