package def

import (
	"net/http"
)

type Storager interface {

	UploadDoneHandler(uuid string,file_id string,filename string,totalpart int)

	DownloadHandler(w http.ResponseWriter, req *http.Request)

	WriteGridFile(filename string,
		shortName string,
		uuid string,
		chunkSize int,
		totalSize int,
		totalPart int,
		offset int,
		index int,
		datas []byte) error



	PacketChunks(cookie string,index int,datas []byte,uuid string,chunkSize int,totalSize int,filename string) (out []byte,oid string)

	WriteChunkPacket(index int,datas []byte,fileid string,uuid string)

	GetFinalFileID(cookie string, uuid string,chunkSize int,totalFileSize int,filename string) interface{}
}