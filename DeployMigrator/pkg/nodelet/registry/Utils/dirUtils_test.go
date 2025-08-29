package utils

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func createTestDirWithFiles(baseDir string) (string, error) {
	// 创建临时文件夹
	tempDir, err := os.MkdirTemp(baseDir, "testdir")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %v", err)
	}

	// 创建子目录
	dir1 := filepath.Join(tempDir, "test1")
	dir2 := filepath.Join(tempDir, "test2")
	if err := os.Mkdir(dir1, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %v", dir1, err)
	}
	if err := os.Mkdir(dir2, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %v", dir2, err)
	}

	// 创建 1GB 的数据
	data := make([]byte, 1<<30)

	// 创建文件路径
	filePaths := []string{
		filepath.Join(dir1, "file1.txt"),
		filepath.Join(dir2, "file2.txt"),
	}

	// 写入文件内容
	for _, filePath := range filePaths {
		err := os.WriteFile(filePath, data, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write file %s: %v", filePath, err)
		}
	}

	// 返回创建的临时目录路径
	return tempDir, nil
}

func cleanUpDir(path string) error {
	return os.RemoveAll(path)
}

func createTestHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ReceiveDir(w, r, baseDir)
	}
}

// 测试发送文件夹
func TestSendDirAndRestore(t *testing.T) {
	// 创建测试目录并添加文件
	testRootDir, err := createTestDirWithFiles("./")
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	//defer cleanUpDir(testRootDir)

	//baseDir := "./Test"
	//// 模拟一个上传请求的服务器
	//server := httptest.NewServer(createTestHandler(baseDir))
	//defer server.Close()

	url := "http://127.0.0.1:8081/catalogueUpload"

	// 发送文件夹
	err = Traverse(testRootDir, url)
	if err != nil {
		t.Fatalf("SendDir failed: %v", err)
	}
}
