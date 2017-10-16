package main

import (
	"fmt"
	"net/http"
	"github.com/facebookgo/httpcontrol"
	"time"
	"bytes"
	"mime/multipart"
	"os"
	"io"
	"io/ioutil"
	"github.com/satori/go.uuid"
	"strconv"
	"crypto/md5"
	"math"
	"sync"
	"flag"
	"log"
	"encoding/json"
	"github.com/boltdb/bolt"

)

type tsdbrelayHTTPTransport struct {
	http.RoundTripper
}

type uploadResult struct {
	index int
	isok bool
}

type uploadRet struct {
	Success bool `json:"success"`
}

type boltUploadStruct struct {
	CheckSum string
	Uuid string
	FilePath string
	StartTime time.Time
	OverTime time.Time
	IsOver bool
	PartInfo []uploadResult
	IndexMap map[int]int
}

var boltDB *bolt.DB

var chunckSize = flag.Uint64("c", 2*1024*1024, "chunk size,defaults to 2*1024*1024")
var filePath = flag.String("f", "", "Upload file Path")
var conNum = flag.Int("n", 5, "Batch Connections,defaults to 5")
var host = flag.String("h", "172.29.231.80:8082", "Upload host Path")


var pool chan uploadResult
var locker sync.Mutex
func init() {

}

func upload(file string,fuid string,filename string,qqtotalfilesize int64,qqpartindex int, qqpartbyteoffset int64,qqchunksize int64,qqtotalparts int ) {
	var bResult bool = false

	defer func() {
		pool <- uploadResult {
			qqpartindex,
			bResult,
		}
		//fmt.Println("upload over:",qqpartindex)
	}()

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	// Add your image file
	f, err := os.Open(file)
	if err != nil {
		return
	}

	defer f.Close()
	fw, err := w.CreateFormFile("qqfile", filename)
	if err != nil {
		return
	}

	_,err = f.Seek(qqpartbyteoffset,io.SeekStart)

	if _, err = io.CopyN(fw,f,qqchunksize); err != nil {
		return
	}
	//qqpartbyteoffset
	//qqchunksize
	//qqtotalparts
	//qqtotalfilesize
	if fw, err = w.CreateFormField("qqtotalparts"); err != nil {
		return
	}
	if _, err = fw.Write([]byte(strconv.Itoa(qqtotalparts))); err != nil {
		return
	}

	if fw, err = w.CreateFormField("qqtotalfilesize"); err != nil {
		return
	}

	if _, err = fw.Write([]byte(strconv.FormatInt(qqtotalfilesize, 10))); err != nil {
		return
	}

	if fw, err = w.CreateFormField("qqpartbyteoffset"); err != nil {
		return
	}
	if _, err = fw.Write([]byte(strconv.FormatInt(qqpartbyteoffset,10))); err != nil {
		return
	}

	if fw, err = w.CreateFormField("qqchunksize"); err != nil {
		return
	}
	if _, err = fw.Write([]byte(strconv.FormatInt(qqchunksize,10))); err != nil {
		return
	}


	// Add the other fields
	if fw, err = w.CreateFormField("qqpartindex"); err != nil {
		return
	}
	if _, err = fw.Write([]byte(strconv.Itoa(qqpartindex))); err != nil {
		return
	}

	if fw, err = w.CreateFormField("qqfilename"); err != nil {
		return
	}
	if _, err = fw.Write([]byte(filename)); err != nil {
		return
	}


	if fw, err = w.CreateFormField("qquuid"); err != nil {
		return
	}
	if _, err = fw.Write([]byte(fuid)); err != nil {
		return
	}


	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	//body := bytes.NewBuffer(reader.buf.Bytes())


	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/upload",*host), &b)//HTTP://127.0.0.1:8081/upload
	if err != nil {
		///verbose("bosun connect error: %v", err)
		return
	}
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		//verbose("bosun relay error: %v", err)
		return
	}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("%s -- %v\n", string(buf), err)
	}


	resp.Body.Close()
	if err != nil {
		return
	}

	ret := uploadRet{}

	err = json.Unmarshal(buf,&ret)
	if err != nil {
		return;
	}
	if !ret.Success {

	}
	//verbose("bosun relay success")
	bResult = true

}

/*
qquuid:7174ed7c-8276-48a0-be2f-d87e70935282
qqfilename:win32-x64-51_binding.node
qqtotalfilesize:2172928
qqtotalparts:2
*/
func uploadDone(fuid string,filename string,qqtotalfilesize int64,qqtotalparts int ) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	// Add your image file

	fw, err := w.CreateFormField("qqtotalparts")
	if err != nil {
		return
	}
	if _, err = fw.Write([]byte(strconv.Itoa(qqtotalparts))); err != nil {
		return
	}

	if fw, err = w.CreateFormField("qqtotalfilesize"); err != nil {
		return
	}

	if _, err = fw.Write([]byte(strconv.FormatInt(qqtotalfilesize, 10))); err != nil {
		return
	}

	if fw, err = w.CreateFormField("qqfilename"); err != nil {
		return
	}
	if _, err = fw.Write([]byte(filename)); err != nil {
		return
	}


	if fw, err = w.CreateFormField("qquuid"); err != nil {
		return
	}
	if _, err = fw.Write([]byte(fuid)); err != nil {
		return
	}


	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	//body := bytes.NewBuffer(reader.buf.Bytes())
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/chunksdone",*host), &b)//"HTTP://127.0.0.1:8081/chunksdone
	if err != nil {
		///verbose("bosun connect error: %v", err)
		return
	}
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		//verbose("bosun relay error: %v", err)
		return
	}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("%s -- %v\n", string(buf), err)
	}

	resp.Body.Close()
	//verbose("bosun relay success")

}




func uploadAll(file string) {
	/*
	1. 判断文件大小
	2. 如果文件大于chuncksize 进行分片
	3. 启动go rountime
	4. 获取已经上传完成的分片id

	*/
	boltInfo := boltUploadStruct{}
	//defer boltDB.Close()
	defer func() {
		//https://bl.ocks.org/joyrexus/22c3ef0984ed957f54b9
		err := boltDB.Update(func(tx *bolt.Tx) error {
			upload, err := tx.CreateBucketIfNotExists([]byte("upload"))
			if err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}

			enc, err1 := json.Marshal(boltInfo)
			if err1 != nil {
				return err1
			}

			err = upload.Put([]byte(boltInfo.CheckSum), enc)
			return err
		})

		if err != nil {
			fmt.Printf("save data error:%v",err)
		}
	}()
	log.Printf("Start upload File: %s\n",file)

	boltInfo.CheckSum = checksum(file,*chunckSize)
	boltInfo.FilePath = file
	boltInfo.IsOver = false
	//boltInfo.OverTime = null


	var filechunk uint64 = *chunckSize//2*1024*1024 //1024// we settle for 8KB
	//file := "D:\\DOTA2Setup\\DOTA2Setup20160201\\Dota2.7z.001"//"C:\\Users\\wangxh\\Pictures\\1.png"//"D:\\DOTA2Setup\\DOTA2Setup20160201\\Dota2.7z.001"
	uid := uuid.NewV4()
	//fmt.Printf("UUIDv4: %s\n", uid)
	finfo, err := os.Stat(file)
	if err != nil {
		fmt.Println(finfo.Size(),finfo.Name(),err)
	}

	boltInfo.Uuid = uid.String()
	boltInfo.StartTime = time.Now()






	//checksum2 := checksum(file,filechunk)
	//fmt.Printf("%s checksum is %x\n", file,checksum2)
	//uid,err := uuid.FromString(checksum2)
	//fmt.Println(err)

	if finfo.Size() <= int64(*conNum)*(int64(filechunk)) {
		*conNum = int(math.Ceil(float64(float64(finfo.Size()) / float64(filechunk))))
	}

	//fmt.Println(math.Ceil(3.0/2.0))


	boltInfo.IndexMap = make(map[int]int)
	qqtotalparts := int(math.Ceil(float64(float64(finfo.Size()) / float64(filechunk))))
	for i := 0; i < int(math.Ceil(float64(float64(finfo.Size()) / float64(filechunk)))); i++  {
		boltInfo.IndexMap[i] = 0
	}


	var wg sync.WaitGroup

	wg.Add(*conNum)

	for i  := 0; i < *conNum;i++  {
		go func(index int) {
			upload(file,uid.String(),finfo.Name(),finfo.Size(),index,int64(index)*int64(filechunk),int64(filechunk),10)
			//
		}(i)
	}
	go func() {
		for {
			select {
			case r := <-pool:
				//fmt.Println("Acquire:共享资源",r.index)
				var index = -1
				locker.Lock()
				if r.isok {
					//delete(indexMap,r.index)
					boltInfo.IndexMap[r.index] = 1
				} else {
					boltInfo.IndexMap[r.index] = 0
				}
				for i,v := range boltInfo.IndexMap {
					if v == 0 {
						boltInfo.IndexMap[i] = -1
						index = i
						break
					}
				}

				locker.Unlock()
				if index > -1 {
					var size int64 = int64(filechunk)
					if finfo.Size() <= int64(uint64(index+1) * filechunk) {
						size = finfo.Size() - int64(uint64(index) * filechunk)
					}
					upload(file,uid.String(),finfo.Name(),finfo.Size(),index,int64(index)*int64(filechunk),int64(size),qqtotalparts)
				} else {
					wg.Done()
				}
				//	default:

			}
		}
	}()

	wg.Wait()

	uploadDone(uid.String(),finfo.Name(),finfo.Size(),qqtotalparts)

	log.Println("upload over")
	boltInfo.IsOver = true
	boltInfo.OverTime = time.Now()

}

func checksum(path string ,chuncksize uint64) string {

	file, err := os.Open(path)

	if err != nil {
		panic(err.Error())
	}

	defer file.Close()

	// calculate the file size
	info, _ := file.Stat()

	filesize := info.Size()

	blocks := uint64(math.Ceil(float64(filesize) / float64(chuncksize)))

	hash := md5.New()

	for i := uint64(0); i < blocks; i++ {
		blocksize := int(math.Min(float64(chuncksize), float64(filesize-int64(i*chuncksize))))
		buf := make([]byte, blocksize)

		file.Read(buf)
		io.WriteString(hash, string(buf)) // append into the hash
	}

	fmt.Printf("%s checksum is %x\n", file.Name(), hash.Sum(nil))
	var bytes []byte
	bytes = hash.Sum(nil)
	return string(bytes)
}


func main() {
	flag.Parse()
	var err error
	boltDB, err = bolt.Open("my.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer boltDB.Close()

	/*boltDB.View(func(tx *bolt.Tx) error {
		bk := tx.Bucket([]byte("upload"))

		c := bk.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("key=%s, value=%s\n", k, v)
		}
		return nil
	})
*/
	//fmt.Println("ss")
	//file := "D:\\DOTA2Setup\\DOTA2Setup20160201\\Dota2.7z.001"//"C:\\Users\\wangxh\\Pictures\\1.png"//"D:\\DOTA2Setup\\DOTA2Setup20160201\\Dota2.7z.001"
	//"E:\\GYJC\\VNC-5.2.2-Windows.exe"
	if filePath == nil || len(*filePath) == 0{
		flag.Usage()
		return
	}
	pool = make(chan uploadResult, *conNum)
	client := &http.Client{
		Transport: &tsdbrelayHTTPTransport{
			&httpcontrol.Transport{
				RequestTimeout:      time.Minute,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: *conNum,
			},
		},
	}
	http.DefaultClient = client

	uploadAll(*filePath)
	//go upload()

	time.Sleep(time.Millisecond * 500)
}
