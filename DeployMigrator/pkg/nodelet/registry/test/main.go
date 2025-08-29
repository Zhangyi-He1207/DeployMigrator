package main

func main() {
	// url := "http://localhost:8888/download?filename=test.txt"
	// savePath := "/home/public/goprojects/Combine-ysk-0102/tmp/data/downloads"

	// err := utils.DownloadFile(url, savePath)
	// if err != nil {
	// 	fmt.Println("Download failed:", err)
	// } else {
	// 	fmt.Println("Download successful!")
	// }

	// url := "http://localhost:8888/upload"
	// filePath := "./downloads/test.txt"
	// fmt.Println("aaaaaUploading file:", filePath)
	// err := utils.UploadFile(filePath, "v1.0.0", url)
	// fmt.Println("bbbbUploading file:", filePath)
	// if err != nil {
	// 	fmt.Println("Upload failed:", err)
	// } else {
	// 	fmt.Println("Upload successful!")
	// }

	// url := "http://localhost:8888/upload?filename=downloads"
	// filePath := "./downloads"
	// err := utils.Traverse(filePath, url)
	// if err != nil {
	// 	fmt.Println("Upload failed:", err)
	// } else {
	// 	fmt.Println("Upload successful!")
	// }

	// url := "http://localhost:8888/download?filename=downloads"

	// 发送 GET 请求
	// resp, err := http.Get(url)
	// if err != nil {
	// 	fmt.Println("Request failed:", err)
	// 	return
	// }
	// defer resp.Body.Close() // 确保响应体被关闭

	// // filePath := "./test"
	// http.HandleFunc("/receive", func(w http.ResponseWriter, r *http.Request) {
	// 	utils.ReceiveDir(w, r, filePath)
	// })

	// port := ":8080"
	// log.Printf("Server is running on port %s", port)
	// if err := http.ListenAndServe(port, nil); err != nil {
	// 	log.Fatalf("Failed to start server: %v", err)
	// }

}
