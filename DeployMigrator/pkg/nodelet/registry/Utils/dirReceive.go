package utils

import (
	"fmt"
	"hit.edu/framework/pkg/component-base/logs"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// createDir 创建目录
func createDir(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		err := os.MkdirAll(dirPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dirPath, err)
		}
		fmt.Printf("Directory created: %s\n", dirPath)
	} else {
		fmt.Printf("Directory already exists: %s\n", dirPath)
	}
	return nil
}

// ReceiveDir 接收文件和目录信息，并正确创建或存储
func ReceiveDir(w http.ResponseWriter, r *http.Request, baseDir string) {
	if baseDir == "" {
		http.Error(w, "BaseDir is not set", http.StatusInternalServerError)
		return
	}
	baseDir = filepath.Clean(baseDir)

	// 从 URL 查询参数获取 filename（用于根目录）
	//filename := r.Header.Get("rootPath")
	filename := r.URL.Query().Get("filename")
	if filename != "" {
		// 使用 filename 参数作为顶层目录名，加入到 baseDir
		baseDir = filepath.Join(baseDir, filename)
	}

	contentType := r.Header.Get("Content-Type")
	parentPath := r.Header.Get("Parent-Path")
	fileType := r.Header.Get("FileType")

	// 处理终止报文
	if fileType == "completion" {
		fmt.Println("Received transmission completion signal.")
		w.Write([]byte("Transmission completed successfully"))
		return
	}

	if parentPath == "" {
		http.Error(w, "Parent path is missing", http.StatusBadRequest)
		return
	}

	// 最终的父路径（本地路径）
	targetParent := filepath.Join(baseDir, filepath.Clean(parentPath))
	err := os.MkdirAll(targetParent, os.ModePerm)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create parent directory: %v", err), http.StatusInternalServerError)
		return
	}

	if contentType == "application/text" {
		// 创建子目录
		dirName := r.Header.Get("Directory-Name")
		if dirName == "" {
			http.Error(w, "Directory name is missing", http.StatusBadRequest)
			return
		}
		fullDirPath := filepath.Join(targetParent, dirName)
		if err := os.MkdirAll(fullDirPath, os.ModePerm); err != nil {
			http.Error(w, fmt.Sprintf("failed to create directory: %v", err), http.StatusInternalServerError)
			return
		}
		logs.Infof("Created directory: %s\n", fullDirPath)
		w.Write([]byte("Directory created successfully"))
		return
	}

	if contentType == "application/octet-stream" {
		fileName := r.Header.Get("File-Name")
		if fileName == "" {
			http.Error(w, "File name is missing", http.StatusBadRequest)
			return
		}
		filePath := filepath.Join(targetParent, fileName)
		file, err := os.Create(filePath)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create file: %v", err), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		_, err = io.Copy(file, r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to write file: %v", err), http.StatusInternalServerError)
			return
		}
		logs.Infof("Saved file: %s\n", filePath)
		w.Write([]byte("File uploaded successfully"))
		return
	}

	http.Error(w, "Unsupported content type", http.StatusBadRequest)
}
