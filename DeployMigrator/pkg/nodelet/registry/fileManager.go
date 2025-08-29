package fileManager

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hit.edu/framework/pkg/component-base/logs"
	utils "hit.edu/framework/pkg/nodelet/registry/Utils"
)

// 测试使用，暂时只支持单个文件的上传和下载，删除需要手动。
// 在config.go中配置上传和下载的url、保存路径等信息。
const (
	//未下载
	NotDownloaded string = "not_downloaded"
	//正在下载
	Downloading string = "downloading"
	//下载完成
	Downloaded string = "download_completed"
	//下载失败
	DownloadFailed string = "download_failed"
	//未上传
	NotUploaded string = "not_uploaded"
	//正在上传
	Uploading string = "uploading"
	//上传完成
	Uploaded string = "upload_completed"
	//上传失败
	UploadFailed string = "upload_failed"
)

type FileManager struct {
	// 一些配置参数，例如上传和下载的URL、保存路径等
	DataSavedDir string
	UploadURL    string
	ForwardURL   string
	DownloadURL  string
	// 记录本地的runtime的文件下载情况：未下载、正在下载、下载完成、下载失败。key是带后缀的runtime.Name-文件名（例如R1-xxxxx...-test.txt），value是文件的下载状态
	DownloadStatus map[string]string
	// 记录本地的runtime的文件上传情况：未上传、正在上传、上传完成、上传失败。key是带后缀的runtime.Name，value是data[]里面所有文件的上传状态
	UploadStatus map[string]string
}

// type FileHandler interface {
// 	UploadFile(filePath string) (string, error)
// 	uploadDir(dirPath string) (string, error)
// 	DownloadFile(filename, savePath string) (string, error)
// 	DownloadDir(dirPath, savePath string) (string, error)
// 	ForwardFile(filePath string) (string, error)
// 	DeleteFile(filePath string) (string, error)
// 	DeleteDir(dirPath string) (string, error)
// }

func NewFileManager(fileRegistry string) *FileManager {
	logs.Infof("DataSavedDir: %s", DataSavedDir)
	logs.Infof("UploadURL: %s", UploadURL)
	logs.Infof("ForwardURL: %s", ForwardURL)
	logs.Infof("DownloadURL: %s", DownloadURL)
	downloadStatus := make(map[string]string)
	uploadStatus := make(map[string]string)
	// 初始化 FileManager
	FileManager := &FileManager{
		DataSavedDir:   DataSavedDir,
		UploadURL:      fileRegistry + UploadURL,
		ForwardURL:     fileRegistry + ForwardURL,
		DownloadURL:    fileRegistry + DownloadURL,
		DownloadStatus: downloadStatus,
		UploadStatus:   uploadStatus,
	}
	err := FileManager.Init()
	if err != nil {
		logs.Errorf("NewFileManager Init Err! 需要创建 home/public/tmp/data 文件夹")
	}
	return FileManager
}

func (fm *FileManager) Init() error {
	//这个协程用来检查和创建apis.FileFolder
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logs.Errorf("获取用户主目录失败: %v", err)
		return err
	}
	tmpDir := filepath.Join(homeDir, "tmp")
	//tmpDir := "../tmp"
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		// 目录不存在，创建目录
		err := os.Mkdir(tmpDir, os.ModePerm) // 权限
		if err != nil {
			logs.Errorf("Handler创建目录时发生错误: %v\n", err)
			return err
		}
	}
	dataFir := filepath.Join(tmpDir, "data")
	//dataFir := "../tmp/data"
	if _, err := os.Stat(dataFir); os.IsNotExist(err) {
		err := os.Mkdir(dataFir, os.ModePerm)
		if err != nil {
			logs.Errorf("Handler创建目录时发生错误: %v\n", err)
			return err
		}
	}

	//TODO:后续可能会需要在这里进行group目录进行删除操作

	return nil
}

func (fm *FileManager) UploadFile(filePath string) (string, error) {
	// 调用 utils.UploadFile 函数上传文件
	// 这里的 filePath 是要上传的文件路径
	// 返回上传结果和错误信息
	url := fm.UploadURL
	err := utils.UploadFile(filePath, "v1.0.0", url)
	if err != nil {
		fmt.Println("Upload failed:", err)
		return "", err
	} else {
		fmt.Println("Upload successful!")
		return "Upload successful!", nil
	}
}

// 上传dirPath这个目录下的所有文件
func (fm *FileManager) UploadFolder(dirPath string) (string, error) {
	// 调用 utils.UploadDir 函数上传目录
	// 这里的 dirPath 是要上传的目录路径
	// 返回上传结果和错误信息
	// 根据dirPath获取文件夹名称，最后一个/后面的部分
	if dirPath == "" {
		return "", errors.New("dirPath is empty")
	}
	parts := strings.Split(dirPath, "/")
	if len(parts) == 0 {
		return "", errors.New("dirPath does not contain any parts")
	}
	// 获取最后一个部分作为文件夹名称
	// 如果最后一个部分是空字符串，说明路径以/结尾，可能是一个空目录
	// 例如 "/home/user/documents/"，最后一个部分是空
	if parts[len(parts)-1] == "" {
		if len(parts) < 2 {
			return "", errors.New("dirPath does not contain a valid folder name")
		}
		// 如果是空目录，使用倒数第二个部分作为文件夹名称
		folderName := parts[len(parts)-2]
		if folderName == "" {
			return "", errors.New("folder name is empty")
		}
		// fmt.Println("Folder Name:", folderName)
	}
	folderName := parts[len(parts)-1]
	if folderName == "" {
		return "", errors.New("folder name is empty")
	}
	// fmt.Println("Folder Name:", folderName)
	// 构造上传URL
	url := fm.UploadURL + "?filename=" + folderName

	err := utils.Traverse(dirPath, url)
	if err != nil {
		fmt.Println("Upload failed:", err)
	} else {
		fmt.Println("Upload successful!")
	}
	return "Upload successful!", nil
}

func (fm *FileManager) DownloadFile(filename, savePath string) (string, error) {
	// 调用 utils.DownloadFile 函数下载文件
	// 这里的 filename 是要下载的文件名，savePath 是保存路径
	// 返回下载结果和错误信息
	// downloadURL := fm.DownloadURL + filename

	url := fm.DownloadURL
	downloadURL := url + filename
	fm.DownloadStatus[filename] = Downloading
	err := utils.DownloadFile(downloadURL, savePath)
	if err != nil {
		fmt.Println("Download failed:", err)
		fm.DownloadStatus[filename] = DownloadFailed
		fmt.Printf("file:%s,Download status:%s", filename, DownloadFailed)
		return "", err
	} else {
		fm.DownloadStatus[filename] = Downloaded
		fmt.Println("Download successful!")
		return "Download successful!", nil
	}
	// TODO 更新文件下载的状态
}

// TODO:优化并发下载的情况，尤其是端口如何处理，服务端和客户端都要处理
func (fm *FileManager) DownloadFolder(folderName, savePath string) (string, error) {
	// 1. 创建HTTP服务器
	// 端口8080
	mux := http.NewServeMux()
	done := make(chan struct{})

	// 2. 设置处理函数（使用闭包捕获savePath）

	mux.HandleFunc("/receive", func(w http.ResponseWriter, r *http.Request) {
		utils.ReceiveDir(w, r, savePath)

		// 检查传输完成信号
		if fileType := r.Header.Get("FileType"); fileType == "completion" {
			logs.Info("File transfer completed")
			close(done)
		}
	})
	server := &http.Server{Addr: ":8920", Handler: mux}

	// 3. 在goroutine中启动服务器
	go func() {
		logs.Info("Start the file receiving service....")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server startup failed: %v", err)
		}
	}()

	// 4. 短时等待确保服务器已启动
	time.Sleep(100 * time.Millisecond)

	// 5. 发送下载请求
	downloadURL := fm.DownloadURL + folderName

	// 创建 GET 请求
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		log.Fatal("Failed to create request:", err)
	}

	req.Header.Set("FileType", "folder")

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Request failed:", err)
	}
	defer resp.Body.Close()

	// 6. 等待传输完成或超时
	select {
	case <-done:
		logs.Info("Prepare to shut down the server...")
		if err := server.Shutdown(context.Background()); err != nil {
			logs.Errorf("Server shutdown error: %v", err)
		}
		logs.Infof("The file was successfully received and saved to %v", savePath)
		return "文件接收成功并保存至 " + savePath, nil
	case <-time.After(5 * time.Minute):
		if err := server.Shutdown(context.Background()); err != nil {
			logs.Errorf("Server shutdown error: %v", err)
		}
		logs.Errorf("folder download failed because timeout!")
		return "", errors.New("文件接收超时")
	}
}