package exporter

import (
	"../config"
	"gopkg.in/mgo.v2"
	"log"
	"../def"
	"github.com/colinmarc/hdfs"
	"fmt"
	"../hdfspool"
	"gopkg.in/mgo.v2/bson"
	"io"
	"github.com/cheggaaa/pb"
)

type Gridfs2Hdfs struct {
	config config.Config
	session *mgo.Session
	hdfspool pool.Pool
}

func New(config config.Config) *Gridfs2Hdfs {
	return &Gridfs2Hdfs{
		config:config,
	}
}

func (this *Gridfs2Hdfs) InitMongoDB() error{
	session, err := mgo.Dial(this.config.InputMongo.MongoServer)

	log.Println("dial mongodb:",this.config.InputMongo.MongoServer)
	if err != nil {
		log.Println("[grid]mgo.Dial ", err)
		return err
	}
	this.session = session

	index := mgo.Index{
		Key:    []string{"files_id", "n"},
		Unique: true,
	}

	database := session.DB(def.DATA_BASE)
	gridFs := database.GridFS(def.PREFIX)
	gridFs.Chunks.EnsureIndex(index)
	return nil
}

func (this *Gridfs2Hdfs) InitHDFS() error{
	factory := func() (*hdfs.Client, error) {
		return  hdfs.NewClient(hdfs.ClientOptions{Addresses:[]string{this.config.OutputHdfs.Url},User:this.config.OutputHdfs.User})
	}

	nsqPool, err := pool.NewClientPool(10, 64, factory)
	if nil != err {
		return err
	}
	this.hdfspool = nsqPool

	return nil
}

func (this *Gridfs2Hdfs) Start() {
	err := this.InitHDFS()
	if nil != err {
		fmt.Println(err)
		return
	}
	err = this.InitMongoDB()
	if nil != err {
		fmt.Println(err)
		return
	}

	defer this.hdfspool.Close()
	defer this.session.Close()

	session := this.session.Copy()
	defer session.Close()
	database := session.DB(def.DATA_BASE)
	gridFs := database.GridFS(def.PREFIX)

	var objFile def.GfsFile
	err = gridFs.Files.Find(bson.M{ "md5": this.config.InputMongo.FileMd5}).One(&objFile)
	if err != nil {
		log.Println("Find file:",err)
		return
	}

	file,err := gridFs.OpenId(objFile.Id)
	if err != nil {
		log.Println("open file:",err)
		return
	}
	defer file.Close()
	client,err := this.hdfspool.Get()
	if err != nil {
		log.Println("hdfspool get:",err)
		return
	}
	defer client.Close()

	filepath := fmt.Sprintf("/%s/%s",def.DATA_BASE,objFile.Filename)
	f,err := client.Create(filepath)
	if err != nil {
		log.Println("Create File :",err)
		return
	}
	defer f.Close()

	bar := pb.New(int(objFile.Length))
	// show percents (by default already true)
	bar.ShowPercent = true
	// show bar (by default already true)
	bar.ShowBar = true
	// no counters
	bar.ShowCounters = false
	// show "time left"
	bar.ShowTimeLeft = true
	// show average speed
	bar.ShowSpeed = true
	// sets the width of the progress bar
	bar.SetWidth(100)
	// sets the width of the progress bar, but if terminal size smaller will be ignored
	//bar.SetMaxWidth(80)
	// convert output to readable format (like KB, MB)
	bar.SetUnits(pb.U_BYTES)
	bar.Start()

	reader := bar.NewProxyReader(file)
	n,err := io.Copy(f, reader)
	if err != nil {
		log.Println("Copy :",err,n)
		return
	}
//	fmt.Println(n)

	bar.Finish()
}