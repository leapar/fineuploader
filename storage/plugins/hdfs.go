package plugins

import (
	"fineuploader/config"
	"fineuploader/def"
	"fineuploader/hdfspool"
	"fineuploader/storage"
	"fineuploader/utils"
	"fmt"
	"github.com/colinmarc/hdfs"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Hdfs struct {
	lock sync.Mutex
	config config.Config
	hdfspool pool.Pool
	//client *hdfs.Client
}

func init()  {
	storage.Add("hdfs", func(config config.Config) def.Storager {
		out := Hdfs{
			config:config,
		}
		out.Init(config.OutputHdfs.Url,config.OutputHdfs.User)
		return &out
	})

}

func  (srv *Hdfs)test() {

	for i:=0;i<600;i++ {
		client,err := srv.hdfspool.Get()

		file, err := client.Create(fmt.Sprintf("/%d.part",i))



		//f,err := os.Open("F:\\模型\\M076900O201030219037\\obj\\M076900O201030219037-00-00\\0_SFusion.mtl")
		f,err := os.Open("C:\\Users\\WXH\\Downloads\\87847b63d1da669dbc8b52670d03b0c5_53.part")

		n,err := io.Copy(file,f)
		log.Println("end",n,err )
		file.Close()
		f.Close()
		client.Close()
	}

}

func (srv *Hdfs)Init(url string,user string) {
	//client,err := hdfs.New("0.0.0.0:19000")


	factory := func() (*hdfs.Client, error) {
		return  hdfs.NewClient(hdfs.ClientOptions{Addresses:[]string{url},User:user})
	}

	nsqPool, err := pool.NewClientPool(10, 64, factory)
	if nil != err {
		fmt.Println(err)
		return
	}
	srv.hdfspool = nsqPool
	client,err := srv.hdfspool.Get()
	defer client.Close()

	client.Mkdir( fmt.Sprintf("/%s/",def.DATA_BASE,),0077)

	//srv.test()
	//f,err := client.Stat("/fileserver/模型2.zip")
	//fmt.Println(f,err)

	/*log.Println("start")
	file, err := client.Create("/2333354.zip")
	defer file.Close()
	fmt.Println(err)

	f,err := os.Open("F:\\模型\\M076900O201030219037\\obj\\M076900O201030219037-00-00\\0_SFusion.mtl")
	defer f.Close()
	n,err := io.Copy(file,f)
	log.Println("end",n,err )*/
/*
file, _ := client.Open("/9999.zip")
	buf := make([]byte, 59)
	file.ReadAt(buf, 48847)

	fmt.Println(string(buf))*/
}

func (srv *Hdfs)GetFinalFileID(cookie string, uuid string,chunkSize int,totalFileSize int,filename string) interface{}{
	return nil
}

/*
// Rename renames (moves) a file.
func (c *Client) Concat(oldpath string ,sources []string) error {

	req := &hdfs.ConcatRequestProto{
		Trg:           proto.String(oldpath),
		Srcs:          sources,
	}
	resp := &hdfs.ConcatResponseProto{}

	err := c.namenode.Execute("concat", req, resp)
	if err != nil {
		if nnErr, ok := err.(*rpc.NamenodeError); ok {
			err = interpretException(nnErr.Exception, err)
		}

		return &os.PathError{"concat", oldpath, err}
	}

	return nil
}

*/

func (this *Hdfs)UploadDoneHandler(uuid string,file_id string,filename string,totalpart int) {
	var sources []string
	sources = make([]string,totalpart-1)
	for i := 1; i < totalpart;i++  {
		sources[i-1] = fmt.Sprintf("/%s/%s_%d.part",def.DATA_BASE,uuid,i)
	}
	client,err := this.hdfspool.Get()
	defer client.Close()

	err = client.Concat(fmt.Sprintf("/%s/%s_0.part",def.DATA_BASE,uuid),sources)
	if err != nil {
		log.Println(err)
		panic(err)
	}
	finalName := fmt.Sprintf("/%s/%s.%s",def.DATA_BASE,time.Now().Format("2006-01-02_03_04_05_PM"),filename)
	err = client.Rename(fmt.Sprintf("/%s/%s_0.part",def.DATA_BASE,uuid),finalName)
	if err != nil {
		log.Println(err)
		panic(err)
	}


	f,err := client.Open(finalName)
	defer f.Close()
	if err == nil {
		bytes,err := f.Checksum()
		fmt.Println(fmt.Sprintf("%x",bytes),err)
	} else {
		fmt.Println(err)
	}
/*

Concat has some restrictions, like the need for src file having last block size to be the same as the configured dfs.block.size.
If all the conditions are met, below command example should work (where we are concatenating /user/root/file-2 into /user/root/file-1):

curl -i -X POST "http:HTTPFS_HOST:14000/webhdfs/v1/user/root/file-1?user.name=root&op=CONCAT&sources=/user/root/file-2"
*/
	/*

	b,err := this.fs.Concat(gowfs.Path{Name: fmt.Sprintf("/%s/%s_0.part",def.DATA_BASE,uuid)},sources)

	fmt.Println(b,err)
	b,err = this.fs.Rename(gowfs.Path{Name: fmt.Sprintf("/%s/%s_0.part",def.DATA_BASE,uuid)},
		gowfs.Path{Name: fmt.Sprintf("/%s/%s-%s",def.DATA_BASE,uuid,filename)})
	if err != nil {
		fmt.Println(b,err)
		panic(err)
	}*/
}

func (srv *Hdfs) WriteGridFile(filename string,
	shortName string,
	uuid string,
	chunkSize int,
	totalSize int,
	totalPart int,
	offset int,
	index int,
	datas []byte) error {

	filepath := fmt.Sprintf("/%s/%s",def.DATA_BASE,filename)
	client,err := srv.hdfspool.Get()
	defer client.Close()
	client.Mkdir( fmt.Sprintf("/%s/",def.DATA_BASE,),0077)
	f,err := client.Stat(filepath)
	if err == nil && f != nil{
		client.Remove(filepath)
	}

	w,err := client.Create(filepath)
	if err != nil {
		fmt.Println(err)
		//panic(err)
		return err
	}
	defer w.Close()

	_,err = w.Write(datas)
	if err != nil {
		fmt.Println(err)
		//panic(err)
		return err
	}

	return nil
}

func (srv*Hdfs)PacketChunks(cookie string,index int,datas []byte,uuid string,chunkSize int,totalSize int,filename string) (out []byte,oid string) {
	return datas,""
}
/*
func (srv*Hdfs)WriteChunkPacket3(index int,datas []byte,fileid string,uuid string){
	filepath := fmt.Sprintf("/%s/%s.part",def.DATA_BASE,uuid)
	client,err := srv.hdfspool.Get()
	defer client.Close()

	var blockSize int64 = 1024*1024*128
	client.Mkdir( fmt.Sprintf("/%s/",def.DATA_BASE,),0077)
	fi,err := client.Stat(filepath)
	var f *hdfs.FileWriter
	if err != nil {
		fmt.Println(fi)
		f,err = client.CreateFile(filepath,1,blockSize , 0644)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		f.Write(make([]byte,1024*1024*20))
		f.Close()
	}


	defer func() {
		//err := f.Close()
		if err != nil {
			log.Println("close:",err)
		}
	}()

	var offset int64 = 2*1024
	n,err := client.Write2(filepath,offset)

	n.Write(datas)
	defer n.Close()
	if err != nil {
		fmt.Println(n,err)
		panic(err)
	}
}*/

func (srv*Hdfs)WriteChunkPacket(index int,datas []byte,fileid string,uuid string){
	filepath := fmt.Sprintf("/%s/%s_%d.part",def.DATA_BASE,uuid,index)
	client,err := srv.hdfspool.Get()
	defer client.Close()

	client.Mkdir( fmt.Sprintf("/%s/",def.DATA_BASE,),0077)
	fi,err := client.Stat(filepath)
	if err == nil && fi != nil{
		err = client.Remove(filepath)
		if err != nil {
			fmt.Println(err)
		}
	}

	f,err := client.Create(filepath)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Println("close:",err)
		}
	}()
	n,err := f.Write(datas)

	if err != nil {
		fmt.Println(n,err)
		panic(err)
	}
}

func (srv *Hdfs) DownloadHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		errorMsg := fmt.Sprintf("Method [%s] is not supported:", req.Method)
		http.Error(w, errorMsg, http.StatusMethodNotAllowed)

		return
	}

	fileId := req.FormValue("id")
	client,err := srv.hdfspool.Get()
	defer client.Close()

	fi,err := client.Stat(fileId)
	if err != nil || fi == nil {
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
			if endPos > int(fi.Size()) {
				endPos = int(fi.Size())
			}
		} else {
			endPos = endPos + 1
		}

		size := endPos - startPos
		if size <= 0 {
			utils.WriteDownloadHeader(w,fi.Name(),size)

			w.Header().Add("Content-Range", "bytes "+strconv.Itoa(startPos)+"-"+strconv.Itoa(endPos-1)+"/"+strconv.Itoa(int(fi.Size())))
			w.WriteHeader(http.StatusOK)
			return
		}

		//开始计算长度
		w.Header().Add("Content-Range", "bytes "+strconv.Itoa(startPos)+"-"+strconv.Itoa(endPos-1)+"/"+strconv.Itoa(int(fi.Size())))
		w.WriteHeader(http.StatusPartialContent)

		f,err := client.Open(fileId)
		defer f.Close()
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		f.Seek(int64(startPos),io.SeekStart)
		io.CopyN(w,f,int64(size))

		utils.WriteDownloadHeader(w,fi.Name(),size)
		return
	}

	utils.WriteDownloadHeader(w,fi.Name(),int(fi.Size()))
	w.WriteHeader(http.StatusOK)
	if req.Method == http.MethodHead {
		return
	}
	f,err := client.Open(fileId)
	defer f.Close()
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	io.Copy(w,f)
	return
}

func (srv *Hdfs) DeleteHandler(filename string) {
	client,err := srv.hdfspool.Get()
	defer client.Close()

	err = client.Remove(fmt.Sprintf("/%s/%s",def.DATA_BASE,filename))
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}