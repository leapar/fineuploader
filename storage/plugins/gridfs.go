package plugins

import (
	"fineuploader/config"
	"fineuploader/def"
	"fineuploader/storage"
	"fineuploader/utils"
	"fmt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type OutputGridfs struct {
	session *mgo.Session
	lock sync.Mutex
	config config.Config
}

func init()  {
	storage.Add("mongodb", func(config config.Config) def.Storager {
		out := OutputGridfs{
			config:config,
		}
		out.Init(config.OutputMongo.MongoServer)
		return &out
	})

}

func (srv *OutputGridfs)Init(url string) {
	session, err := mgo.Dial(url)

	log.Println("dial mongodb:",url)
	if err != nil {
		log.Println("[grid]mgo.Dial ", err)
		return
	}
	srv.session = session

	index := mgo.Index{
		Key:    []string{"files_id", "n"},
		Unique: true,
	}

	database := session.DB(def.DATA_BASE)
	gridFs := database.GridFS(def.PREFIX)
	gridFs.Chunks.EnsureIndex(index)

}

func (srv *OutputGridfs)GetFinalFileID(cookie string, uuid string,chunkSize int,totalFileSize int,filename string) interface{}{
	if  bson.IsObjectIdHex(cookie) {
		//fmt.Println(cookie)
		return bson.ObjectIdHex(cookie)
	}

	srv.lock.Lock()
	defer srv.lock.Unlock()
	var objFile def.GfsFile
	//err := srv.gridFs.Files.Find(bson.M{ "filename": bson.RegEx{fmt.Sprintf("%s#%s", uuid,filename), "i"}}).One(&objFile)
	session := srv.session.Copy()
	defer session.Close()
	database := session.DB(def.DATA_BASE)
	gridFs := database.GridFS(def.PREFIX)

	err := gridFs.Files.Find(bson.M{ "md5": uuid}).One(&objFile)
	if err == nil {
		return objFile.Id
	}
	finalId := bson.NewObjectId()

	objFile = def.GfsFile {
		Id:finalId,
		UploadDate: bson.Now(),
		Length: int64(totalFileSize),
		ChunkSize: chunkSize,
		Filename:fmt.Sprintf("%s", filename),
		MD5:uuid,
	}
	err = gridFs.Files.Insert(objFile)
	if err != nil {
		def.DBErrCount.Inc(1)
		log.Println("Files.Insert",err)
		panic(err)
	}
	return finalId
}

func (this *OutputGridfs)UploadDoneHandler(uuid string,file_id string,filename string,totalpart int) {
	session := this.session.Copy()
	defer session.Close()
	database := session.DB(def.DATA_BASE)
	gridFs := database.GridFS(def.PREFIX)

	var objFile []def.GfsFile
	err := gridFs.Files.Find(bson.M{ "md5": uuid}).All(&objFile)
	if err != nil {
		fmt.Println("Files.Find",err)
		def.DBErrCount.Inc(1)
		panic(err)
	}

	if len(objFile) == 0 {
		if bson.IsObjectIdHex(file_id){
			fmt.Println("no file")
			gridFs.Chunks.RemoveAll(bson.M{"files_id":bson.ObjectIdHex(file_id)})
			panic(err)
		} else {
			fmt.Println("no file and no cookie")
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
				def.DBErrCount.Inc(1)
				panic(err)
			}
			err = gridFs.Files.Remove(bson.M{"_id":file.Id})
			if err != nil {
				fmt.Println("Files.Remove",err)
				def.DBErrCount.Inc(1)
				panic(err)
			}
		}
	}

	err = gridFs.Files.Update(bson.M{"_id":firstFile.Id}, bson.M{"$set": bson.M{ "uploadDate":  bson.Now(), }})
	if err != nil {
		fmt.Println("Files.Update",err)
		def.DBErrCount.Inc(1)
		panic(err)
	}
}

func (srv *OutputGridfs) WriteGridFile(filename string,
	shortName string,
	uuid string,
	chunkSize int,
	totalSize int,
	totalPart int,
	offset int,
	index int,
	datas []byte) error {

	session := srv.session.Copy()
	defer session.Close()
	database := session.DB(def.DATA_BASE)
	gridFs := database.GridFS(def.PREFIX)


	gridFile, err := gridFs.Create(filename)
	if err != nil {
		def.DBErrCount.Inc(1)
		log.Println(err)
		//panic(err)
		return err
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
		def.DBErrCount.Inc(1)
		//panic(err)
		return err
	}

	return nil
}

func (srv*OutputGridfs)WriteChunks(cookie string,index int,datas []byte,uuid string,chunkSize int,totalSize int,filename string) string {

	start := time.Now().UnixNano()
	defer func() {
		def.WriteChunkTime.Update((float64(time.Now().UnixNano() - start) / float64(1e9)))
	}()

	datas,id := srv.PacketChunks(cookie,index,datas,uuid,chunkSize,totalSize,filename)

	srv.WriteChunkPacket(index,datas,id,uuid)
	return id
}

func (srv*OutputGridfs)PacketChunks(cookie string,index int,datas []byte,uuid string,chunkSize int,totalSize int,filename string) (out []byte,oid string) {

	var id bson.ObjectId

	id = srv.GetFinalFileID(cookie,uuid,chunkSize,totalSize,filename).(bson.ObjectId)

	/*buf := bytebufferpool.Get()
	defer func() {
		if buf != nil {
			bytebufferpool.Put(buf)
		}
	}()
	*/
	var err error
	//fmt.Printf("%d %v %s\n",len(datas),unsafe.Pointer(&datas),"  in")
	datas, err = bson.Marshal(def.GfsChunk{bson.NewObjectId(), id, index, datas})
	//fmt.Printf("%d %v %s\n",len(datas),unsafe.Pointer(&datas),"  out")

	if err != nil {
		log.Println(err)
		panic(err)
	}

	return datas,id.Hex()
}

func (srv*OutputGridfs)WriteChunkPacket(index int,datas []byte,fileid string,uuid string){
	session := srv.session.Copy()
	defer session.Close()
	database := session.DB(def.DATA_BASE)
	gridFs := database.GridFS(def.PREFIX)

	err := gridFs.Chunks.Insert(bson.Raw{Data: datas})

	if err != nil{
		//fmt.Println("E11000")
		if strings.LastIndex(err.Error(),"E11000") != -1 {
			gridFs.Chunks.Remove(bson.M{"files_id":bson.ObjectIdHex(fileid),"n":index})
			err = gridFs.Chunks.Insert(bson.Raw{Data: datas})
			if err != nil{
				log.Println("retry",err)
				def.DBErrCount.Inc(1)
				panic(err)
			}
		}else {
			log.Println(err)
			def.DBErrCount.Inc(1)
			panic(err)
		}
	}
}




func (srv *OutputGridfs) DownloadHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		errorMsg := fmt.Sprintf("Method [%s] is not supported:", req.Method)
		http.Error(w, errorMsg, http.StatusMethodNotAllowed)

		return
	}

	fileId := req.FormValue("id")
	var objFile def.GfsFile

	session := srv.session.Copy()
	defer session.Close()
	database := session.DB(def.DATA_BASE)
	gridFs := database.GridFS(def.PREFIX)

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
			utils.WriteDownloadHeader(w,objFile.Filename,size)

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

			var chunk def.GfsChunk



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
		utils.WriteDownloadHeader(w,objFile.Filename,size)




		return
	}
	var chunk def.GfsChunk
	utils.WriteDownloadHeader(w,objFile.Filename,int(objFile.Length))

	w.WriteHeader(http.StatusOK)

	if req.Method == http.MethodHead {
		return
	}
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