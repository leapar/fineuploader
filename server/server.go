package server

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fineuploader/config"
	"fineuploader/def"
	"fineuploader/input"
	_ "fineuploader/input/plugins"
	"fineuploader/output"
	_ "fineuploader/output/plugins"
	"fineuploader/restapi"
	"fineuploader/restapi/restmodel"
	"fineuploader/server/middlewares"
	"fineuploader/server/model"
	"fineuploader/storage"
	_ "fineuploader/storage/plugins"
	"fmt"
	"github.com/NYTimes/gziphandler"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/oxtoacart/bpool"
	"github.com/rcrowley/go-metrics"
	"gopkg.in/mgo.v2/bson"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"time"
)

var bufpool *bpool.BufferPool

type HttpServer struct {
	config   config.Config
	outputer def.Outputer
}

func New(config config.Config) *HttpServer {
	bufpool = bpool.NewBufferPool(22)
	return &HttpServer{
		config: config,
	}
}

func (this *HttpServer) newApp() *gin.Engine {
	gob.Register(&model.Login{})
	router := gin.Default()

	restapi := restapi.New(this.config)

	this.config.Storage = storage.Storages[this.config.StorageName](this.config)
	this.outputer = output.Outputs[this.config.Output](this.config)
	if this.config.InputNsq.Enable {
		input.Iutputs["nsq"](this.config)
	}

	//store := sessions.NewCookieStore([]byte("secret"))
	router.Use( middlewares.SessionStore)

	// Example for binding JSON ({"user": "manu", "password": "123"})
	router.POST("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		fmt.Println("login:", session.Get("user"))
		var user model.Login
		session.Delete("user")
		session.Save()
		if err := c.ShouldBindJSON(&user); err == nil {
			auth, err := restapi.Login(user.Username, user.Password)
			if err != nil {
				c.JSON(http.StatusOK, model.NormalRes{
					Code:    model.ERROR,
					Message: err.Error(),
				})
				return
			}
			user.Auth = auth
			if user.Auth.AccessToken != "" {
				userInfo, err := restapi.GetUserInfo(user.Auth)
				if err != nil {
					c.JSON(http.StatusOK, model.NormalRes{
						Code:    model.ERROR,
						Message: err.Error(),
					})
					return
				}
				user.ID = userInfo.ID
				user.User = userInfo
				session.Set("user", &user)
				//fmt.Println("login:", user, resp, err)
				session.Save()

				c.Set("user", &user)

				c.JSON(http.StatusOK, model.NormalRes{
					Code: model.OK,
					Data: user,
				})

			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"status": "unauthorized"})
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}

	})

	//router.Use(middlewares.RequireAuthentication)
	//提交上传文件信息，获取文件ID存入COOKIE
	router.POST("/preupload", func(context *gin.Context) {
		batchId := context.PostForm("batchId")
		taskId := context.PostForm("taskId")
		modelId := context.PostForm("modelId")

		if batchId != "" {
			isColorBoard, _ := strconv.ParseBool(context.PostForm("isColorBoard"))
			isHardware, _ := strconv.ParseBool(context.PostForm("isHardware"))

			if isColorBoard == false && isHardware == false {
				context.AbortWithStatusJSON(http.StatusInternalServerError, model.UploadResponse{
					Error: "参数错误",
				})
				return
			}

		} else if taskId != "" {

		} else if modelId != "" {

		} else {
			context.AbortWithStatusJSON(http.StatusInternalServerError, model.UploadResponse{
				Error: "参数错误",
			})
			return
		}

		err := this.PreUploadHandler(context)
		if err != nil {
			context.JSON(http.StatusInternalServerError, model.UploadResponse{
				Error: err.Error(),
			})
		} else {
			val, _ := context.Get("user")
			user := val.(*model.Login)
			extFile := restmodel.ExtFile{}
			extFile.CheckSum = context.PostForm(def.ParamUuid) //"CheckSum"
			extFile.Type = restmodel.EXT_FILE_TYPE_MONGILEFS
			extFile.Status = restmodel.EXT_FILE_STATUS_READY
			totalSize, _ := strconv.Atoi(context.PostForm(def.ParamTotalFileSize))
			extFile.Size = uint64(totalSize)                       //1 << 10
			extFile.FileName = context.PostForm(def.ParamFileName) //"FileName"

			if batchId != "" {
				isColorBoard, _ := strconv.ParseBool(context.PostForm("isColorBoard"))
				isHardware, _ := strconv.ParseBool(context.PostForm("isHardware"))

				if isColorBoard {
					batch := restmodel.BatchColorboard{ColorboardPath: &extFile, Id: batchId,}
					err := restapi.SaveBatchInfo(user.Auth, &batch)
					if err != nil {
						context.AbortWithStatusJSON(http.StatusInternalServerError, model.UploadResponse{
							Error: err.Error(),
						})
						return
					}
				} else if isHardware {
					batch := restmodel.BatchColorboard{HardwarePath: &extFile, Id: batchId,}
					err := restapi.SaveBatchInfo(user.Auth, &batch)

					if err != nil {
						context.AbortWithStatusJSON(http.StatusInternalServerError, model.UploadResponse{
							Error: err.Error(),
						})

						return
					}
				}
			} else if taskId != "" {
				task := restmodel.ProjectModelTaskPlan{ExtFile: &extFile, Id: taskId,}
				err := restapi.SaveTaskPackageInfo(user.Auth, &task)
				if err != nil {
					context.AbortWithStatusJSON(http.StatusInternalServerError, model.UploadResponse{
						Error: err.Error(),
					})

					return
				}
			} else if modelId != "" {
				m := restmodel.ProjectModel{OriginFile: &extFile, Id: modelId,}
				err := restapi.SaveModelPackageInfo(user.Auth, &m)
				if err != nil {
					context.AbortWithStatusJSON(http.StatusInternalServerError, model.UploadResponse{
						Error: err.Error(),
					})

					return
				}
			}

			context.JSON(http.StatusOK, model.UploadResponse{
				Success: true,
			})
		}
	})
	//上传文件块
	router.POST("/upload", func(context *gin.Context) {
		err := this.UploadHandler(context)
		if err != nil {
			context.JSON(http.StatusInternalServerError, model.UploadResponse{
				Error: err.Error(),
			})
		} else {
			context.JSON(http.StatusOK, model.UploadResponse{
				Success: true,
			})
		}
	})

	router.GET("/metrics", func(context *gin.Context) {
		this.Metrics(context)
	})
	//上传完成
	router.POST("/chunksdone", func(context *gin.Context) {
		batchId := context.PostForm("batchId")
		taskId := context.PostForm("taskId")
		modelId := context.PostForm("modelId")

		if batchId != "" {
			isColorBoard, _ := strconv.ParseBool(context.PostForm("isColorBoard"))
			isHardware, _ := strconv.ParseBool(context.PostForm("isHardware"))

			if isColorBoard == false && isHardware == false {
				context.AbortWithStatusJSON(http.StatusInternalServerError, model.UploadResponse{
					Error: "参数错误",
				})
			}

		} else if taskId != "" {

		} else if modelId != "" {

		} else {
			context.AbortWithStatusJSON(http.StatusInternalServerError, model.UploadResponse{
				Error: "参数错误",
			})
		}

		err := this.ChunksDoneHandler(context)

		if err != nil {
			context.JSON(http.StatusInternalServerError, model.UploadResponse{
				Error: err.Error(),
			})
		} else {
			val, _ := context.Get("user")
			user := val.(*model.Login)

			extFile, err := restapi.GetExtFileInfo(user.Auth, context.PostForm(def.ParamUuid))
			//batch := restmodel.BatchColorboard{HardwarePath:&extFile,Id:batchId,}
			//err := restapi.SaveBatchInfo(user.Auth,&batch)

			if err != nil {
				context.AbortWithStatusJSON(http.StatusInternalServerError, model.UploadResponse{
					Error: err.Error(),
				})
				return
			}

			err = restapi.UpdateExtFileStatus(user.Auth, &restmodel.ExtFile{ID: extFile.ID, Status: restmodel.EXT_FILE_STATUS_DONE})
			if err != nil {
				fmt.Println(err)
				context.AbortWithStatusJSON(http.StatusInternalServerError, model.UploadResponse{
					Error: err.Error(),
				})
				return
			}

			context.JSON(http.StatusOK, model.UploadResponse{
				Success: true,
			})
		}
	})

	router.StaticFS("/res", AssetsFS(AssetsOpts{
		Develop: false,
	}));
	router.Static("/uploads", "./ui")

	/*router.GET("/res/", func(context *gin.Context) {
		http.StripPrefix("/res/", Assets(AssetsOpts{
			Develop: true,
		})).ServeHTTP(context.Writer, context.Request)
	})*/

	router.HEAD("/files", func(context *gin.Context) {
		this.DownloadHandler(context.Writer, context.Request)
	})

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.GET("/files", func(context *gin.Context) {
		this.DownloadHandler(context.Writer, context.Request)
		context.Abort()
	})

	/*app.Get("/", func(ctx iris.Context) {
		http.ServeFile(ctx.ResponseWriter(), ctx.Request(), ctx.Request().URL.Path[1:])
	})*/
	/*
		app.StaticWeb("/swagger", "./ui/swagger-ui/")
		app.StaticWeb("/ui", "./ui/static/")
		app.StaticWeb("/fine-uploader", "./ui/static/fine-uploader")
	*/
	//mySubdomainFsServer := app.Party("uploads/")
	//h := app.StaticHandler("./ui", true, false)
	// /* http://mysubdomain.mydomain.com/static/css/style.css */
	//app.Get("/ui", h)
	//app.SPA(h)

	//uploads := app.Party("/static")
	//app.Get("/", app.StaticHandler("./ui", true,true))

	//	app.StaticWeb("/uploads", "./ui/")
	//	app.Get("/uploads/*", app.StaticHandler("./ui", true,true))

	/*
		http.Handle("/upload/", http.StripPrefix("/upload/", http.HandlerFunc(this.UploadHandler)))
		http.HandleFunc("/res/", func(w http.ResponseWriter, r *http.Request) {
			http.StripPrefix("/res/", Assets(AssetsOpts{
				Develop:false,
			})).ServeHTTP(w, r)
		})
	*/

	//http.HandleFunc("/files",srv.DownloadHandler)

	hostPort := fmt.Sprintf("%s:%d", this.config.Host, this.config.Port)
	log.Printf("Initiating server listening at [%s]", hostPort)
	//log.Printf("Base upload directory set to [%s]", srv.dir)

	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 5*time.Second)

	return router
}

func (srv *HttpServer) Start() {
	hostPort := fmt.Sprintf("%s:%d", srv.config.Host, srv.config.Port)
	app := srv.newApp()
	app.Run(hostPort)
	return

	if srv.config.InputNsq.Enable {
		input.Iutputs["nsq"](srv.config)
	}
	/*srv.Init("127.0.0.1:4150")
	*/
	//	http.Handle("/upload/", http.StripPrefix("/upload/", http.HandlerFunc(srv.UploadHandler)))
	http.HandleFunc("/res/", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/res/", Assets(AssetsOpts{
			Develop: false,
		})).ServeHTTP(w, r)
	})
	http.HandleFunc("/uploads/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})
	http.Handle("/files", gziphandler.GzipHandler(http.HandlerFunc(srv.DownloadHandler)))
}

func (srv *HttpServer) DownloadHandler(w http.ResponseWriter, req *http.Request) {
	srv.config.Storage.DownloadHandler(w, req)
}

func (srv *HttpServer) Metrics(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, metrics.DefaultRegistry.(*metrics.StandardRegistry))
}

func (srv *HttpServer) PreUploadHandler(ctx *gin.Context) error {
	return srv.getID(ctx)
}

func (srv *HttpServer) getID(ctx *gin.Context) error {

	uuid := ctx.PostForm(def.ParamUuid)
	if len(uuid) == 0 {
		log.Printf("No uuid received, invalid upload request")
		return fmt.Errorf("No uuid received")
	}

	chunkSize, err := strconv.Atoi(ctx.PostForm(def.ParamChunkSize))
	if err != nil {
		fmt.Println(err)
		return err
	}
	totalSize, err := strconv.Atoi(ctx.PostForm(def.ParamTotalFileSize))

	//fmt.Println(len(buf.Bytes()))
	//cookie, err := req.Cookie(uuid)
	session := sessions.Default(ctx)
	fileUUID, _ := session.Get(uuid).(string)
	id := srv.config.Storage.GetFinalFileID(fileUUID, uuid, chunkSize, totalSize, ctx.PostForm(def.ParamFileName))
	if id == nil {
		return fmt.Errorf("file id is null")
	}

	file_id, _ := id.(bson.ObjectId)
	session.Set(uuid, file_id.Hex())
	session.Save()

	return nil
}

func (srv *HttpServer) delete(w http.ResponseWriter, req *http.Request) {
	/*log.Printf("Delete request received for uuid [%s]", req.URL.Path)
	err := os.RemoveAll(srv.dir + "/" + req.URL.Path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)*/
}

func (srv *HttpServer) UploadHandler(ctx *gin.Context) (err error) {
	//w http.ResponseWriter,
	req := ctx.Request
	start := time.Now().UnixNano()
	defer func() {
		req.Body.Close()
		def.ReqUploadTime.Update((float64(time.Now().UnixNano()-start) / float64(1e9)))

		if r := recover(); r != nil {
			fmt.Println("panic upload error")
			def.ReqUploadErrCount.Inc(1)

			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}

			//utils.WriteUploadResponse(w, r.(error))
		}
	}()

	start2 := time.Now().UnixNano()

	def.ReqUploadCount.Inc(1)
	//atomic.AddInt64(&reqUploadCount,1)
	//req.ParseMultipartForm(64)
	forms := make(map[string]string)
	//	var blob   *bytebufferpool.ByteBuffer
	var blob *bytes.Buffer

	reader, err := req.MultipartReader()
	if err != nil {
		fmt.Println(err)
		//panic(err)
		return err
	}

	//fmt.Println(err)
	for {
		part, err_part := reader.NextPart()
		if err_part == io.EOF {
			break
		}
		if err_part != nil {
			panic(err_part)
			fmt.Println("err_part", err_part)
			break
		}
		name := part.FormName()
		if name == "" {
			continue
		}

		buf := new(bytes.Buffer)

		if name != def.ParamFile {
			buf.ReadFrom(part)
			//log.Println(name,"  is: ", buf.String())
			forms[name] = buf.String()

		} else {
			//	buf := bytebufferpool.Get()
			//bpool 预分配，免得gc太慢
			buf := bufpool.Get()
			defer func() {
				bufpool.Put(buf)
			}()
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
		return srv.singleFile(&forms, blob.Bytes())
		//srv.singleFile(w,req,&forms,blob)
	}

	return srv.multiFile(ctx, &forms, blob.Bytes())
}

func (srv *HttpServer) singleFile(forms *map[string]string, blob []byte) error {
	uuid := (*forms)[def.ParamUuid]
	if len(uuid) == 0 {
		log.Printf("No uuid received, invalid upload request")
		//http.Error(w, "No uuid received", http.StatusBadRequest)
		def.ReqUploadParamErrCount.Inc(1)
		return fmt.Errorf("No uuid received")
	}

	filename := fmt.Sprintf("%s", (*forms)[def.ParamFileName])
	totalSize := len(blob)
	totalPart := 1
	offset := 0
	index := 0

	err := srv.config.Storage.WriteGridFile(filename, (*forms)[def.ParamFileName], uuid, totalSize, totalSize, totalPart, offset, index, blob)

	if err != nil {
		return err
	}
	def.ReqUploadOkCount.Inc(1)
	return nil
}

func (srv *HttpServer) multiFile(ctx *gin.Context, forms *map[string]string, blob [] byte) error {
	uuid := (*forms)[def.ParamUuid]
	if len(uuid) == 0 {
		log.Printf("No uuid received, invalid upload request")
		//http.Error(w, "No uuid received", http.StatusBadRequest)
		def.ReqUploadParamErrCount.Inc(1)
		return fmt.Errorf("No uuid received")
	}

	chunkSize, err := strconv.Atoi((*forms)[def.ParamChunkSize])
	if err != nil {
		fmt.Println(err)
	}
	totalSize, err := strconv.Atoi((*forms)[def.ParamTotalFileSize])
	index, err := strconv.Atoi((*forms)[def.ParamPartIndex])
	//fmt.Println(len(buf.Bytes()))

	session := sessions.Default(ctx)

	cookieVal, _ := session.Get(uuid).(string)

	id := srv.outputer.WriteChunks(cookieVal, index, blob, uuid, chunkSize, totalSize, (*forms)[def.ParamFileName])

	if cookieVal != id {
		session.Set(uuid, cookieVal)
		session.Save()
	}

	def.ReqUploadOkCount.Inc(1)

	return nil

}

func (this *HttpServer) ChunksDoneHandler(ctx *gin.Context) (err error) {
	//w http.ResponseWriter, req *http.Request
	uuid := ctx.PostForm(def.ParamUuid)
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("panic upload done error")
			def.ReqUploadDoneErrCount.Inc(1)
			//http.Error(w, "", http.StatusInternalServerError)
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}
		}
	}()

	def.ReqUploadDoneCount.Inc(1)
	filename := ctx.PostForm(def.ParamFileName)
	if uuid == "" || filename == "" {
		def.ReqUploadDoneParamErrCount.Inc(1)
		return fmt.Errorf("参数错误")
	}
	session := sessions.Default(ctx)
	cookieVal, _ := session.Get(uuid).(string)
	index, err := strconv.Atoi(ctx.PostForm(def.ParamTotalParts))
	this.config.Storage.UploadDoneHandler(uuid, cookieVal, filename, index)
	session.Delete(uuid)
	session.Save()
	def.ReqUploadDoneOkCount.Inc(1)
	return nil
}
