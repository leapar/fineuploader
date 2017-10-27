package server

import (
	"net/http"
	"fmt"
	"log"
	"os"
	"io/ioutil"
	"bytes"
	"io"
	"strconv"
	"time"
	"gopkg.in/mgo.v2/bson"
	"encoding/json"
	"gopkg.in/mgo.v2"
)

type UploadResponse struct {
	Success      bool   `json:"success"`
	Error        string `json:"error,omitempty"`
	PreventRetry bool   `json:"preventRetry"`
}

// Chunked request parameters
const (
	paramUuid = "qquuid" // uuid
	paramFile = "qqfile" // file name

	paramPartIndex       = "qqpartindex"      // part index
	paramPartBytesOffset = "qqpartbyteoffset" // part byte offset
	paramTotalFileSize   = "qqtotalfilesize"  // total file size
	paramTotalParts      = "qqtotalparts"     // total parts
	paramFileName        = "qqfilename"       // file name for chunked requests
	paramChunkSize       = "qqchunksize"      // size of the chunks
)

type HttpServer struct {
	host string
	port int
	dir string
	mongo string
	session *mgo.Session
	gridFs *mgo.GridFS
}

func New(port int,host string,mongo string,dir string) *HttpServer {
	return &HttpServer{
		host:host,
		port:port,
		dir:dir,
		mongo:mongo,
	}
}

func (srv *HttpServer) Start() {
	session, err := mgo.Dial(srv.mongo)
	if err != nil {
		log.Println("[grid]mgo.Dial ", err)
		return
	}
	srv.session = session
	defer session.Close()

	database := session.DB("test")
	srv.gridFs = database.GridFS("uploader")

	http.HandleFunc("/upload",srv.UploadHandler)
	http.HandleFunc("/chunksdone", srv.ChunksDoneHandler)
	http.Handle("/upload/", http.StripPrefix("/upload/", http.HandlerFunc(srv.UploadHandler)))
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})
	http.HandleFunc("/uploads/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})
	hostPort := fmt.Sprintf("%s:%d",srv.host, srv.port)
	log.Printf("Initiating server listening at [%s]", hostPort)
	log.Printf("Base upload directory set to [%s]", srv.dir)
	log.Fatal(http.ListenAndServe(hostPort, nil))
}

func (srv *HttpServer) UploadHandler(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		srv.upload(w, req)
		return
	case http.MethodDelete:
		srv.delete(w, req)
		return
	}
	errorMsg := fmt.Sprintf("Method [%s] is not supported:", req.Method)
	http.Error(w, errorMsg, http.StatusMethodNotAllowed)
}

func (srv *HttpServer)delete(w http.ResponseWriter, req *http.Request) {
	log.Printf("Delete request received for uuid [%s]", req.URL.Path)
	err := os.RemoveAll(srv.dir + "/" + req.URL.Path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
}


func (srv *HttpServer)singleFile(w http.ResponseWriter, req *http.Request) {
	uuid := req.FormValue(paramUuid)
	if len(uuid) == 0 {
		log.Printf("No uuid received, invalid upload request")
		http.Error(w, "No uuid received", http.StatusBadRequest)
		return
	}
	log.Printf("Starting upload handling of request with uuid of [%s]\n", uuid)
	file, headers, err := req.FormFile(paramFile)
	if err != nil {
		srv.writeUploadResponse(w, err)
		return
	}

	fileDir := fmt.Sprintf("%s/%s", srv.dir, uuid)
	if err := os.MkdirAll(fileDir, 0777); err != nil {
		srv.writeUploadResponse(w, err)
		return
	}

	var filename string
	filename = fmt.Sprintf("%s/%s", fileDir, headers.Filename)

	outfile, err := os.Create(filename)
	if err != nil {
		srv.writeUploadResponse(w, err)
		return
	}
	defer outfile.Close()

	datas,err := ioutil.ReadAll(file)
	buf := bytes.NewBuffer(datas)

	///_, err = io.Copy(outfile, file)
	_, err = io.Copy(outfile, buf)
	if err != nil {
		srv.writeUploadResponse(w, err)
		return
	}


	filename = fmt.Sprintf("%s",  headers.Filename)
	totalSize := len(datas)
	totalPart := 1
	offset := 0
	index := 0

	srv.writeGridFile(filename,req.FormValue(paramFileName),uuid,totalSize,totalSize,totalPart,offset,index,datas)

	srv.writeUploadResponse(w, nil)
}

func (srv *HttpServer)multiFile(w http.ResponseWriter, req *http.Request) {
	uuid := req.FormValue(paramUuid)
	if len(uuid) == 0 {
		log.Printf("No uuid received, invalid upload request")
		http.Error(w, "No uuid received", http.StatusBadRequest)
		return
	}
	log.Printf("Starting upload handling of request with uuid of [%s]\n", uuid)
	file, _, err := req.FormFile(paramFile)
	if err != nil {
		srv.writeUploadResponse(w, err)
		return
	}

	fileDir := fmt.Sprintf("%s/%s", srv.dir, uuid)
	if err := os.MkdirAll(fileDir, 0777); err != nil {
		srv.writeUploadResponse(w, err)
		return
	}
	partIndex := req.FormValue(paramPartIndex)
	var filename string
	filename = fmt.Sprintf("%s/%s_%05s", fileDir, uuid, partIndex)
	outfile, err := os.Create(filename)
	if err != nil {
		srv.writeUploadResponse(w, err)
		return
	}
	defer outfile.Close()

	datas,err := ioutil.ReadAll(file)
	buf := bytes.NewBuffer(datas)

	///_, err = io.Copy(outfile, file)
	_, err = io.Copy(outfile, buf)
	if err != nil {
		srv.writeUploadResponse(w, err)
		return
	}

	filename = fmt.Sprintf("%s_%05s.part", uuid, partIndex)
	chunkSize,err := strconv.Atoi(req.FormValue(paramChunkSize))
	totalSize,err := strconv.Atoi(req.FormValue(paramTotalFileSize))
	totalPart,err :=  strconv.Atoi(req.FormValue(paramTotalParts))
	offset,err := strconv.Atoi(req.FormValue(paramPartBytesOffset))
	index,err := strconv.Atoi(req.FormValue(paramPartIndex))

	srv.writeGridFile(filename,req.FormValue(paramFileName),uuid,chunkSize,totalSize,totalPart,offset,index,datas)

	srv.writeUploadResponse(w, nil)
}

func (srv *HttpServer) writeGridFile(filename string,
	shortName string,
	uuid string,
	chunkSize int,
	totalSize int,
	totalPart int,
	offset int,
	index int,
	datas []byte)  {
	gridFile, err := srv.gridFs.Create(filename)
	if err != nil {
		log.Println(err)
		return
	}

	gridFile.SetChunkSize(chunkSize)

	gridFile.SetMeta(&struct{
		Index int
		Uuid string
		Filename string
		ChunkSize int
		TotalSize int
		TotalPart int
		Offset int} {
		Uuid:uuid,
		Filename:shortName,
		ChunkSize: chunkSize,
		TotalSize:totalSize,
		TotalPart:totalPart,
		Offset:offset,
		Index:index,
	})
	gridFile.Write(datas)

	defer gridFile.Close()
}



func (srv *HttpServer)upload(w http.ResponseWriter, req *http.Request) {
	partIndex := req.FormValue(paramPartIndex)
	if len(partIndex) == 0 {
		srv.singleFile(w,req)
		return
	}
	srv.multiFile(w,req)
	return
}

func (srv *HttpServer)ChunksDoneHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		errorMsg := fmt.Sprintf("Method [%s] is not supported", req.Method)
		http.Error(w, errorMsg, http.StatusMethodNotAllowed)
	}

	uuid := req.FormValue(paramUuid)
	filename := req.FormValue(paramFileName)

	totalFileSize, err := strconv.Atoi(req.FormValue(paramTotalFileSize))
	if err != nil {
		srv.writeHttpResponse(w, http.StatusInternalServerError, err)
		return
	}
	totalParts, err := strconv.Atoi(req.FormValue(paramTotalParts))
	if err != nil {
		srv.writeHttpResponse(w, http.StatusInternalServerError, err)
		return
	}

	finalFilename := fmt.Sprintf("%s/%s/%s", srv.dir, uuid, filename)
	f, err := os.Create(finalFilename)
	if err != nil {
		srv.writeHttpResponse(w, http.StatusInternalServerError, err)
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
	srv.gridFs.Files.Find(bson.M{ "filename": bson.RegEx{fmt.Sprintf("%s", uuid), "i"}}).All(&objFile)
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


		err := srv.gridFs.Files.Remove(bson.M{"_id": value.Id})
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

	err = srv.gridFs.Files.Insert(doneFile)
	if err != nil {
		fmt.Println(err)
	}

	for id, value := range ids {
		err := srv.gridFs.Chunks.Update(bson.M{"files_id":id}, bson.M{"$set": bson.M{ "files_id": finalId,"n":value, }})
		if err != nil {
			fmt.Println(err)
		}
	}

	//gridFs.Chunks.UpdateAll()

	go srv.ChunksDone(finalFilename, uuid, totalParts, totalFileSize)
}

func (srv *HttpServer)ChunksDone(finalFilename string, uuid string, totalParts int, totalFileSize int) {
	f, err := os.Create(finalFilename)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()

	var totalWritten int64
	for i := 0; i < totalParts; i++ {
		part := fmt.Sprintf("%[1]s/%[2]s/%[2]s_%05[3]d", srv.dir, uuid, i)
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

func (srv *HttpServer)writeHttpResponse(w http.ResponseWriter, httpCode int, err error) {
	w.WriteHeader(httpCode)
	if err != nil {
		log.Printf("An error happened: %v", err)
		w.Write([]byte(err.Error()))
	}
}

func (srv *HttpServer)writeUploadResponse(w http.ResponseWriter, err error) {
	uploadResponse := new(UploadResponse)
	if err != nil {
		uploadResponse.Error = err.Error()
	} else {
		uploadResponse.Success = true
	}
	w.Header().Set("Content-Type", "text/plain")
	json.NewEncoder(w).Encode(uploadResponse)
}
