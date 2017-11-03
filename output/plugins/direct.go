package plugins

import (
	"../../def"
	"../../config"
	"time"
	"../../output"
)

type OutputDirect struct {
	config config.Config
}

func init()  {
	output.Add("direct", func(config config.Config) def.Outputer {
		out := OutputDirect{
			config:config,

		}

		return &out
	})
}

func (srv*OutputDirect)WriteChunks(cookie string,index int,datas []byte,uuid string,chunkSize int,totalSize int,filename string) string {
	start := time.Now().UnixNano()
	defer func() {
		def.WriteChunkTime.Update((float64(time.Now().UnixNano() - start) / float64(1e9)))
	}()
	datas,id := srv.config.Storage.PacketChunks(cookie,index,datas,uuid,chunkSize,totalSize,filename)
	srv.config.Storage.WriteChunkPacket(index,datas,id)

	return id
}
