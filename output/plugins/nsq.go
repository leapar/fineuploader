package plugins

import (
	"sync"
	"../../def"
	"../../config"
	"github.com/nsqio/go-nsq"
	"time"
	"fmt"
	"github.com/qgymje/nsqpool"
	"../../output"
)

type OutputNSQ struct {
	lock sync.Mutex
	config config.Config
	nsqpool pool.Pool
}

func init()  {
	output.Add("nsq", func(config config.Config) def.Outputer {
		out := OutputNSQ{
			config:config,
		}
		out.init(config.OutputNsq.NsqServer)
		return &out
	})
}

func (srv *OutputNSQ)init(addr string) {
	factory := func() (*nsq.Producer, error) {
		config := nsq.NewConfig()
		return nsq.NewProducer(addr, config)
	}

	nsqPool, err := pool.NewChannelPool(64, 64, factory)

	srv.nsqpool = nsqPool
	producer, err := nsqPool.Get()
	//  try to ping
	err = producer.Ping()
	if nil != err {
		return
	}
}

func (srv*OutputNSQ)WriteChunks(cookie string,index int,datas []byte,uuid string,chunkSize int,totalSize int,filename string) string {
	start := time.Now().UnixNano()
	defer func() {
		def.WriteChunkTime.Update((float64(time.Now().UnixNano() - start) / float64(1e9)))
	}()
	datas,id := srv.config.Storage.PacketChunks(cookie,index,datas,uuid,chunkSize,totalSize,filename)
	producer, err := srv.nsqpool.Get()
	if err != nil {
		fmt.Println("nsqpool Get",err)
		panic(err)
	}
	defer producer.Close()
	err = producer.Publish(srv.config.OutputNsq.Topic,datas)
	if err != nil {
		fmt.Println("Publish",err)
		panic(err)
	}
	return id
}
