package Controller

import (
	utils "hit.edu/framework/pkg/nodelet/registry/Utils"
	"net/http"
)

func Receive(w http.ResponseWriter, r *http.Request, baseDir string) {
	fileType := r.Header.Get("FileType")
	switch fileType {
	case "file":
		utils.ReceiveFile(w, r, baseDir)
	case "folder":
		utils.ReceiveDir(w, r, baseDir)
	default:
		http.Error(w, "Invalid FileType", http.StatusBadRequest)
	}
}
