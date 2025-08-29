package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func ReceiveFile(w http.ResponseWriter, r *http.Request, savePath string) error {
	// 提取文件名
	fileName := extractFileName(r.Header.Get("Content-Disposition"))

	// 确保 savePath 目录存在
	savePath = filepath.Clean(savePath)
	err := os.MkdirAll(savePath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// 拼接完整文件路径
	filePath := filepath.Join(savePath, fileName)

	// 创建文件
	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer outFile.Close()

	// 复制文件内容
	_, err = io.Copy(outFile, r.Body)
	if err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	fmt.Printf("File downloaded successfully: %s\n", filePath)
	return nil
}
