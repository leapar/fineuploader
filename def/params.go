package def

const (
	DATA_BASE = "fileserver"
	PREFIX = "uploader"
)

// Chunked request parameters
const (
	ParamUuid = "qquuid" // uuid
	ParamFile = "qqfile" // file name

	ParamPartIndex       = "qqpartindex"      // part index
	ParamPartBytesOffset = "qqpartbyteoffset" // part byte offset
	ParamTotalFileSize   = "qqtotalfilesize"  // total file size
	ParamTotalParts      = "qqtotalparts"     // total parts
	ParamFileName        = "qqfilename"       // file name for chunked requests
	ParamChunkSize       = "qqchunksize"      // size of the chunks
)
