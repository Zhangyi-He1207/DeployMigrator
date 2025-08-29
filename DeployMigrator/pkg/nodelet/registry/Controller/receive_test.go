package Controller

import (
	utils "hit.edu/framework/pkg/nodelet/registry/Utils"
	"fmt"
	"testing"
)

func TestReceive(t *testing.T) {
	url := "http://localhost:8081/upload?filename=downloads"
	filePath := "./downloads"
	err := utils.Traverse(filePath, url)
	if err != nil {
		fmt.Println("Upload failed:", err)
	} else {
		fmt.Println("Upload successful!")
	}
}
