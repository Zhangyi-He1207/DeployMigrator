package utils

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

// GetQueryParamCaseInsensitive 获取不区分大小写的查询参数
func GetQueryParamCaseInsensitive(params map[string][]string, paramName string) string {
	for key, values := range params {
		if strings.EqualFold(key, paramName) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// GetFileParams 从请求中提取文件名和标签参数
func GetFileParams(r *http.Request, defaultTag string) (string, string, string, string, error) {
	// 获取参数
	param := r.URL.Query()

	// 获取文件名参数
	fileName := GetQueryParamCaseInsensitive(param, "filename")
	if fileName == "" {
		return "", "", "", "", fmt.Errorf("filename is required")
	}
	// 清理路径，防止路径遍历攻击
	fileName = filepath.Clean(fileName)

	// 获取标签参数（如果未提供，使用默认值）
	tag := GetQueryParamCaseInsensitive(param, "tag")
	if tag == "" {
		tag = defaultTag
	}

	// 获取文件所有者
	owner := r.RemoteAddr

	// 获取文件类型
	fileType := r.Header.Get("FileType")
	if fileType == "" {
		fileType = "file"
	}

	return fileName, tag, owner, fileType, nil
}
