package utils

import (
	"net/http"
	"encoding/json"
	"strconv"
)

type UploadResponse struct {
	Success      bool   `json:"success"`
	Error        string `json:"error,omitempty"`
	PreventRetry bool   `json:"preventRetry"`
}

func WriteUploadResponse(w http.ResponseWriter, err error) {
	uploadResponse := new(UploadResponse)
	if err != nil {
		uploadResponse.Error = err.Error()
	} else {
		uploadResponse.Success = true
	}
	w.Header().Set("Content-Type", "text/plain")
	json.NewEncoder(w).Encode(uploadResponse)
}

func WriteDownloadHeader(w http.ResponseWriter,filename string,size int) {
	w.Header().Add("Accept-Ranges", "bytes")
	w.Header().Add("Content-Length", strconv.Itoa(size))
	//fmt.Println(strconv.Itoa(size))
	w.Header().Add("Content-Disposition", "attachment;filename="+filename)
}