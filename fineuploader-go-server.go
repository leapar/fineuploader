// An upload server for fineuploader.com javascript upload library
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"gopkg.in/mgo.v2"

	"bytes"
	"io/ioutil"
	"gopkg.in/mgo.v2/bson"
	"time"
)

var port = flag.Int("p", 8080, "Port number to listen to, defaults to 8080")
var uploadDir = flag.String("d", "uploads", "Upload directory, defaults to 'uploads'")

// Request parameters
const (
	paramUuid = "qquuid" // uuid
	paramFile = "qqfile" // file name
)

// Chunked request parameters
const (
	paramPartIndex       = "qqpartindex"      // part index
	paramPartBytesOffset = "qqpartbyteoffset" // part byte offset
	paramTotalFileSize   = "qqtotalfilesize"  // total file size
	paramTotalParts      = "qqtotalparts"     // total parts
	paramFileName        = "qqfilename"       // file name for chunked requests
	paramChunkSize       = "qqchunksize"      // size of the chunks
)

type UploadResponse struct {
	Success      bool   `json:"success"`
	Error        string `json:"error,omitempty"`
	PreventRetry bool   `json:"preventRetry"`
}
var gridFs *mgo.GridFS
func main() {
	flag.Parse()
	hostPort := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("Initiating server listening at [%s]", hostPort)
	log.Printf("Base upload directory set to [%s]", *uploadDir)
	http.HandleFunc("/upload", UploadHandler)
	http.HandleFunc("/chunksdone", ChunksDoneHandler)
	http.Handle("/upload/", http.StripPrefix("/upload/", http.HandlerFunc(UploadHandler)))
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})
	http.HandleFunc("/uploads/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})
	session, err := mgo.Dial("127.0.0.1:27017")
	if err != nil {
		log.Println("[grid]mgo.Dial ", err)
		return
	}

	database := session.DB("test")
	gridFs = database.GridFS("uploader")

/*
	file ,err := gridFs.Create("test.data")

	file.SetChunkSize(128)
	file.Write(make([]byte,1024))
	//file.Seek(5,os.SEEK_SET)
	//file.Write([]byte("test"))
	file.SetMeta(&struct{ A string
	B string } {
		A:"a",
		B:"b",
	})
	file.Close()*/
	defer session.Close()



	log.Fatal(http.ListenAndServe(hostPort, nil))
}

func UploadHandler(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		upload(w, req)
		return
	case http.MethodDelete:
		delete(w, req)
		return
	}
	errorMsg := fmt.Sprintf("Method [%s] is not supported:", req.Method)
	http.Error(w, errorMsg, http.StatusMethodNotAllowed)
}

type passthru struct {
	io.ReadCloser
	buf bytes.Buffer
}

func (p *passthru) Read(b []byte) (int, error) {
	n, err := p.ReadCloser.Read(b)
	p.buf.Write(b[:n])
	return n, err
}

func upload(w http.ResponseWriter, req *http.Request) {
	uuid := req.FormValue(paramUuid)
	if len(uuid) == 0 {
		log.Printf("No uuid received, invalid upload request")
		http.Error(w, "No uuid received", http.StatusBadRequest)
		return
	}
	log.Printf("Starting upload handling of request with uuid of [%s]\n", uuid)
	file, headers, err := req.FormFile(paramFile)
	if err != nil {
		writeUploadResponse(w, err)
		return
	}

	fileDir := fmt.Sprintf("%s/%s", *uploadDir, uuid)
	if err := os.MkdirAll(fileDir, 0777); err != nil {
		writeUploadResponse(w, err)
		return
	}

	var filename string
	partIndex := req.FormValue(paramPartIndex)
	if len(partIndex) == 0 {
		filename = fmt.Sprintf("%s/%s", fileDir, headers.Filename)

	} else {
		filename = fmt.Sprintf("%s/%s_%05s", fileDir, uuid, partIndex)
	}
	outfile, err := os.Create(filename)
	if err != nil {
		writeUploadResponse(w, err)
		return
	}
	defer outfile.Close()


	datas,err := ioutil.ReadAll(file)


	buf := bytes.NewBuffer(datas)
/*
	if _, err := io.Copy(buf, file); err != nil {
		return
	}
*/


	///_, err = io.Copy(outfile, file)
	_, err = io.Copy(outfile, buf)
	if err != nil {
		writeUploadResponse(w, err)
		return
	}
/*
	gridFile, err := gridFs.Open(req.FormValue(paramFileName))
	if err != nil {
		gridFile, err = gridFs.Create(req.FormValue(paramFileName))
		chunkSize,err := strconv.Atoi(req.FormValue(paramChunkSize))
		if err != nil {
			log.Println("strconv.Atoi(req.FormValue(paramChunkSize))")
		}
		gridFile.SetChunkSize(chunkSize)
		totalSize,err := strconv.Atoi(req.FormValue(paramTotalFileSize))
		totalSize = 1*1024*1024*1024
		if err != nil {
			log.Println("strconv.Atoi(req.FormValue(paramTotalFileSize))")
		}

		totalPart,err := strconv.Atoi(req.FormValue(paramTotalParts))
		totalPart = 512
		if err != nil {
			log.Println("strconv.Atoi(req.FormValue(paramTotalFileSize))")
		}
		for i := 0; i < totalPart - 1;  i++{
			gridFile.Write(make([]byte,chunkSize))
		}
		gridFile.Write(make([]byte,totalSize-chunkSize*(totalPart-1)))
		gridFile.Close()



	} else {
		gridFile.Close()
	}
*/
	if len(partIndex) == 0 {
		filename = fmt.Sprintf("%s",  headers.Filename)

	} else {
		filename = fmt.Sprintf("%s_%05s", uuid, partIndex)
	}
	gridFile, err := gridFs.Create(/*req.FormValue(paramFileName)*/filename)
	chunkSize,err := strconv.Atoi(req.FormValue(paramChunkSize))
	if err != nil {
		log.Println("strconv.Atoi(req.FormValue(paramChunkSize))")
		gridFile.SetChunkSize(len(datas))
	} else {
		gridFile.SetChunkSize(chunkSize)
	}


	//gridFile.Seek(int64(offset),os.SEEK_SET)


	totalSize,err := strconv.Atoi(req.FormValue(paramTotalFileSize))

	if err != nil {
		log.Println("strconv.Atoi(req.FormValue(paramTotalFileSize))")
		totalSize = len(datas)
	}

	totalPart,err := strconv.Atoi(req.FormValue(paramTotalParts))

	if err != nil {
		log.Println("strconv.Atoi(req.FormValue(paramTotalFileSize))")
		totalPart = 1
	}

	offset,err := strconv.Atoi(req.FormValue(paramPartBytesOffset))
	if err != nil {
		offset = 0
	}
	index,err := strconv.Atoi(partIndex)
	if err != nil {
		index = 0
	}
	gridFile.SetMeta(&struct{
		Index int
		Uuid string
		Filename string
		ChunkSize int
		TotalSize int
		TotalPart int
		Offset int} {
		Uuid:uuid,
		Filename:req.FormValue(paramFileName),
		ChunkSize: chunkSize,
		TotalSize:totalSize,
		TotalPart:totalPart,
		Offset:offset,
		Index:index,
	})
	gridFile.Write(datas)

	defer gridFile.Close()
	writeUploadResponse(w, nil)
}

func delete(w http.ResponseWriter, req *http.Request) {
	log.Printf("Delete request received for uuid [%s]", req.URL.Path)
	err := os.RemoveAll(*uploadDir + "/" + req.URL.Path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)

}

func ChunksDoneHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		errorMsg := fmt.Sprintf("Method [%s] is not supported", req.Method)
		http.Error(w, errorMsg, http.StatusMethodNotAllowed)
	}

	uuid := req.FormValue(paramUuid)
	filename := req.FormValue(paramFileName)

	totalFileSize, err := strconv.Atoi(req.FormValue(paramTotalFileSize))
	if err != nil {
		writeHttpResponse(w, http.StatusInternalServerError, err)
		return
	}
	totalParts, err := strconv.Atoi(req.FormValue(paramTotalParts))
	if err != nil {
		writeHttpResponse(w, http.StatusInternalServerError, err)
		return
	}

	finalFilename := fmt.Sprintf("%s/%s/%s", *uploadDir, uuid, filename)
	f, err := os.Create(finalFilename)
	if err != nil {
		writeHttpResponse(w, http.StatusInternalServerError, err)
		return
	}
	defer f.Close()

	type gfsFile struct {
		Id          interface{} "_id"
		ChunkSize   int         "chunkSize"
		UploadDate  time.Time   "uploadDate"
		Length      int64       ",minsize"
		MD5         string
		Filename    string    ",omitempty"
		ContentType string    "contentType,omitempty"
		Metadata    *bson.Raw ",omitempty"
	}
	var objFile []gfsFile
	gridFs.Files.Find(bson.M{ "filename": bson.RegEx{fmt.Sprintf("%s", uuid), "i"}}).All(&objFile)
	//count,err := gridFs.Files.Find(bson.M{ "filename": bson.RegEx{fmt.Sprintf("%s", uuid), "i"}}).Count()

	var ids map[bson.ObjectId]int
	ids = make(map[bson.ObjectId]int)
	var chunkSize int
	for key, value := range objFile {
		fmt.Println(key)
		result := struct {
			Index int
			Uuid string
			Filename string
			ChunkSize int
			TotalSize int
			TotalPart int
			Offset int
		}{}
		err = bson.Unmarshal(value.Metadata.Data, &result)
		fmt.Println(result)
		oid, ok := value.Id.(bson.ObjectId)
		if !ok  {
			fmt.Println(ok)
		}
		ids[oid] = result.Index

		if chunkSize == 0 {
			chunkSize = result.ChunkSize
		}


		err := gridFs.Files.Remove(bson.M{"_id": value.Id})
		if err != nil {
			fmt.Println(err)
		}


	}
	fmt.Println(ids)

	finalId := bson.NewObjectId()
	doneFile := &gfsFile {
		Id:finalId,
		UploadDate: bson.Now(),
		Length: int64(totalFileSize),
		ChunkSize: chunkSize,
		Filename:filename,
		MD5:uuid,
	}

	err = gridFs.Files.Insert(doneFile)
	if err != nil {
		fmt.Println(err)
	}

	for id, value := range ids {
		err := gridFs.Chunks.Update(bson.M{"files_id":id}, bson.M{"$set": bson.M{ "files_id": finalId,"n":value, }})
		if err != nil {
			fmt.Println(err)
		}
	}

	//gridFs.Chunks.UpdateAll()

	go ChunksDone(finalFilename, uuid, totalParts, totalFileSize)
}

func ChunksDone(finalFilename string, uuid string, totalParts int, totalFileSize int) {
	f, err := os.Create(finalFilename)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()

	var totalWritten int64
	for i := 0; i < totalParts; i++ {
		part := fmt.Sprintf("%[1]s/%[2]s/%[2]s_%05[3]d", *uploadDir, uuid, i)
		partFile, err := os.Open(part)
		if err != nil {
			log.Println(err)
			//writeHttpResponse(w, http.StatusInternalServerError, err)
			return
		}
		written, err := io.Copy(f, partFile)
		if err != nil {
			log.Println(err)
			//writeHttpResponse(w, http.StatusInternalServerError, err)
			return
		}
		partFile.Close()
		totalWritten += written

		if err := os.Remove(part); err != nil {
			log.Printf("Error: %v", err)
		}
	}

	if totalWritten != int64(totalFileSize) {
		errorMsg := fmt.Sprintf("Total file size mistmatch, expected %d bytes but actual is %d", totalFileSize, totalWritten)
		//http.Error(w, errorMsg, http.StatusMethodNotAllowed)
		log.Println(errorMsg)
	}
}

func writeHttpResponse(w http.ResponseWriter, httpCode int, err error) {
	w.WriteHeader(httpCode)
	if err != nil {
		log.Printf("An error happened: %v", err)
		w.Write([]byte(err.Error()))
	}
}

func writeUploadResponse(w http.ResponseWriter, err error) {
	uploadResponse := new(UploadResponse)
	if err != nil {
		uploadResponse.Error = err.Error()
	} else {
		uploadResponse.Success = true
	}
	w.Header().Set("Content-Type", "text/plain")
	json.NewEncoder(w).Encode(uploadResponse)
}
