package main

import (
	"flag"
	"log"
	"github.com/boltdb/bolt"
	Uploader "./uploader"
	"syscall"
	"os/signal"
	"os"
)

var chunckSize = flag.Uint64("c", 2*1024*1024, "chunk size,defaults to 2*1024*1024")
var filePath = flag.String("f", "", "Upload file Path")
var conNum = flag.Int("n", 5, "Batch Connections,defaults to 5")
var host = flag.String("h", "172.29.231.80:8082", "Upload host Path")

func main() {
	flag.Parse()
	boltDB, err := bolt.Open("uploader.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
		return
	}

	defer boltDB.Close()
	/*historyMap = make(map[string]boltUploadStruct)

	boltDB.View(func(tx *bolt.Tx) error {
		bk := tx.Bucket([]byte("upload"))

		if err := bk.ForEach(func(k, v []byte) error {

			//fmt.Printf("A %s is %s.\n", k, v)
			bolt := boltUploadStruct{}
			json.Unmarshal(v,&bolt)
			historyMap[string(k)] = bolt
			fmt.Printf("%s: %t.\t %s-%s  \n", bolt.FilePath, bolt.IsOver, bolt.StartTime, bolt.OverTime)
			return nil
		}); err != nil {
			return err
		}
		return nil
	})*/

	/*
	clear
	boltDB.Update(func(tx *bolt.Tx) error {
		bk := tx.Bucket([]byte("upload"))
		for key, _ := range historyMap {
			bk.Delete([]byte(key))
		}
		return nil
	})
	*/

	//fmt.Println("ss")
	//file := "D:\\DOTA2Setup\\DOTA2Setup20160201\\Dota2.7z.001"//"C:\\Users\\wangxh\\Pictures\\1.png"//"D:\\DOTA2Setup\\DOTA2Setup20160201\\Dota2.7z.001"
	//"E:\\GYJC\\VNC-5.2.2-Windows.exe"

	if filePath == nil || len(*filePath) == 0 {
		flag.Usage()
		return
	}
	chQuit := make(chan os.Signal, 1)

	signal.Notify(chQuit, syscall.SIGINT, syscall.SIGTERM)
	//log.Println(<-chQuit)

	uploader := Uploader.New(*conNum,*chunckSize,boltDB,*host,chQuit)
	uploader.UploadAll(*filePath)
	/*
	//log.Println(<-ch)
	uploadAll(*filePath)
	*/
	//go upload()

	//time.Sleep(time.Millisecond * 500)

	///Users/wangxiaohua/Downloads/AIA-SWEXSYS.ppt
}
