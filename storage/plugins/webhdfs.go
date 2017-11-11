package plugins

import (
	"log"
	"fmt"
	"net/http"
	"sync"
	"../../storage"
	"../../def"
	"../../config"
	"github.com/vladimirvivien/gowfs"
	"bytes"
	"strconv"
	"io"
	"strings"
	"../../utils"
	"path/filepath"
)

type WebHdfs struct {
	lock sync.Mutex
	config config.Config
	fs *gowfs.FileSystem
}

func init()  {
	storage.Add("webhdfs", func(config config.Config) def.Storager {
		out := WebHdfs{
			config:config,
		}
		out.Init(config.OutputWebHdfs.Url)
		return &out
	})

}

func (srv *WebHdfs)Init(url string) {
	fs, err := gowfs.NewFileSystem(gowfs.Configuration{Addr: url, User: "WXH"})
	if err != nil{
		log.Fatal(err)
	}
	srv.fs = fs
}

func (srv *WebHdfs)GetFinalFileID(cookie string, uuid string,chunkSize int,totalFileSize int,filename string) interface{}{
	return nil
}

func (this *WebHdfs)UploadDoneHandler(uuid string,file_id string,filename string,totalpart int) {

	var sources []string
/*

Concat has some restrictions, like the need for src file having last block size to be the same as the configured dfs.block.size.
If all the conditions are met, below command example should work (where we are concatenating /user/root/file-2 into /user/root/file-1):

curl -i -X POST "http:HTTPFS_HOST:14000/webhdfs/v1/user/root/file-1?user.name=root&op=CONCAT&sources=/user/root/file-2"
*/
	sources = make([]string,totalpart-1)
	for i := 1; i < totalpart;i++  {
		sources[i-1] = fmt.Sprintf("/%s/%s_%d.part",def.DATA_BASE,uuid,i)
	}

	b,err := this.fs.Concat(gowfs.Path{Name: fmt.Sprintf("/%s/%s_0.part",def.DATA_BASE,uuid)},sources)

	fmt.Println(b,err)
	b,err = this.fs.Rename(gowfs.Path{Name: fmt.Sprintf("/%s/%s_0.part",def.DATA_BASE,uuid)},
		gowfs.Path{Name: fmt.Sprintf("/%s/%s-%s",def.DATA_BASE,uuid,filename)})
	if err != nil {
		fmt.Println(b,err)
		panic(err)
	}
}

func (srv *WebHdfs) WriteGridFile(filename string,
	shortName string,
	uuid string,
	chunkSize int,
	totalSize int,
	totalPart int,
	offset int,
	index int,
	datas []byte)  {

	b,err := srv.fs.Create(bytes.NewBuffer(datas),
			gowfs.Path{Name: fmt.Sprintf("/%s/%s",def.DATA_BASE,filename)},
			false,
			0,
			1,
		0777,
			uint(len(datas)))
	if err != nil {
		fmt.Println(b,err)
		panic(err)
	}
}

func (srv*WebHdfs)PacketChunks(cookie string,index int,datas []byte,uuid string,chunkSize int,totalSize int,filename string) (out []byte,oid string) {

	return datas,""
}

func (srv*WebHdfs)WriteChunkPacket(index int,datas []byte,fileid string,uuid string){
	b,err := srv.fs.Create(bytes.NewBuffer(datas),
		gowfs.Path{Name: fmt.Sprintf("/%s/%s_%d.part",def.DATA_BASE,uuid,index)},
		false,
		0,
		1,
		0777,
		uint(len(datas)))
	if err != nil {
		fmt.Println(b,err)
		panic(err)
	}
}

func (srv *WebHdfs) DownloadHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		errorMsg := fmt.Sprintf("Method [%s] is not supported:", req.Method)
		http.Error(w, errorMsg, http.StatusMethodNotAllowed)
		return
	}



	fileId := req.FormValue("id")
	filename := filepath.Base(fileId)

	fi,err := srv.fs.GetFileStatus(gowfs.Path{Name: fileId})
	if err != nil || fi.Length <= 0 {
		//http.Error(w, "no file", http.StatusNotFound)
		fmt.Println(err,fi)
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

		maxReadSize := 2*1024*1024
		if len(posArr) == 1 {
			startPos, _ = strconv.Atoi(posArr[0])

		} else if len(posArr) == 2 {
			startPos, _ = strconv.Atoi(posArr[0])
			endPos, _ = strconv.Atoi(posArr[1])
		}
		//debugLog.Println(len(posArr), startPos, endPos)
		if endPos == 0 {
			endPos = startPos + maxReadSize
			if endPos > int(fi.Length) {
				endPos = int(fi.Length)
			}
		} else {
			endPos = endPos + 1
		}

		size := endPos - startPos
		if size <= 0 {
			utils.WriteDownloadHeader(w,filename,size)

			w.Header().Add("Content-Range", "bytes "+strconv.Itoa(startPos)+"-"+strconv.Itoa(endPos-1)+"/"+strconv.Itoa(int(fi.Length)))
			w.WriteHeader(http.StatusOK)
			return
		}

		//开始计算长度
		w.Header().Add("Content-Range", "bytes "+strconv.Itoa(startPos)+"-"+strconv.Itoa(endPos-1)+"/"+strconv.Itoa(int(fi.Length)))
		w.WriteHeader(http.StatusPartialContent)

		f,err := srv.fs.Open(gowfs.Path{Name: fileId},int64(startPos),int64(size),0)
		defer f.Close()
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		io.CopyN(w,f,int64(size))

		utils.WriteDownloadHeader(w,filename,size)
		return
	}

	utils.WriteDownloadHeader(w,filename,int(fi.Length))
	w.WriteHeader(http.StatusOK)
	if req.Method == http.MethodHead {
		return
	}
	f,err := srv.fs.Open(gowfs.Path{Name: fileId},0,int64(fi.Length),0)
	defer f.Close()
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	io.Copy(w,f)
	return
}

func (srv *WebHdfs) DeleteHandler(filename string) {
	b,err := srv.fs.Delete(gowfs.Path{Name: fmt.Sprintf("/%s/%s",def.DATA_BASE,filename)},true)
	if err != nil {
		fmt.Println(b,err)
		panic(err)
	}
}