package server

import (
	"net/http"
	"fmt"
	"log"
	"os"
	"io/ioutil"
	"strconv"
	"time"
	"gopkg.in/mgo.v2/bson"
	"encoding/json"
	"gopkg.in/mgo.v2"
	"github.com/rcrowley/go-metrics"
	 //"github.com/vrischmann/go-metrics-influxdb"
	_ "net/http/pprof"
	"strings"
	"math"
)

type UploadResponse struct {
	Success      bool   `json:"success"`
	Error        string `json:"error,omitempty"`
	PreventRetry bool   `json:"preventRetry"`
}

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

type gfsChunk struct {
	Id      interface{} "_id"
	FilesId interface{} "files_id"
	N       int
	Data    []byte
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

var (
	reqUploadCount = metrics.NewRegisteredCounter("upload.request", metrics.DefaultRegistry)
	reqUploadOkCount = metrics.NewRegisteredCounter("upload.request.ok", metrics.DefaultRegistry)
	reqUploadErrCount = metrics.NewRegisteredCounter("upload.request.error", metrics.DefaultRegistry)
	reqUploadParamErrCount = metrics.NewRegisteredCounter("upload.request.params.error", metrics.DefaultRegistry)

	reqUploadDoneCount = metrics.NewRegisteredCounter("uploaddone.request", metrics.DefaultRegistry)
	reqUploadDoneOkCount = metrics.NewRegisteredCounter("uploaddone.request.ok", metrics.DefaultRegistry)
	reqUploadDoneErrCount = metrics.NewRegisteredCounter("uploaddone.request.error", metrics.DefaultRegistry)
	reqUploadDoneParamErrCount = metrics.NewRegisteredCounter("uploaddone.request.params.error", metrics.DefaultRegistry)

	DBErrCount = metrics.NewRegisteredCounter("db.error", metrics.DefaultRegistry)


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


func (srv *HttpServer) EnsureConnected(){
	defer func() {
		if r := recover(); r != nil {
			log.Println("ReConnect....")
			//Your reconnect logic here.
			session, err := mgo.Dial(srv.mongo)
			if err != nil {
				log.Println("[grid]mgo.Dial ", err)
				panic(err)
				return
			}
			srv.session = session

			database := session.DB("test")
			srv.gridFs = database.GridFS("uploader")
			log.Println("ReConnect ok")
		}
	}()

	//Ping panics if session is closed. (see mgo.Session.Panic())
	err := srv.session.Ping()
	if err != nil {
		srv.session.Close()
		log.Println("ping:",err)
		DBErrCount.Inc(1)
		panic(err)
	}
}


func (srv *HttpServer) Start() {
	session, err := mgo.Dial(srv.mongo)

	log.Println("dial mongodb:",srv.mongo)
	if err != nil {
		log.Println("[grid]mgo.Dial ", err)
		return
	}
	srv.session = session
	defer srv.session.Close()

	database := session.DB("test")
	srv.gridFs = database.GridFS("uploader")

	http.HandleFunc("/upload",srv.UploadHandler)
	http.HandleFunc("/chunksdone", srv.ChunksDoneHandler)
	http.Handle("/upload/", http.StripPrefix("/upload/", http.HandlerFunc(srv.UploadHandler)))
	http.HandleFunc("/res/", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/res/", Assets(AssetsOpts{
			Develop:false,
		})).ServeHTTP(w, r)
	})
	http.HandleFunc("/uploads/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})
	http.HandleFunc("/files",srv.DownloadHandler)
	http.HandleFunc("/metrics",srv.Metrics)
	hostPort := fmt.Sprintf("%s:%d",srv.host, srv.port)
	log.Printf("Initiating server listening at [%s]", hostPort)
	log.Printf("Base upload directory set to [%s]", srv.dir)

	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 5*time.Second)

	//influxdb.InfluxDB(metrics.DefaultRegistry, 10e9, "127.0.0.1:8086","metrics", "test", "test" )

	log.Fatal(http.ListenAndServe(hostPort, nil))


}

func (srv *HttpServer) DownloadHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		errorMsg := fmt.Sprintf("Method [%s] is not supported:", req.Method)
		http.Error(w, errorMsg, http.StatusMethodNotAllowed)

		return
	}
	srv.EnsureConnected()
	fileId := req.FormValue("id")
	var objFile gfsFile
	err := srv.gridFs.Files.Find(bson.M{ "md5":fileId }).One(&objFile)
	if err != nil {
		//http.Error(w, "no file", http.StatusNotFound)
		w.WriteHeader(http.StatusNotFound)
		return
	}


	strRange := req.Header.Get("Range")
	if strRange != "" {
		strRange = strRange[6:]
		//debugLog.Println(strRange)
		posArr := strings.Split(strRange, "-")
		startPos := 0
		endPos := 0

		maxReadSize := objFile.ChunkSize
		if len(posArr) == 1 {
			startPos, _ = strconv.Atoi(posArr[0])

		} else if len(posArr) == 2 {
			startPos, _ = strconv.Atoi(posArr[0])
			endPos, _ = strconv.Atoi(posArr[1])
		}
		//debugLog.Println(len(posArr), startPos, endPos)
		if endPos == 0 {
			endPos = startPos + maxReadSize
			if endPos > int(objFile.Length) {
				endPos = int(objFile.Length)
			}
		} else {
			endPos = endPos + 1
		}

		size := endPos - startPos
		if size <= 0 {
			w.Header().Add("Accept-Ranges", "bytes")
			w.Header().Add("Content-Length", strconv.Itoa(size))
			w.Header().Add("Content-Range", "bytes "+strconv.Itoa(startPos)+"-"+strconv.Itoa(endPos-1)+"/"+strconv.Itoa(int(objFile.Length)))
			w.WriteHeader(http.StatusOK)
			return
		}


		var chunks []gfsChunk
		err = srv.gridFs.Chunks.Find(bson.M{ "files_id":objFile.Id }).Sort("n").All(&chunks)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		//开始计算长度

		partSize := int(math.Floor(float64(float64(startPos) / float64(objFile.ChunkSize))))
	//	fmt.Println(strRange,partSize)


		buffer := make([]byte,size)

//		fmt.Println(len(chunks))
		var write int = 0

		i := partSize
		for ; write < size;  {
			var index int = 0
			var count int = 0

			if i == partSize{
				index = startPos - i*objFile.ChunkSize
			}

			if len(chunks[i].Data) - index  <= size - write {
				count = len(chunks[i].Data) - index
			} else {
				count = size - write
			}

			copy(buffer[write:],chunks[i].Data[index:index+count])
			write += count
		//	io.CopyN(buf,chunks[i].Data,100)
			i++;

		}
		w.Header().Add("Accept-Ranges", "bytes")
		w.Header().Add("Content-Length", strconv.Itoa(size))
		w.Header().Add("Content-Range", "bytes "+strconv.Itoa(startPos)+"-"+strconv.Itoa(endPos-1)+"/"+strconv.Itoa(int(objFile.Length)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(buffer)

		//w.Header().Add("Content-Type", contentType)

		//w.Header().Add("Accept-Ranges", "bytes")
	//	w.Header().Add("Content-Length", strconv.Itoa(int(objFile.Length)))
	//	w.Header().Add("Content-Range", "bytes "+strconv.Itoa(startPos)+"-"+strconv.Itoa(endPos-1)+"/"+strconv.Itoa(int(fileSize)))

	//	w.Header().Add("")
	//	w.WriteHeader(http.StatusOK)
	//	w.Write([]byte("ss"))
		return
	}
	var chunks []gfsChunk
	err = srv.gridFs.Chunks.Find(bson.M{ "files_id":objFile.Id }).Sort("n").All(&chunks)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Accept-Ranges", "bytes")
	w.Header().Add("Content-Length", strconv.Itoa(int(objFile.Length)))

	w.WriteHeader(http.StatusOK)
	for _, chunk := range chunks {
		w.Write(chunk.Data)
	}

	return
	//srv.EnsureConnected()
	//srv.gridFs.Files.Find()
}

func (srv *HttpServer) Metrics(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("content-type", "application/json")
	b, _ := metrics.DefaultRegistry.(*metrics.StandardRegistry).MarshalJSON()

	w.Write(b)
//	metrics.WriteJSONOnce(metrics.DefaultRegistry,w)
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
		reqUploadParamErrCount.Inc(1)
		return
	}

	file, headers, err := req.FormFile(paramFile)
	if err != nil {
		log.Println(err)
		srv.writeUploadResponse(w, err)
		reqUploadParamErrCount.Inc(1)
		return
	}
	defer file.Close()

	datas,err := ioutil.ReadAll(file)

	filename := fmt.Sprintf("%s",  headers.Filename)
	totalSize := len(datas)
	totalPart := 1
	offset := 0
	index := 0

	srv.writeGridFile(filename,req.FormValue(paramFileName),uuid,totalSize,totalSize,totalPart,offset,index,datas)
	srv.writeUploadResponse(w, nil)

	reqUploadOkCount.Inc(1)
}

func (srv *HttpServer)multiFile(w http.ResponseWriter, req *http.Request) {
	uuid := req.FormValue(paramUuid)
	if len(uuid) == 0 {
		log.Printf("No uuid received, invalid upload request")
		http.Error(w, "No uuid received", http.StatusBadRequest)
		reqUploadParamErrCount.Inc(1)
		return
	}

	file, _, err := req.FormFile(paramFile)
	if err != nil {
		log.Println(err)
		srv.writeUploadResponse(w, err)
		reqUploadParamErrCount.Inc(1)
		return
	}
	defer file.Close()
	datas,err := ioutil.ReadAll(file)

	chunkSize,err := strconv.Atoi(req.FormValue(paramChunkSize))
	totalSize,err := strconv.Atoi(req.FormValue(paramTotalFileSize))
	index,err := strconv.Atoi(req.FormValue(paramPartIndex))
	srv.writeChunks(index,datas,uuid,chunkSize,totalSize,req.FormValue(paramFileName))

	srv.writeUploadResponse(w, nil)
	reqUploadOkCount.Inc(1)
}

func (srv *HttpServer)getFinalFileID(uuid string,chunkSize int,totalFileSize int,filename string) interface{}{
	srv.EnsureConnected()

	var objFile gfsFile
	err := srv.gridFs.Files.Find(bson.M{ "filename": bson.RegEx{fmt.Sprintf("%s#%s", uuid,filename), "i"}}).One(&objFile)
	if err == nil {
		return objFile.Id
	}
	finalId := bson.NewObjectId()

	objFile = gfsFile {
		Id:finalId,
		UploadDate: bson.Now(),
		Length: int64(totalFileSize),
		ChunkSize: chunkSize,
		Filename:fmt.Sprintf("%s#%s", uuid,filename),
		MD5:uuid,
	}
	err = srv.gridFs.Files.Insert(objFile)
	if err != nil {
		DBErrCount.Inc(1)
		log.Println("Files.Insert",err)
		panic(err)
	}

	return finalId
}

func (srv*HttpServer)writeChunks(index int,datas []byte,uuid string,chunkSize int,totalSize int,filename string)  {


	// We may not own the memory of data, so rather than
	// simply copying it, we'll marshal the document ahead of time.
	fileid := srv.getFinalFileID(uuid,chunkSize,totalSize,filename)
	data, err := bson.Marshal(gfsChunk{bson.NewObjectId(), fileid, index, datas})
	if err != nil {
		log.Println(err)
		panic(err)
		return
	}
	srv.gridFs.Chunks.Remove(bson.M{"files_id":fileid,"n":index})
	err = srv.gridFs.Chunks.Insert(bson.Raw{Data: data})

	if err != nil{

		log.Println(err)
		DBErrCount.Inc(1)
		panic(err)
	}
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

	srv.EnsureConnected()
	gridFile, err := srv.gridFs.Create(filename)
	if err != nil {
		DBErrCount.Inc(1)
		log.Println(err)
		panic(err)
		return
	}
	defer gridFile.Close()
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
	_,err = gridFile.Write(datas)
	if err != nil{
		DBErrCount.Inc(1)
		panic(err)
	}


}

func (srv *HttpServer)upload(w http.ResponseWriter, req *http.Request) {
	defer func() {

		if r := recover(); r != nil {
			fmt.Println("panic upload error")
			reqUploadErrCount.Inc(1)
		}
	}()

	reqUploadCount.Inc(1)
	//atomic.AddInt64(&reqUploadCount,1)
	partIndex := req.FormValue(paramPartIndex)
	if len(partIndex) == 0 {
		srv.singleFile(w,req)
		return
	}
	srv.multiFile(w,req)
	return
}

func (srv *HttpServer)ChunksDoneHandler(w http.ResponseWriter, req *http.Request) {

	defer func() {

		if r := recover(); r != nil {
			fmt.Println("panic upload done error")
			reqUploadDoneErrCount.Inc(1)
		}
	}()


	if req.Method != http.MethodPost {
		errorMsg := fmt.Sprintf("Method [%s] is not supported", req.Method)
		http.Error(w, errorMsg, http.StatusMethodNotAllowed)
	}
	reqUploadDoneCount.Inc(1)
	uuid := req.FormValue(paramUuid)
	filename := req.FormValue(paramFileName)

	if uuid == ""  || filename == ""{
		reqUploadDoneParamErrCount.Inc(1)
		return
	}


	var objFile gfsFile
	srv.EnsureConnected()
	err := srv.gridFs.Files.Find(bson.M{ "filename": bson.RegEx{fmt.Sprintf("%s#%s", uuid,filename), "i"}}).One(&objFile)
	//count,err := gridFs.Files.Find(bson.M{ "filename": bson.RegEx{fmt.Sprintf("%s", uuid), "i"}}).Count()
	if err != nil {
		DBErrCount.Inc(1)
		reqUploadDoneErrCount.Inc(1)
	}

	err = srv.gridFs.Files.Update(bson.M{"_id":objFile.Id}, bson.M{"$set": bson.M{ "uploadDate":  bson.Now(), }})
	if err != nil {
		DBErrCount.Inc(1)
		reqUploadDoneErrCount.Inc(1)
	}
	reqUploadDoneOkCount.Inc(1)

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
