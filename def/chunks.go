package def

import (
	"time"
	"gopkg.in/mgo.v2/bson"
)

type GfsFile struct {
	Id          interface{} "_id"
	ChunkSize   int         "chunkSize"
	UploadDate  time.Time   "uploadDate"
	Length      int64       ",minsize"
	MD5         string
	Filename    string    ",omitempty"
	ContentType string    "contentType,omitempty"
	Metadata    *bson.Raw ",omitempty"
}

type GfsChunk struct {
	Id      interface{} "_id"
	FilesId interface{} "files_id"
	N       int
	Data    []byte
}

