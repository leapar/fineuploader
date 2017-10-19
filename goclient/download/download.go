package download

import (
	"os"
	"github.com/boltdb/bolt"
	"fmt"
	"context"
	"time"
	"net/http"

	"syscall"
	"os/signal"
	"log"
	"regexp"
	"io"
	"path/filepath"
	"net/url"
	"strings"
	"math"
	//"encoding/json"
	"github.com/satori/go.uuid"
	"github.com/gosuri/uiprogress"
	"sync"
	"encoding/json"
)

var (
	contentDispositionRe *regexp.Regexp
 	locker sync.Mutex
)

const (
	rr           = 120 * time.Millisecond
	maxRedirects = 10
	downloadDir = "downloads"
	userAgent    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/55.0.2883.95 Safari/537.36"

	DOWNLOAD_FLAG_NORMAL = 0
	DOWNLOAD_FLAG_DOING = -1
	DOWNLOAD_FLAG_OK = 1
)

func init() {
	// https://regex101.com/r/N4AovD/3
	contentDispositionRe = regexp.MustCompile(`filename[^;\n=]*=(['"](.*?)['"]|[^;\n]*)`)
}

type DownLoaderInfo struct {
	FilePath string//文件存储名称
	CheckSum string
	UUid string//本地生成的uuid

	AcceptRanges string
	FileSize int64//文件总大小
	Location string//真实请求地址
	Parts []*Part//每个切片信息
	OverIndex int//已经下载完成的切片数
	ChunkSize int64//块大小
	StartTime time.Time
	OverTime time.Time
	Url string//请求地址
}

type DownLoader struct {
	boltDB *bolt.DB
	conNum int//并发量
	pool chan goResult
	chQuit chan os.Signal//退出信号量
	progress chan goResult//进度条
	info *DownLoaderInfo
}

type Part struct {
	//Name                 string
	Start, Stop			 int64
	Flag                 int
	Index				 int
}


type goResult struct {
	index int
	isok bool
}

func New(chunkSize int64,conNum int,boltDb *bolt.DB)  *DownLoader{
	return &DownLoader{
		info:&DownLoaderInfo{
			ChunkSize:chunkSize,
			UUid: uuid.NewV4().String(),
		},
		conNum: conNum,
		pool:make(chan goResult, conNum),
		chQuit: make(chan os.Signal,1),
		progress:make(chan goResult, conNum),
		boltDB:boltDb,
	}
}

func (d *DownLoader) Download(url string,timeout int) {
	isOver := false
	d.info.Url = url
	d.boltDB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("download"))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(d.info.Url))
		//fmt.Printf("%s\n", v)
		bolt := DownLoaderInfo{}
		err := json.Unmarshal(v,&bolt)
		if err == nil {
			d.info = &bolt
			fmt.Println("start time:",bolt.StartTime)
		}

		return nil
	})

	if !d.info.OverTime.IsZero()  {
		fmt.Println("already down")
		return
	}

	d.info.StartTime = time.Now()

	code,err := d.follow(url,"")
	d.exitOnError(err)

	ctx := context.Background()
	var cancel context.CancelFunc
	//if timeout > 0 {
	//	ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	//} else {
		ctx, cancel = context.WithCancel(ctx)
	//}
	go d.onCancelSignal(cancel)

	if code == http.StatusOK {
		d.calcParts()
	}

	/*
		1. 判断文件大小
		2. 如果文件大于chuncksize进行分片
		3. 启动go rountime
		4. 获取已经上传完成的分片id
	*/

	uiprogress.Start()
	bar := uiprogress.AddBar(100).AppendCompleted()
	//bar.AppendCompleted()
	//bar.PrependElapsed()


//	defer d.boltDB.Close()
	defer func() {
		//fmt.Println("uiprogress.Stop()")
		uiprogress.Stop()
	}()

	var filechunk int64 = d.info.ChunkSize//2*1024*1024 //1024// we settle for 8KB
	var cNum int = d.conNum

	//checksum2 := checksum(file,filechunk)
	//fmt.Printf("%s checksum is %x\n", file,checksum2)

	//fmt.Println(err)

	if d.info.FileSize <= int64(cNum)*(int64(filechunk)) {
		cNum = len(d.info.Parts)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	var inum = 0
	for key, value := range d.info.Parts {
		if value.Flag == DOWNLOAD_FLAG_DOING {
			d.info.Parts[key].Flag = DOWNLOAD_FLAG_NORMAL
		}
		if value.Flag == DOWNLOAD_FLAG_OK {
			d.info.OverIndex++
		}
		if value.Flag != DOWNLOAD_FLAG_OK  && inum < cNum {
			inum++
			go func(index int) {
				d.downloadPart(ctx,d.info.Parts[index])
			}(key)
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
			case r := <-d.progress:
				//fmt.Println(float64(boltInfo.OverIndex) / float64(len(boltInfo.IndexMap)))
				if r.isok {
					//delete(indexMap,r.index)
					//boltInfo.IndexMap[r.index] = UPLOAD_FLAG_OK
					d.info.OverIndex++
					//bar.Incr()
					//bar2.Incr(int((float64(1) / float64(len(boltInfo.IndexMap)))*100))
				}

				bar.Set(int((float64(d.info.OverIndex) / float64(len(d.info.Parts)))*100))
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

	d.progress <- goResult {
		-1,
		false,
	}


	go func() {
		defer func() {
			quitSignal <- 1
			wg.Done()
		}()
		overNum := 0
		for {
			select {
			case <-d.chQuit:
				//wg.Add(-cNum)
				//fmt.Println("chQuit")
				return
			case r := <-d.pool:
				//fmt.Println("Acquire:共享资源",r.index)
				var index = -1
				locker.Lock()
				if r.isok {
					//delete(indexMap,r.index)
					d.info.Parts[r.index].Flag = DOWNLOAD_FLAG_OK
				} else {
					d.info.Parts[r.index].Flag = DOWNLOAD_FLAG_NORMAL
					//time.Sleep(time.Millisecond * 100)
				}
				for i,v := range d.info.Parts {
					if v.Flag == DOWNLOAD_FLAG_NORMAL {
						d.info.Parts[i].Flag = DOWNLOAD_FLAG_DOING
						index = i
						break
					}
				}

				locker.Unlock()
				if index > -1 {
					go d.downloadPart(ctx,d.info.Parts[index])
				} else {
					overNum++
					if overNum == cNum{
						isOver = true
						return
					}
				}
				//	default:
			}
		}
	}()

	wg.Wait()

	if isOver {
		d.concatenateParts()
	}

	err = d.boltDB.Update(func(tx *bolt.Tx) error {
		upload, err := tx.CreateBucketIfNotExists([]byte("download"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		enc, err1 := json.Marshal(d.info)
		if err1 != nil {
			return err1
		}

		err = upload.Put([]byte(d.info.Url), enc)
		return err
	})

	if err != nil {
		fmt.Printf("save data error:%v",err)
	}
	log.Println("exit....")
}

func (d*DownLoader)downloadPart(ctx context.Context,p* Part)  {
	var bResult bool = false
	//fmt.Println("downloadPart")
	defer func() {
		//fmt.Println("defer.....")
		select {
		case <-ctx.Done():
			_ = ctx.Err()
			//fmt.Println(err)
		default:
			//fmt.Println("deferdeferdefer")
			d.pool <- goResult {
				p.Index,
				bResult,
			}

			d.progress <- goResult {
				p.Index,
				bResult,
			}
		}


		//fmt.Println("upload over:",qqpartindex)
	}()

	//fmt.Println(d.url)
	//ctx := context.Background()
	//var cancel context.CancelFunc
	//if timeout > 0 {
	//	ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	//} else {
	//	ctx, cancel = context.WithCancel(ctx)
	//}
	//go onCancelSignal(cancel)

	req, err := http.NewRequest(http.MethodGet, d.info.Location, nil)
	if err != nil {
		fmt.Println(err)
		return
		//panic(err)
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Range", p.getRange())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		return
		//panic(err)
	}

	//total := p.Stop - p.Start + 1
	if resp.StatusCode == http.StatusOK  || resp.StatusCode == http.StatusPartialContent{
		//fmt.Println(resp.StatusCode)
		dst, err1 := os.Create(fmt.Sprintf("%s/%s/%d.part", downloadDir, d.info.UUid,p.Index))
		if err1 != nil {
			log.Println("error create file")
			return
		}
		defer dst.Close()

		err = p.writeToFile(dst,resp)
		if err != nil {
			fmt.Println("writeToFile error")
			return
		}

		bResult = true
	} else {
		fmt.Println(resp.StatusCode)
	}
}

func (d*DownLoader)onCancelSignal(cancel context.CancelFunc) {
	defer cancel()
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs

	log.Printf("%v: canceling...\n", sig)
	d.chQuit <- sig
}

func (p *Part) getRange() string {
	if p.Stop <= 0 {
		return ""
	}
	start := p.Start
	/*if p.Written > 0 {
		start = start + p.Written
	}*/
	return fmt.Sprintf("bytes=%d-%d", start, p.Stop)
}


func (p *Part) writeToFile(dst *os.File, resp *http.Response) (err error) {
	defer resp.Body.Close()

	for i := 0; i < 3; i++ {
		//var written int64
		_, err = io.Copy(dst, resp.Body)
		//fmt.Println(written)
		//p.Written += written
		if err != nil && isTemporary(err) {
			//fmt.Println("isTemporary")
			time.Sleep(1e9)
			continue
		}
		break
	}

	return
}


func (d*DownLoader)follow(fileUrl, outFileName string) (int,error) {
	var retCode int = http.StatusNotFound
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	next := fileUrl

	var redirectsFollowed int
	for {
		log.Printf("%s\n", next)
		//fmt.Printf("HTTP request sent, awaiting response... ")
		req, err := http.NewRequest(http.MethodGet, next, nil)
		if err != nil {
			//fmt.Println()
			return retCode, err
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := client.Do(req)
		if err != nil {
			//fmt.Println()
			return  retCode,err
		}
		defer resp.Body.Close()
		//fmt.Println(resp.Status)

		if outFileName == "" {
			outFileName = d.trimFileName(d.parseContentDisposition(resp.Header.Get("Content-Disposition")))
			if outFileName == "" {
				outFileName = d.trimFileName(filepath.Base(fileUrl))
				outFileName, _ = url.QueryUnescape(outFileName)
			}
		}
		retCode = resp.StatusCode
		/*al = &ActualLocation{
			Location:          next,
			SuggestedFileName: outFileName,
			AcceptRanges:      resp.Header.Get("Accept-Ranges"),
			StatusCode:        resp.StatusCode,
			ContentLength:     resp.ContentLength,
			ContentMD5:        resp.Header.Get("Content-MD5"),
		}*/
		d.info.AcceptRanges = resp.Header.Get("Accept-Ranges")
		d.info.FilePath = outFileName
		d.info.FileSize = resp.ContentLength
		d.info.Location = next

		if !d.isRedirect(resp.StatusCode) {
			if resp.StatusCode == http.StatusOK {
				//humanSize := mpb.Format(resp.ContentLength).To(mpb.UnitBytes)

				log.Printf("Length: %d [%s]\n", resp.ContentLength,resp.Header.Get("Content-Type"))
				//format := fmt.Sprintf("Length: %%s [%s]\n", resp.Header.Get("Content-Type"))
				//var length string
				/*if totalWritten > 0 && d.AcceptRanges != "" {
					//remaining := resp.ContentLength - totalWritten

				} else if resp.ContentLength < 0 {
					length = "unknown"
				} else {
					//length = fmt.Sprintf("%d (%s)", resp.ContentLength, humanSize)
				}
				fmt.Printf(format, length)*/
				if d.info.AcceptRanges == "" {
					fmt.Println("Looks like server doesn't support ranges (no party, no resume)")
				}
				fmt.Printf("Saving to: %q\n\n", outFileName)
			}
			break
		}

		loc, err := resp.Location()
		if err != nil {
			return retCode, err
		}
		redirectsFollowed++
		if redirectsFollowed > maxRedirects {
			return retCode,fmt.Errorf("maximum number of redirects (%d) followed", maxRedirects)
		}
		next = loc.String()
		fmt.Printf("Location: %s [following]\n", next)
	}
	return retCode, nil
}

func (d*DownLoader)trimFileName(name string) string {
	name = strings.Split(name, "?")[0]
	name = strings.Trim(name, " ")
	return name
}

func (d*DownLoader)isRedirect(status int) bool {
	return status > 299 && status < 400
}

func (d*DownLoader)parseContentDisposition(input string) string {
	groups := contentDispositionRe.FindAllStringSubmatch(input, -1)
	if groups == nil {
		return ""
	}
	for _, group := range groups {
		if group[2] != "" {
			return group[2]
		}
		split := strings.Split(group[1], "'")
		if len(split) == 3 && strings.ToLower(split[0]) == "utf-8" {
			unescaped, _ := url.QueryUnescape(split[2])
			return unescaped
		}
		if split[0] != `""` {
			return split[0]
		}
	}
	return ""
}


func (d*DownLoader)exitOnError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func (d *DownLoader) calcParts() {
	fileDir := fmt.Sprintf("%s/%s", downloadDir, d.info.UUid)
	if err := os.MkdirAll(fileDir, 0777); err != nil {
		fmt.Println("MkdirAll error")
		return
	}

	partSize := int64(math.Ceil(float64(float64(d.info.FileSize) / float64(d.info.ChunkSize))))
	if partSize <= 0 {
		//return
		partSize = 1
	}
	d.info.Parts = make([]*Part, partSize)

	for i := int64(0); i < partSize; i++ {
		//stop = start
		//start = stop - partSize
		stop := (i+1)*d.info.ChunkSize - 1
		if i == partSize -1 {
			stop = d.info.FileSize - 1
		}

		d.info.Parts[i] = &Part{
			Index: int(i),
			//Name:  fmt.Sprintf("%s.part%d", al.SuggestedFileName, i),
			Start: d.info.ChunkSize * i,
			Stop:  stop,
			Flag:	DOWNLOAD_FLAG_NORMAL,
		}
	}

	//js,_ := json.Marshal(d)
	//fmt.Println(string(js))
}



func (d *DownLoader) concatenateParts() error {
	fmt.Println("concat part files to single file")
	var wg sync.WaitGroup
	wg.Add(1)

	bar := uiprogress.AddBar(100).AppendCompleted()

	go func() error {
		defer wg.Done()
		finalFilename := fmt.Sprintf("%s/%s/%s", downloadDir, d.info.UUid,d.info.FilePath)

		f, err := os.Create(finalFilename)
		if err != nil {
			return err
		}
		defer f.Close()

		var totalWritten int64
		for i := 0; i < len(d.info.Parts); i++ {
			part := fmt.Sprintf("%s/%s/%d.part", downloadDir, d.info.UUid,i)
			fparti, err := os.Open(part)
			if err != nil {
				return err
			}
			written, err := io.Copy(f, fparti)
			if err != nil {
				log.Println(err)
				//writeHttpResponse(w, http.StatusInternalServerError, err)
				return err
			}
			fparti.Close()
			totalWritten += written

			if err := os.Remove(part); err != nil {
				log.Printf("Error: %v", err)
			}
			bar.Set(int(float64(i+1)/float64(len(d.info.Parts))*100))
			time.Sleep(time.Millisecond * 20)
		}

		return nil
	}()
	d.info.OverTime = time.Now()
	wg.Wait()

	return nil
}
