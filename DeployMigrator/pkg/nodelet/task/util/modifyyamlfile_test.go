package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestName(t *testing.T) {
	testDir := "/home/public/goprojects/reference-pod/pkg/nodelet/task/util"
	srcFile := filepath.Join(testDir, "grpc-client-pod.yaml")
	destDir := testDir
	destFileName := "grpc-client-pod-copy.yaml"
	destFile := filepath.Join(destDir, "grpc-client-pod-copy.yaml")

	err := NewCopyYmal(srcFile, destDir, destFileName, "pve2")
	if err != nil {
		t.Error(err)
	}
	t.Run("test", func(t *testing.T) { //子测试创建方法
		if _, err := os.Stat(destFile); os.IsNotExist(err) { // 获取文件信息  // 判断错误是否为"文件不存在"
			t.Error("Output file was not created")
		}
	})

	t.Run("test2", func(t *testing.T) {
		data, err := os.ReadFile(destFile)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}
		content := string(data)

		expectedStrings := []string{
			"name: grpc-client-pod-copy",
			"app: grpc-client-copy",
			"name: grpc-client-service-copy",
		}

		for _, s := range expectedStrings {
			if !strings.Contains(content, s) {
				t.Errorf("Output file did not contain expected output file content: %s", content)
			}
		}
		// 验证原始未修改字段仍然存在
		if !strings.Contains(content, "kind: Pod") || !strings.Contains(content, "kind: Service") {
			t.Error("Original fields were corrupted")
		}

	})
}
