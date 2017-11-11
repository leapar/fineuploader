package uploader

import (
	"math"
	"crypto/md5"
	"fmt"
	"os"
	"sync"
	"bytes"
	"mime/multipart"
	"strconv"
	"io/ioutil"
	"net/http"
	"time"
	"github.com/boltdb/bolt"

	//"github.com/satori/go.uuid"
	"io"
	"encoding/json"
	"log"
	"github.com/facebookgo/httpcontrol"
	"github.com/gosuri/uiprogress"
	"github.com/dustin/go-humanize"
	//"github.com/vbauerster/mpb"
	//"github.com/vbauerster/mpb/decor"
	"net/http/cookiejar"
	"context"
	//"github.com/satori/go.uuid"
	"errors"
	"github.com/satori/go.uuid"
)

type Uploader struct {
	boltDB *bolt.DB
	conNum int
	chunkSize uint64
	host string
	pool chan uploadResult
	chQuit chan os.Signal
	progress chan uploadResult
}


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

type BoltUploadStruct struct {
	CheckSum string
	//Uuid string
	FilePath string
	StartTime time.Time
	OverTime time.Time
	IsOver bool
	ChunkSize uint64
	//PartInfo []uploadResult
	IndexMap map[int]int

	OverIndex int
	TotalPart int
	mapLock sync.Mutex
}

const (
	/*
	red = iota   // red == 0
	blue         // blue == 1
	green        // green == 2
	*/
	UPLOAD_FLAG_UNKNOW = 0
	UPLOAD_FLAG_DOING = -1
	UPLOAD_FLAG_OK = 1
)


//var historyMap map[string]boltUploadStruct

func New(conNum int,chunkSize uint64,db *bolt.DB,host string,chQuit chan os.Signal) *Uploader {
	//jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatal(err)
	}
	client := &http.Client{
		Jar: jar,
		Transport: &tsdbrelayHTTPTransport{
			&httpcontrol.Transport{
				RequestTimeout:      time.Minute,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: conNum,
			},
		},
	}
	http.DefaultClient = client

	return &Uploader{
		boltDB:db,
		conNum:conNum,
		chunkSize:chunkSize,
		host:host,
		pool:make(chan uploadResult, conNum),
		chQuit:chQuit,
		progress:make(chan uploadResult, conNum),
	}
}

func init() {

}

func (s* Uploader) upload(ctx context.Context,file string,fuid string,filename string,qqtotalfilesize int64,qqpartindex int, qqpartbyteoffset int64,qqchunksize int64,qqtotalparts int ) {
	var bResult bool = false
	//fmt.Println(filename,qqpartindex)
	if fuid == "" {
		fmt.Println("fuid  nil")
	}
	defer func() {
		select {
		case <-ctx.Done():
			_ = ctx.Err()
		default:
			s.pool <- uploadResult{
				qqpartindex,
				bResult,
			}
			s.progress <- uploadResult{
				qqpartindex,
				bResult,
			}
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

	//qqpartbyteoffset
	//qqchunksize
	//qqtotalparts
	//qqtotalfilesize
	var fw io.Writer

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

	fw, err = w.CreateFormFile("qqfile", filename)
	if err != nil {
		return
	}

	_,err = f.Seek(qqpartbyteoffset,io.SeekStart)

	if _, err = io.CopyN(fw,f,qqchunksize); err != nil {
		return
	}



	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	//body := bytes.NewBuffer(reader.buf.Bytes())


	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/upload",s.host), &b)//HTTP://127.0.0.1:8081/upload
	if err != nil {
		///verbose("bosun connect error: %v", err)
		return
	}

	req = req.WithContext(ctx)
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		//verbose("bosun relay error: %v", err)
		return
	}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		fmt.Printf("%s -- %v\n", string(buf), err)
	}


	resp.Body.Close()
	if err != nil {
		fmt.Println(err)
		return
	}

	ret := uploadRet{}

	err = json.Unmarshal(buf,&ret)
	if err != nil {
		fmt.Println(err)
		return;
	}
	if !ret.Success {
		fmt.Println(ret)
		return
	}
	//verbose("bosun relay success")
	bResult = true

}

func (s* Uploader) preUpload(fuid string,filename string,qqtotalfilesize int64,chucksize int ) error{
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	// Add your image file

	fw, err := w.CreateFormField("qqtotalfilesize")
	if err != nil {
		return err
	}

	if _, err = fw.Write([]byte(strconv.FormatInt(qqtotalfilesize, 10))); err != nil {
		return err
	}

	if fw, err = w.CreateFormField("qqfilename"); err != nil {
		return err
	}
	if _, err = fw.Write([]byte(filename)); err != nil {
		return err
	}


	if fw, err = w.CreateFormField("qquuid"); err != nil {
		return err
	}
	if _, err = fw.Write([]byte(fuid)); err != nil {
		return err
	}

	if fw, err = w.CreateFormField("qqchunksize"); err != nil {
		return err
	}
	if _, err = fw.Write([]byte(strconv.Itoa(chucksize))); err != nil {
		return err
	}

	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	//body := bytes.NewBuffer(reader.buf.Bytes())
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/preupload",s.host), &b)//"HTTP://127.0.0.1:8081/chunksdone
	if err != nil {
		///verbose("bosun connect error: %v", err)
		return err
	}
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		//verbose("bosun relay error: %v", err)
		return err
	}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("%s -- %v\n", string(buf), err)
		return err
	}

	resp.Body.Close()
	//verbose("bosun relay success")
	return nil
}

/*
qquuid:7174ed7c-8276-48a0-be2f-d87e70935282
qqfilename:win32-x64-51_binding.node
qqtotalfilesize:2172928
qqtotalparts:2
*/
func (s* Uploader) uploadDone(fuid string,filename string,qqtotalfilesize int64,qqtotalparts int )error {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	// Add your image file

	fw, err := w.CreateFormField("qqtotalparts")
	if err != nil {
		return err
	}
	if _, err = fw.Write([]byte(strconv.Itoa(qqtotalparts))); err != nil {
		return err
	}

	if fw, err = w.CreateFormField("qqtotalfilesize"); err != nil {
		return err
	}

	if _, err = fw.Write([]byte(strconv.FormatInt(qqtotalfilesize, 10))); err != nil {
		return err
	}

	if fw, err = w.CreateFormField("qqfilename"); err != nil {
		return err
	}
	if _, err = fw.Write([]byte(filename)); err != nil {
		return err
	}


	if fw, err = w.CreateFormField("qquuid"); err != nil {
		return err
	}
	if _, err = fw.Write([]byte(fuid)); err != nil {
		return err
	}


	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	//body := bytes.NewBuffer(reader.buf.Bytes())
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/chunksdone",s.host), &b)//"HTTP://127.0.0.1:8081/chunksdone
	if err != nil {
		///verbose("bosun connect error: %v", err)
		return err
	}
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		//verbose("bosun relay error: %v", err)
		return err
	}
	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("%s -- %v\n", string(buf), err)
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("code:%d",resp.StatusCode))
	}

	//verbose("bosun relay success")

	return nil
}

func (bolt* BoltUploadStruct)setMapData(key int,value int)  {
	bolt.mapLock.Lock()
	defer bolt.mapLock.Unlock()
	bolt.IndexMap[key] = value
}

func (s* Uploader) UploadAll(file string) {
	//fmt.Println("UploadAll")


	ctx,cancel := context.WithCancel(context.Background())


	/*
		1. 判断文件大小
		2. 如果文件大于chuncksize进行分片
		3. 启动go rountime
		4. 获取已经上传完成的分片id
	*/
	/*mp := mpb.New()
	bar2 := mp.AddBar(int64(100),
		mpb.PrependDecorators(
			decor.StaticName("sss", 0, decor.DwidthSync|decor.DidentRight),
			decor.ETA(4, decor.DSyncSpace),
		),
		mpb.AppendDecorators(
			decor.Percentage(5, 0),
		),
	)*/

	uiprogress.Start()
	bar := uiprogress.AddBar(100).AppendCompleted()
	//bar.AppendCompleted()
	//bar.PrependElapsed()

	boltInfo := BoltUploadStruct{}






	//defer boltDB.Close()
	defer func() {
		//fmt.Println("uiprogress.Stop()")
		uiprogress.Stop()

	}()
	log.Printf("Start upload File: %s\n",file)

	boltInfo.CheckSum = s.checksum(file,s.chunkSize)
	boltInfo.FilePath = file
	boltInfo.IsOver = false

	if s.boltDB != nil {
		s.boltDB.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("upload"))
			if b == nil {
				return nil
			}
			v := b.Get([]byte(boltInfo.CheckSum))
			//fmt.Printf("%s\n", v)
			bolt := BoltUploadStruct{}
			err := json.Unmarshal(v,&bolt)
			if err == nil {
				boltInfo = bolt
				fmt.Println("start time:",boltInfo.StartTime)
			}

			return nil
		})
	}


	if boltInfo.IsOver == true {
		bar.Set(bar.Total)
		//bar2.Incr(100)
		log.Println("already upload over")
		return
	}
	//boltInfo.OverTime = null
	/*if value, ok := historyMap[boltInfo.CheckSum]; ok {
		boltInfo = value
		if value.IsOver == true {
			log.Println("already upload over")
			return
		}
	}*/

	var filechunk uint64 = s.chunkSize//2*1024*1024 //1024// we settle for 8KB
	var cNum int = s.conNum
	if boltInfo.ChunkSize > 0 {
		filechunk = boltInfo.ChunkSize
	}

	//file := "D:\\DOTA2Setup\\DOTA2Setup20160201\\Dota2.7z.001"//"C:\\Users\\wangxh\\Pictures\\1.png"//"D:\\DOTA2Setup\\DOTA2Setup20160201\\Dota2.7z.001"
	//uid := uuid.NewV4()
	//uid,err = uuid.FromString(boltInfo.CheckSum)
	//fmt.Printf("UUIDv4: %s\n", uid)
	finfo, err := os.Stat(file)
	if err != nil {
		fmt.Println(finfo.Size(),finfo.Name(),err)
	}

	//boltInfo.Uuid = uid.String()
	boltInfo.StartTime = time.Now()
	boltInfo.ChunkSize = filechunk
	boltInfo.OverIndex = 0
	//checksum2 := checksum(file,filechunk)
	//fmt.Printf("%s checksum is %x\n", file,checksum2)

	//fmt.Println(err)
	qqtotalparts := int(math.Ceil(float64(float64(finfo.Size()) / float64(filechunk))))
	boltInfo.TotalPart = qqtotalparts

	if finfo.Size() <= int64(cNum)*(int64(filechunk)) {
		cNum = boltInfo.TotalPart
	}

	err = s.preUpload(boltInfo.CheckSum,finfo.Name(),finfo.Size(),int(filechunk))
	if err != nil {
		fmt.Println(err)
		return
	}
	//fmt.Println(math.Ceil(3.0/2.0))




	speed_start := time.Now()
	speed_elapsed := time.Duration(1)
	bar.AppendFunc(
		func(b *uiprogress.Bar) string {
			// elapsed := b.TimeElapsed()
			if b.Current() < b.Total {
				speed_elapsed = time.Now().Sub(speed_start)
			}
			speed := uint64(float64(b.Current()) * float64(boltInfo.ChunkSize) / speed_elapsed.Seconds())
			return humanize.IBytes(speed) + "/sec"
		})

	var wg sync.WaitGroup
	wg.Add(2)


	if len(boltInfo.IndexMap) == 0 {
		boltInfo.IndexMap = make(map[int]int)

		for i := 0; i < boltInfo.TotalPart; i++  {
			boltInfo.setMapData(i,UPLOAD_FLAG_UNKNOW)
		}

		for i  := 0; i < cNum;i++  {
			go func(index int) {
				boltInfo.setMapData(index,UPLOAD_FLAG_DOING)

				s.upload(ctx,file,boltInfo.CheckSum,finfo.Name(),finfo.Size(),index,int64(index)*int64(filechunk),int64(filechunk),qqtotalparts)
				//
			}(i)
		}
	} else {
		var inum = 0

		for key, value := range boltInfo.IndexMap {
			if value == UPLOAD_FLAG_DOING {
				boltInfo.setMapData(key,UPLOAD_FLAG_UNKNOW)
			}

			if value == UPLOAD_FLAG_OK {
				boltInfo.OverIndex++
			}

			if value != UPLOAD_FLAG_OK  && inum < cNum {
				inum++
				boltInfo.setMapData(key,UPLOAD_FLAG_DOING)
				go func(index int) {
					s.upload(ctx,file,boltInfo.CheckSum,finfo.Name(),finfo.Size(),index,int64(index)*int64(filechunk),int64(filechunk),qqtotalparts)
					//
				}(key)
			}
		}
	}



	quitSignal := make(chan int,1)
	go func() {
		defer func() {
			wg.Done()
		}()
		for {
			select {
			case <-quitSignal:
				//fmt.Println("quitSignal")
				return
			case r := <-s.progress:
				//fmt.Println(float64(boltInfo.OverIndex) / float64(len(boltInfo.IndexMap)))
				if r.isok {
					//delete(indexMap,r.index)
					//boltInfo.IndexMap[r.index] = UPLOAD_FLAG_OK
					boltInfo.OverIndex++
					//bar.Incr()
					//bar2.Incr(int((float64(1) / float64(len(boltInfo.IndexMap)))*100))
				}
				boltInfo.mapLock.Lock()
				bar.Set(int((float64(boltInfo.OverIndex) / float64(len(boltInfo.IndexMap)))*100))
				boltInfo.mapLock.Unlock()
			}
		}

		//for i := 1; i <= bar.Total; i++ {
		/*select {
		case <-s.chQuit:
			return

		}*/
		///	bar.Set(int((float64(boltInfo.OverIndex) / float64(len(boltInfo.IndexMap)))*100))
		///	time.Sleep(time.Millisecond * 100)
		//}
	}()

	s.progress <- uploadResult {
		-1,
		false,
	}


	go func() {
		defer func() {
			quitSignal <- 1
			wg.Done()
		}()
		overIndex := 0
		for {
			select {
			case <-s.chQuit:
				cancel()
				//wg.Add(-cNum)
				//fmt.Println("chQuit")
				return
			case r := <-s.pool:
				//fmt.Println("Acquire:共享资源",r.index)
				var index = -1

				if r.isok {
					//delete(indexMap,r.index)
					boltInfo.setMapData(r.index,UPLOAD_FLAG_OK)
				} else {
					boltInfo.setMapData(r.index,UPLOAD_FLAG_UNKNOW)
					//time.Sleep(time.Millisecond * 100)
				}
				boltInfo.mapLock.Lock()
				for i,v := range boltInfo.IndexMap {
					if v == UPLOAD_FLAG_UNKNOW {
						boltInfo.IndexMap[i] = UPLOAD_FLAG_DOING
						index = i
						break
					}
				}
				boltInfo.mapLock.Unlock()


				if index > -1 {
					var size int64 = int64(filechunk)
					if finfo.Size() <= int64(uint64(index+1) * filechunk) {
						size = finfo.Size() - int64(uint64(index) * filechunk)
					}
					go s.upload(ctx,file,boltInfo.CheckSum,finfo.Name(),finfo.Size(),index,int64(index)*int64(filechunk),int64(size),qqtotalparts)
				} else {
					overIndex++
					if overIndex == cNum {
						return
					}

				}
				//	default:
			}
		}
	}()

	wg.Wait()


	var allOver bool = true
	boltInfo.mapLock.Lock()
	for _,v := range boltInfo.IndexMap {
		if v != 1 {
			allOver = false
			break
		}
	}
	boltInfo.mapLock.Unlock()

	if allOver {
		attemp := -1
		err := s.uploadDone(boltInfo.CheckSum,finfo.Name(),finfo.Size(),qqtotalparts)
		if err != nil {
			for attemp >= 0 {//别重试 hadoop时候第一次文件合并，但中途出错有些文件就已经删除了，第二次合并文件就不见了
				time.Sleep(time.Microsecond * 5)
				err := s.uploadDone(boltInfo.CheckSum,finfo.Name(),finfo.Size(),qqtotalparts)
				if err == nil {
					break
				} else {
					fmt.Println("attemp:",5-attemp)
				}
				attemp--
			}

		}
		boltInfo.IsOver = true
		log.Println("upload over")
		boltInfo.OverTime = time.Now()
	} else {
		log.Println("exit....")
	}

	//https://bl.ocks.org/joyrexus/22c3ef0984ed957f54b9
	if  s.boltDB != nil {
		err = s.boltDB.Update(func(tx *bolt.Tx) error {
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
	}


}

func (s* Uploader) checksum(path string ,chuncksize uint64) string {
	

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
	var bytes []byte
	bytes = hash.Sum(nil)
	retStr := fmt.Sprintf("%x",bytes)
	fmt.Printf("checksum is %s \n",retStr )

	return retStr
}