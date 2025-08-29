package utils

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

// uploadFile 发送文件内容到服务器
func UploadFile(filePath string, tag string, targetURL string) error {
	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", filePath, err)
	}
	defer file.Close()

	// 构建 url
	targetURL = fmt.Sprintf("%s?filename=%s&tag=%s", targetURL, filepath.Base(filePath), tag)
	// 获取文件名
	fileName := filepath.Base(filePath)

	// 构建 HTTP 请求体
	req, err := http.NewRequest("POST", targetURL, file)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("FileType", "file")
	req.Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.QueryEscape(fileName)))

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("File uploaded successfully: %s\n", filePath)
	return nil
}
