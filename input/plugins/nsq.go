package plugins

import (
	"sync"
	"../../input"
	"../../def"
	"../../config"
	"github.com/nsqio/go-nsq"
	"fmt"
	"gopkg.in/mgo.v2/bson"
	"github.com/qgymje/nsqpool"
)

type IutputNSQ struct {
	lock sync.Mutex
	config config.Config
	nsqpool pool.Pool
}

func init()  {
	input.Add("nsq", func(config config.Config) def.Iutputer {
		in := IutputNSQ{
			config:config,
		}
		in.init(config.InputNsq.NsqServer)
		return &in
	})

}

func (srv *IutputNSQ)init(addr string) {
	cfg := nsq.NewConfig()
	consumer, err := nsq.NewConsumer(srv.config.InputNsq.Topic,srv.config.InputNsq.Channel,cfg)
	if nil != err {
		panic(err)
	}
	consumer.AddHandler(srv)
	if err := consumer.ConnectToNSQLookupd(addr); err != nil {
		fmt.Println("ConnectToNSQLookupd", err)
		panic(err)
	}
}

func (srv *IutputNSQ) HandleMessage(msg *nsq.Message) error {
	var err error

	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
		}
	}()

	var chunk def.GfsChunk
	err = bson.Unmarshal(msg.Body,&chunk)
	if err != nil {
		return err
	}
	id,_ := (chunk.FilesId).(bson.ObjectId)

	srv.config.Storage.WriteChunkPacket(chunk.N,msg.Body,id.Hex(),"")

	return err
}

