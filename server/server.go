package server

import (
	"net/http"
	"fmt"
	"log"
	"time"
	"github.com/rcrowley/go-metrics"
	_ "net/http/pprof"

	"github.com/NYTimes/gziphandler"
	 "../output"
	"../storage"
	"../def"
	"../config"
	_"../output/plugins"
	_"../input/plugins"
	_"../storage/plugins"
	"../input"

	"github.com/valyala/bytebufferpool"
	"io"
	"../utils"
	"strconv"
	"gopkg.in/mgo.v2/bson"
	"strings"
)

type HttpServer struct {
	config config.Config
	outputer def.Outputer

}


func New(config config.Config) *HttpServer {
	return &HttpServer{
		config:config,
	}
}

func (srv *HttpServer) Start() {

	fmt.Println(output.Outputs)
	srv.config.Storage = storage.Storages[srv.config.StorageName](srv.config)
	srv.outputer = output.Outputs[srv.config.Output](srv.config)


	if srv.config.InputNsq.Enable {
		input.Iutputs["nsq"](srv.config)
	}
	/*srv.Init("127.0.0.1:4150")
	*/

	//提交上传文件信息，获取文件ID存入COOKIE
	http.HandleFunc("/preupload",srv.PreUploadHandler)
	//上传文件块
	http.HandleFunc("/upload",srv.UploadHandler)
	//上传完成
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
	hostPort := fmt.Sprintf("%s:%d",srv.config.Host, srv.config.Port)
	log.Printf("Initiating server listening at [%s]", hostPort)
	//log.Printf("Base upload directory set to [%s]", srv.dir)

	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 5*time.Second)

	//influxdb.InfluxDB(metrics.DefaultRegistry, 10e9, "127.0.0.1:8086","metrics", "test", "test" )

	log.Fatal(http.ListenAndServe(hostPort, nil))


}

func (srv *HttpServer) DownloadHandler(w http.ResponseWriter, req *http.Request) {
	srv.config.Storage.DownloadHandler(w,req)
}

func (srv *HttpServer) Metrics(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("content-type", "application/json")
	b, _ := metrics.DefaultRegistry.(*metrics.StandardRegistry).MarshalJSON()

	w.Write(b)
}

func (srv *HttpServer) PreUploadHandler(w http.ResponseWriter, req *http.Request) {

	switch req.Method {
	case http.MethodPost:
		srv.preupload(w, req)
		return
	case http.MethodDelete:
		//	srv.delete(w, req)
		return
	}
	errorMsg := fmt.Sprintf("Method [%s] is not supported:", req.Method)
	http.Error(w, errorMsg, http.StatusMethodNotAllowed)
}

func (srv *HttpServer)preupload(w http.ResponseWriter, req *http.Request) {
	defer func() {
		req.Body.Close()
	}()

	//atomic.AddInt64(&reqUploadCount,1)
	//req.ParseMultipartForm(64)
	forms := make(map[string]string)
	var blob   *bytebufferpool.ByteBuffer

	reader,err := req.MultipartReader()
	if  err != nil {
		fmt.Println(err)
		panic(err)
	}

	for {
		part, err_part := reader.NextPart()
		if err_part == io.EOF {
			break
		}
		if err_part != nil {
			panic(err_part)
			fmt.Println("err_part",err_part)
			break
		}
		name := strings.TrimSpace(part.FormName())
		if name == "" {
			continue
		}

		buf := bytebufferpool.Get()
		defer func() {
			bytebufferpool.Put(buf)
		}()

		if name !=  def.ParamFile {
			buf.ReadFrom(part)
			//log.Println(name,"  is: ", buf.String())
			forms[name] = buf.String()

		} else {
			//forms[def.ParamFile] = part.FileName()
			//log.Println(def.ParamFile,part.FileName())
			buf.ReadFrom(io.MultiReader(part))
			blob = buf

		}
	}

	srv.getID(w,req,&forms,blob)
	return
}

func (srv *HttpServer)getID(w http.ResponseWriter, req *http.Request,forms *map[string]string,blob*bytebufferpool.ByteBuffer) {
	uuid,ok := (*forms)[def.ParamUuid]
	if !ok || len(uuid) == 0 {
		log.Printf("No uuid received, invalid upload request")
		http.Error(w, "No uuid received", http.StatusBadRequest)
		return
	}

	chunkSize,err := strconv.Atoi((*forms)[def.ParamChunkSize])
	if err != nil {
		fmt.Println(err)
	}
	totalSize,err := strconv.Atoi((*forms)[def.ParamTotalFileSize])

	//fmt.Println(len(buf.Bytes()))
	cookie, err := req.Cookie(uuid)
	cookieVae := ""
	if err == nil {
		cookieVae = cookie.Value
	}
	id := srv.config.Storage.GetFinalFileID(cookieVae,uuid,chunkSize,totalSize,(*forms)[def.ParamFileName])
	expires := time.Now().AddDate(1, 0, 0)
	ck := http.Cookie{
		Name: uuid,
		//Domain:  srv.host,//fmt.Sprintf("%s:%d",srv.host, srv.port),
		Path: "/",
		Expires: expires,
	}
	// value of cookie
	file_id,_ := id.(bson.ObjectId)
	ck.Value = file_id.Hex()
	// write the cookie to response
	http.SetCookie(w, &ck)
	utils.WriteUploadResponse(w, nil)
}


func (srv *HttpServer) UploadHandler(w http.ResponseWriter, req *http.Request) {

	switch req.Method {
	case http.MethodPost:
		srv.upload(w, req)
		return
	case http.MethodDelete:
	//	srv.delete(w, req)
		return
	}
	errorMsg := fmt.Sprintf("Method [%s] is not supported:", req.Method)
	http.Error(w, errorMsg, http.StatusMethodNotAllowed)
}

func (srv *HttpServer)delete(w http.ResponseWriter, req *http.Request) {
	/*log.Printf("Delete request received for uuid [%s]", req.URL.Path)
	err := os.RemoveAll(srv.dir + "/" + req.URL.Path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)*/
}

func (srv *HttpServer)upload(w http.ResponseWriter, req *http.Request) {
	start := time.Now().UnixNano()
	defer func() {
		req.Body.Close()
		def.ReqUploadTime.Update((float64(time.Now().UnixNano() - start) / float64(1e9)))

		if r := recover(); r != nil {
			fmt.Println("panic upload error")
			def.ReqUploadErrCount.Inc(1)
			utils.WriteUploadResponse(w,r.(error))
		}
	}()

	start2 := time.Now().UnixNano()

	def.ReqUploadCount.Inc(1)
	//atomic.AddInt64(&reqUploadCount,1)
	//req.ParseMultipartForm(64)
	forms := make(map[string]string)
	var blob   *bytebufferpool.ByteBuffer

	reader,err := req.MultipartReader()
	if  err != nil {
		fmt.Println(err)
		panic(err)
	}

	//fmt.Println(err)
	for {
		part, err_part := reader.NextPart()
		if err_part == io.EOF {
			break
		}
		if err_part != nil {
			panic(err_part)
			fmt.Println("err_part",err_part)
			break
		}
		name := part.FormName()
		if name == "" {
			continue
		}

		buf := bytebufferpool.Get()
		defer func() {
			bytebufferpool.Put(buf)
		}()

		if name !=  def.ParamFile {
			buf.ReadFrom(part)
			//log.Println(name,"  is: ", buf.String())
			forms[name] = buf.String()

		} else {
			//forms[def.ParamFile] = part.FileName()
			//log.Println(def.ParamFile,part.FileName())
			buf.ReadFrom(io.MultiReader(part))
			blob = buf

		}
	}

	partIndex := forms[def.ParamPartIndex]
	end2 := time.Now().UnixNano()
	def.UploadICPTime.Update(float64(end2-start2) / float64(1e9))

	if len(partIndex) == 0 {
		srv.singleFile(w,req,&forms,blob)
		//srv.singleFile(w,req,&forms,blob)
		return
	}

	srv.multiFile(w,req,&forms,blob)
	return
}

func (srv *HttpServer)singleFile(w http.ResponseWriter, req *http.Request,forms *map[string]string,blob*bytebufferpool.ByteBuffer) {
	uuid := (*forms)[def.ParamUuid]
	if len(uuid) == 0 {
		log.Printf("No uuid received, invalid upload request")
		http.Error(w, "No uuid received", http.StatusBadRequest)
		def.ReqUploadParamErrCount.Inc(1)
		return
	}

	filename := fmt.Sprintf("%s",  (*forms)[def.ParamFileName])
	totalSize := len(blob.Bytes())
	totalPart := 1
	offset := 0
	index := 0

	srv.config.Storage.WriteGridFile(filename,(*forms)[def.ParamFileName],uuid,totalSize,totalSize,totalPart,offset,index,blob.Bytes())
	utils.WriteUploadResponse(w, nil)

	def.ReqUploadOkCount.Inc(1)
}


func (srv *HttpServer)multiFile(w http.ResponseWriter, req *http.Request,forms *map[string]string,blob*bytebufferpool.ByteBuffer) {
	uuid := (*forms)[def.ParamUuid]
	if len(uuid) == 0 {
		log.Printf("No uuid received, invalid upload request")
		http.Error(w, "No uuid received", http.StatusBadRequest)
		def.ReqUploadParamErrCount.Inc(1)
		return
	}

	chunkSize,err := strconv.Atoi((*forms)[def.ParamChunkSize])
	if err != nil {
		fmt.Println(err)
	}
	totalSize,err := strconv.Atoi((*forms)[def.ParamTotalFileSize])
	index,err := strconv.Atoi((*forms)[def.ParamPartIndex])
	//fmt.Println(len(buf.Bytes()))
	cookie, err := req.Cookie(uuid)
	cookieVae := ""
	if err == nil {
		cookieVae = cookie.Value
	}
	id := srv.outputer.WriteChunks(cookieVae,index,blob.Bytes(),uuid,chunkSize,totalSize,(*forms)[def.ParamFileName])

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


	utils.WriteUploadResponse(w, nil)
	def.ReqUploadOkCount.Inc(1)

}


func (this *HttpServer)ChunksDoneHandler(w http.ResponseWriter, req *http.Request) {
	uuid := req.FormValue(def.ParamUuid)
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
			def.ReqUploadDoneErrCount.Inc(1)
		}
	}()

	if req.Method != http.MethodPost {
		errorMsg := fmt.Sprintf("Method [%s] is not supported", req.Method)
		http.Error(w, errorMsg, http.StatusMethodNotAllowed)
	}
	def.ReqUploadDoneCount.Inc(1)

	filename := req.FormValue(def.ParamFileName)

	if uuid == ""  || filename == ""{
		def.ReqUploadDoneParamErrCount.Inc(1)
		return
	}
	cookie, err := req.Cookie(uuid)
	cookieVae := ""
	if err == nil {
		cookieVae = cookie.Value
	}
	this.config.Storage.UploadDoneHandler(uuid,cookieVae)

	def.ReqUploadDoneOkCount.Inc(1)
}