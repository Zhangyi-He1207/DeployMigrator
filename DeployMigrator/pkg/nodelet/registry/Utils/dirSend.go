package utils

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SendFile 发送文件内容到服务器
func SendDirFile(rootPath, filePath string, parentPath string, url string) error {
	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", filePath, err)
	}
	defer file.Close()

	//// 获取文件信息
	//info, err := file.Stat()
	//if err != nil {
	//	return fmt.Errorf("failed to get file info %s: %v", filePath, err)
	//}

	// 读取文件内容
	//fileContent := make([]byte, info.Size())
	//_, err = file.Read(fileContent)
	//if err != nil && err != io.EOF {
	//	return fmt.Errorf("failed to read file content %s: %v", filePath, err)
	//}

	// 构建 HTTP 请求体
	//body := bytes.NewBuffer(fileContent)
	req, err := http.NewRequest("POST", url, file)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("FileType", "folder")
	req.Header.Set("File-Name", filepath.Base(filePath)) // 文件名
	req.Header.Set("Parent-Path", parentPath)            // 父目录路径
	req.Header.Set("rootPath", rootPath)                 // 根目录路径

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

// SendDir 发送目录信息到服务器
func SendDir(rootPath, dirPath string, parentPath string, url string) error {
	// 构建目录信息
	dirInfo := fmt.Sprintf("DIR:%s", dirPath)
	//body := bytes.NewBuffer([]byte(dirInfo))
	req, err := http.NewRequest("POST", url, strings.NewReader(dirInfo))
	if err != nil {
		return fmt.Errorf("failed to create directory request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/text")
	req.Header.Set("FileType", "folder")
	req.Header.Set("Directory-Name", filepath.Base(dirPath)) // 目录名
	req.Header.Set("Parent-Path", parentPath)                // 父目录路径
	req.Header.Set("rootPath", rootPath)                     // 根目录路径

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send directory request: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Directory uploaded successfully: %s\n", dirPath)
	return nil
}

// Traverse 遍历目录并发送文件或目录信息
func Traverse(rootPath string, url string) error {
	rootPath = filepath.Clean(rootPath)

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking through files: %v", err)
		}

		relativePath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path: %v", err)
		}

		var parentPath string
		if relativePath == "." {
			// 根目录：parentPath 是空字符串
			parentPath = ""
		} else {
			parentPath = filepath.Dir(relativePath)
		}

		if info.IsDir() {
			return SendDir(rootPath, path, parentPath, url)
		}
		return SendDirFile(rootPath, path, parentPath, url)
	})

	if err != nil {
		return err
	}

	if err := SendCompletionSignal(rootPath, url); err != nil {
		return err
	}

	fmt.Printf("Traversal completed successfully for root path: %s\n", rootPath)
	return nil
}

//func Traverse(rootPath string, url string) error {
//	// 确保路径规范化
//	rootPath = filepath.Clean(rootPath)
//
//	// 遍历子目录和文件
//	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
//		if err != nil {
//			return fmt.Errorf("error walking through files: %v", err)
//		}
//
//		// 获取父目录路径
//		parentPath := filepath.Dir(path)
//
//		// 如果是目录，发送目录信息
//		if info.IsDir() {
//			// 忽略根目录，因为它已经被处理过
//			if path != rootPath {
//				err := SendDir(rootPath, path, parentPath, url)
//				if err != nil {
//					return fmt.Errorf("failed to send directory: %v", err)
//				}
//			}
//		} else {
//			// 如果是文件，发送文件内容
//			err := SendDirFile(rootPath, path, parentPath, url)
//			if err != nil {
//				return fmt.Errorf("failed to send file: %v", err)
//			}
//		}
//		return nil
//	})
//	// 发送终止报文
//	if err = SendCompletionSignal(rootPath, url); err != nil {
//		return fmt.Errorf("error sending completion signal: %v", err)
//	}
//
//	fmt.Printf("Traversal completed successfully for root path: %s\n", rootPath)
//	return nil
//}

// SendCompletionSignal 发送终止报文以标识传输结束
func SendCompletionSignal(rootPath, url string) error {
	body := bytes.NewBufferString("TRANSMISSION_COMPLETE") // 自定义终止报文内容
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create completion signal request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/text")
	req.Header.Set("FileType", "completion") // 自定义 FileType 表示终止报文
	req.Header.Set("Root-Path", rootPath)    // 根路径标识

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send completion signal: %v", err)
	}
	defer resp.Body.Close()

	fmt.Println("Transmission completed successfully.")
	return nil
}
