package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func DownloadFile(url string, savePath string) error {
	// 发送 GET 请求
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned error: %s", resp.Status)
	}

	// 提取文件名
	// fileName := extractFileName(resp.Header.Get("Content-Disposition"))
	fileName := extractFileName(url)
	// 确保 savePath 目录存在
	savePath = filepath.Clean(savePath)
	err = os.MkdirAll(savePath, os.ModePerm)
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
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	fmt.Printf("File downloaded successfully: %s\n", filePath)
	return nil
}

// extractFileName 解析 Content-Disposition 头部中的文件名
// func extractFileName(contentDisp string) string {
// 	// 解析 filename*=UTF-8''encoded_filename
// 	re := regexp.MustCompile(`filename\*=(?i)UTF-8''([^;\r\n]+)`)
// 	matches := re.FindStringSubmatch(contentDisp)
// 	if len(matches) > 1 {
// 		decodedName, err := url.QueryUnescape(matches[1])
// 		if err == nil {
// 			return decodedName
// 		}
// 	}

// 	// 解析 filename="filename"
// 	re = regexp.MustCompile(`filename="([^"]+)"`)
// 	matches = re.FindStringSubmatch(contentDisp)
// 	if len(matches) > 1 {
// 		return matches[1]
// 	}

// 	return ""
// }

func extractFileName(downloadUrl string) string {
	// url最后的filename=后面的文件名,例如http://localhost:8919/download?filename=requirements.txt里面解析requirements.txt
	// 解析=后面的内容即可，找到=的位置，然后截取字符串
	if len(downloadUrl) == 0 {
		return ""
	}
	equalIndex := -1
	for i := len(downloadUrl) - 1; i >= 0; i-- {
		if downloadUrl[i] == '=' {
			equalIndex = i
			break
		}
	}
	if equalIndex == -1 || equalIndex == len(downloadUrl)-1 {
		return ""
	}
	fileName := downloadUrl[equalIndex+1:]
	return fileName

}
