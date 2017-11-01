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

	"github.com/NYTimes/gziphandler"
	"sync"
	"io"


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


	DATA_BASE = "fileserver"
	PREFIX = "uploader"
)

var (
	reqUploadTime = metrics.NewRegisteredGaugeFloat64("upload.request.time", metrics.DefaultRegistry)
	writeChunkTime = metrics.NewRegisteredGaugeFloat64("upload.write.chunk.time", metrics.DefaultRegistry)
	uploadICPTime = metrics.NewRegisteredGaugeFloat64("upload.io.copy.time", metrics.DefaultRegistry)

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
	//gridFs *mgo.GridFS
	lock sync.Mutex
	bufferPool BufferPool
}

func New(port int,host string,mongo string,dir string) *HttpServer {
	return &HttpServer{
		host:host,
		port:port,
		dir:dir,
		mongo:mongo,
		bufferPool: NewSyncPool(64),
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
	index := mgo.Index{
		Key:    []string{"files_id", "n"},
		Unique: true,
	}

	database := session.DB(DATA_BASE)
	gridFs := database.GridFS(PREFIX)
	gridFs.Chunks.EnsureIndex(index)

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
	http.Handle("/files",gziphandler.GzipHandler( http.HandlerFunc(srv.DownloadHandler)))
	//http.HandleFunc("/files",srv.DownloadHandler)
	http.HandleFunc("/metrics",srv.Metrics)
	hostPort := fmt.Sprintf("%s:%d",srv.host, srv.port)
	log.Printf("Initiating server listening at [%s]", hostPort)
	log.Printf("Base upload directory set to [%s]", srv.dir)

	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 5*time.Second)

	//influxdb.InfluxDB(metrics.DefaultRegistry, 10e9, "127.0.0.1:8086","metrics", "test", "test" )

	log.Fatal(http.ListenAndServe(hostPort, nil))


}


func (srv *HttpServer)writeDownloadHeader(w http.ResponseWriter,filename string,size int) {
	w.Header().Add("Accept-Ranges", "bytes")
	w.Header().Add("Content-Length", strconv.Itoa(size))
	w.Header().Add("Content-Disposition", "attachment;filename="+filename)
}

func (srv *HttpServer) DownloadHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		errorMsg := fmt.Sprintf("Method [%s] is not supported:", req.Method)
		http.Error(w, errorMsg, http.StatusMethodNotAllowed)

		return
	}

	fileId := req.FormValue("id")
	var objFile gfsFile

	session := srv.session.Copy()
	defer session.Close()
	database := session.DB(DATA_BASE)
	gridFs := database.GridFS(PREFIX)

	err := gridFs.Files.Find(bson.M{ "md5":fileId }).One(&objFile)
	if err != nil {
		//http.Error(w, "no file", http.StatusNotFound)
		fmt.Println(err)
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
			srv.writeDownloadHeader(w,objFile.Filename,size)

			w.Header().Add("Content-Range", "bytes "+strconv.Itoa(startPos)+"-"+strconv.Itoa(endPos-1)+"/"+strconv.Itoa(int(objFile.Length)))
			w.WriteHeader(http.StatusOK)
			return
		}

		//开始计算长度

		partSize := int(math.Floor(float64(float64(startPos) / float64(objFile.ChunkSize))))

		var write int = 0

		w.Header().Add("Content-Range", "bytes "+strconv.Itoa(startPos)+"-"+strconv.Itoa(endPos-1)+"/"+strconv.Itoa(int(objFile.Length)))
		w.WriteHeader(http.StatusPartialContent)

		i := partSize
		for ; write < size;  {
			var index int = 0
			var count int = 0

			if i == partSize{
				index = startPos - i*objFile.ChunkSize
			}

			var chunk gfsChunk



			err = gridFs.Chunks.Find(bson.D{{"files_id", objFile.Id}, {"n", i}}).One(&chunk)
			//err = srv.gridFs.Chunks.Find(bson.M{ "files_id":objFile.Id }).Sort("n").All(&chunks)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}



			if len(chunk.Data) - index  <= size - write {
				count = len(chunk.Data) - index
			} else {
				count = size - write
			}

			w.Write(chunk.Data[index:index+count])
			write += count
			i++;

		}
		srv.writeDownloadHeader(w,objFile.Filename,size)




		return
	}
	var chunk gfsChunk
	srv.writeDownloadHeader(w,objFile.Filename,int(objFile.Length))

	w.WriteHeader(http.StatusOK)
	for i := 0; i < int(math.Ceil(float64(objFile.Length) / float64(objFile.ChunkSize)));i++  {



		err = gridFs.Chunks.Find(bson.D{{"files_id", objFile.Id}, {"n", i}}).One(&chunk)
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(chunk.Data)
	}

	/*
	//内存会有问题
	err = srv.gridFs.Chunks.Find(bson.M{ "files_id":objFile.Id }).Sort("n").All(&chunks)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, chunk := range chunks {
		w.Write(chunk.Data)
	}*/

	return
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

	buf := srv.bufferPool.GetBuffer()
	if len(buf.Bytes()) > 0 {
		fmt.Println(len(buf.Bytes()))
	}

	defer srv.bufferPool.PutBuffer(buf)
	io.Copy(buf,file)


//	fmt.Println(nr)
	/*io.CopyBuffer()

	io.Copy()
	*/
	//srv.writeUploadResponse(w, err)
	//return
	//datas,err := ioutil.ReadAll(file)

	chunkSize,err := strconv.Atoi(req.FormValue(paramChunkSize))
	totalSize,err := strconv.Atoi(req.FormValue(paramTotalFileSize))
	index,err := strconv.Atoi(req.FormValue(paramPartIndex))
	//fmt.Println(len(buf.Bytes()))
	id := srv.writeChunks(req,index,buf.Bytes(),uuid,chunkSize,totalSize,req.FormValue(paramFileName))





	expires := time.Now().AddDate(1, 0, 0)
	ck := http.Cookie{
		Name: uuid,
		//Domain:  srv.host,//fmt.Sprintf("%s:%d",srv.host, srv.port),
		Path: "/",
		Expires: expires,
	}
	// value of cookie
	ck.Value = id
	// write the cookie to response
	http.SetCookie(w, &ck)


	srv.writeUploadResponse(w, nil)
	reqUploadOkCount.Inc(1)

}

func (srv *HttpServer)getFinalFileID(req *http.Request,uuid string,chunkSize int,totalFileSize int,filename string) interface{}{

	//
	//

	cookie, err := req.Cookie(uuid)
	if err == nil && cookie.Value != "" && bson.IsObjectIdHex(cookie.Value ){
		//fmt.Println(cookie)
		return bson.ObjectIdHex(cookie.Value)
	} else {
		//fmt.Println(cookie)
	}
	srv.lock.Lock()
	defer srv.lock.Unlock()
	var objFile gfsFile
	//err := srv.gridFs.Files.Find(bson.M{ "filename": bson.RegEx{fmt.Sprintf("%s#%s", uuid,filename), "i"}}).One(&objFile)
	session := srv.session.Copy()
	defer session.Close()
	database := session.DB(DATA_BASE)
	gridFs := database.GridFS(PREFIX)


	err = gridFs.Files.Find(bson.M{ "md5": uuid}).One(&objFile)

	if err == nil {
		return objFile.Id
	}
	finalId := bson.NewObjectId()

	objFile = gfsFile {
		Id:finalId,
		UploadDate: bson.Now(),
		Length: int64(totalFileSize),
		ChunkSize: chunkSize,
		Filename:fmt.Sprintf("%s", filename),
		MD5:uuid,
	}


	err = gridFs.Files.Insert(objFile)
	if err != nil {
		DBErrCount.Inc(1)
		log.Println("Files.Insert",err)
		panic(err)
	}

	return finalId
}

var bytesBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 256)
	},
}


func (srv*HttpServer)writeChunks(req *http.Request,index int,datas []byte,uuid string,chunkSize int,totalSize int,filename string) string {

	start := time.Now().UnixNano()
	defer func() {
		writeChunkTime.Update((float64(time.Now().UnixNano() - start) / float64(1e9)))
	}()

	// We may not own the memory of data, so rather than
	// simply copying it, we'll marshal the document ahead of time.
	fileid := srv.getFinalFileID(req,uuid,chunkSize,totalSize,filename)
	id,_ := fileid.(bson.ObjectId)

	buf := bytesBufferPool.Get().([]byte)
	defer func() {
		bytesBufferPool.Put(buf[:0])
	}()

	//fmt.Printf("%d %v %s\n",len(datas),unsafe.Pointer(&datas),"  in")
	data, err := bson.MarshalBuffer(gfsChunk{bson.NewObjectId(), fileid, index, datas},buf)
	//fmt.Printf("%d %v %s\n",len(datas),unsafe.Pointer(&datas),"  out")

	if err != nil {
		log.Println(err)
		panic(err)
		return id.Hex()
	}

	session := srv.session.Copy()
	defer session.Close()
	database := session.DB(DATA_BASE)
	gridFs := database.GridFS(PREFIX)


	//gridFs.Chunks.Remove(bson.M{"files_id":fileid,"n":index})
	err = gridFs.Chunks.Insert(bson.Raw{Data: data})

	if err != nil{
		//fmt.Println("E11000")
		if strings.LastIndex(err.Error(),"E11000") != -1 {
			gridFs.Chunks.Remove(bson.M{"files_id":fileid,"n":index})
			err = gridFs.Chunks.Insert(bson.Raw{Data: data})
			if err != nil{
				log.Println("retry",err)
				DBErrCount.Inc(1)
				panic(err)
			}
		}else {
			log.Println(err)
			DBErrCount.Inc(1)
			panic(err)
		}
		//192.168.0.135

	}
	return id.Hex()
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



	session := srv.session.Copy()
	defer session.Close()
	database := session.DB(DATA_BASE)
	gridFs := database.GridFS(PREFIX)


	gridFile, err := gridFs.Create(filename)
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
	start := time.Now().UnixNano()
	defer func() {
		reqUploadTime.Update((float64(time.Now().UnixNano() - start) / float64(1e9)))

		if r := recover(); r != nil {
			fmt.Println("panic upload error")
			reqUploadErrCount.Inc(1)
		}
	}()

	start2 := time.Now().UnixNano()

	reqUploadCount.Inc(1)
	//atomic.AddInt64(&reqUploadCount,1)
	req.ParseMultipartForm(64)
	partIndex := req.FormValue(paramPartIndex)
	end2 := time.Now().UnixNano()
	uploadICPTime.Update(float64(end2-start2) / float64(1e9))

	if len(partIndex) == 0 {
		srv.singleFile(w,req)
		return
	}
	
	srv.multiFile(w,req)
	return
}



func (srv *HttpServer)ChunksDoneHandler(w http.ResponseWriter, req *http.Request) {
	uuid := req.FormValue(paramUuid)
	defer func() {
		expires := time.Now().AddDate(-1, 0, 0)
		ck := http.Cookie{
			Name: uuid,
			//Domain:  srv.host,//fmt.Sprintf("%s:%d",srv.host, srv.port),
			Path: "/",
			Expires: expires,
		}
		// value of cookie
		ck.Value = ""
		// write the cookie to response
		http.SetCookie(w, &ck)

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

	filename := req.FormValue(paramFileName)

	if uuid == ""  || filename == ""{
		reqUploadDoneParamErrCount.Inc(1)
		return
	}

	session := srv.session.Copy()
	defer session.Close()
	database := session.DB(DATA_BASE)
	gridFs := database.GridFS(PREFIX)

	var objFile []gfsFile

	//err := srv.gridFs.Files.Find(bson.M{ "filename": bson.RegEx{fmt.Sprintf("%s#%s", uuid,filename), "i"}}).One(&objFile)
	//count,err := gridFs.Files.Find(bson.M{ "filename": bson.RegEx{fmt.Sprintf("%s", uuid), "i"}}).Count()
	err := gridFs.Files.Find(bson.M{ "md5": uuid}).All(&objFile)
	if err != nil {
		fmt.Println("Files.Find",err)
		DBErrCount.Inc(1)
		panic(err)
	}

	if len(objFile) == 0 {
		cookie, err := req.Cookie(uuid)
		if err == nil && cookie.Value != "" && bson.IsObjectIdHex(cookie.Value ){
			fmt.Println("no file")
			gridFs.Chunks.RemoveAll(bson.M{"files_id":bson.ObjectIdHex(cookie.Value)})
			panic(err)
		} else {
			fmt.Println("no file and no cookie")
			//fmt.Println(cookie)
			panic(err)
		}
	}

	firstFile := objFile[0]
	if len(objFile) > 1 {
		log.Println("len(objFile)")
		//并发上传时候 第一次有可能并发导致创建了多个文件 这时候需要清理  ，当然创建文件时候才有锁机制也可以，但是性能不行
		for i := 1; i < len(objFile);i++  {
			file := objFile[i]
			err := gridFs.Chunks.Update(bson.M{"files_id":file.Id}, bson.M{"$set": bson.M{ "files_id":  firstFile.Id, }})
			if err != nil {
				fmt.Println("Chunks.Update",err)
				DBErrCount.Inc(1)
				panic(err)
			}
			err = gridFs.Files.Remove(bson.M{"_id":file.Id})
			if err != nil {
				fmt.Println("Files.Remove",err)
				DBErrCount.Inc(1)
				panic(err)
			}
		}
	}

	err = gridFs.Files.Update(bson.M{"_id":firstFile.Id}, bson.M{"$set": bson.M{ "uploadDate":  bson.Now(), }})
	if err != nil {
		fmt.Println("Files.Update",err)
		DBErrCount.Inc(1)
		panic(err)
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
