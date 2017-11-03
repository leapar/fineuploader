package def

import (
	"github.com/rcrowley/go-metrics"
)

type Outputer interface {
	WriteChunks(cookie string,index int,datas []byte,uuid string,chunkSize int,totalSize int,filename string) string
}

var (
	ReqUploadTime = metrics.NewRegisteredGaugeFloat64("upload.request.time", metrics.DefaultRegistry)

	UploadICPTime = metrics.NewRegisteredGaugeFloat64("upload.io.copy.time", metrics.DefaultRegistry)

	ReqUploadCount = metrics.NewRegisteredCounter("upload.request", metrics.DefaultRegistry)
	ReqUploadOkCount = metrics.NewRegisteredCounter("upload.request.ok", metrics.DefaultRegistry)
	ReqUploadErrCount = metrics.NewRegisteredCounter("upload.request.error", metrics.DefaultRegistry)
	ReqUploadParamErrCount = metrics.NewRegisteredCounter("upload.request.params.error", metrics.DefaultRegistry)

	WriteChunkTime = metrics.NewRegisteredGaugeFloat64("upload.write.chunk.time", metrics.DefaultRegistry)

	DBErrCount = metrics.NewRegisteredCounter("db.error", metrics.DefaultRegistry)

	ReqUploadDoneCount = metrics.NewRegisteredCounter("uploaddone.request", metrics.DefaultRegistry)
	ReqUploadDoneOkCount = metrics.NewRegisteredCounter("uploaddone.request.ok", metrics.DefaultRegistry)
	ReqUploadDoneErrCount = metrics.NewRegisteredCounter("uploaddone.request.error", metrics.DefaultRegistry)
	ReqUploadDoneParamErrCount = metrics.NewRegisteredCounter("uploaddone.request.params.error", metrics.DefaultRegistry)
)