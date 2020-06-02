package restmodel


type ExtFileType string

const(
	EXT_FILE_TYPE_HADOOP = "HADOOOP"
	EXT_FILE_TYPE_MONGILEFS = "MONGILEFS"
)

type ExtFileStatus string

const(
	EXT_FILE_STATUS_IDEL = "IDEL"
	EXT_FILE_STATUS_READY = "READY"
	EXT_FILE_STATUS_DOING = "DOING"
	EXT_FILE_STATUS_DONE = "DONE"
)

type ExtFile struct {
	ID string `json:"id,omitempty"`
	CheckSum string `json:"checkSum,omitempty"`
	FileName string `json:"name,omitempty"`
	Size uint64 `json:"size,omitempty"`
	Type ExtFileType `json:"type,omitempty"`
	Status ExtFileStatus `json:"status,omitempty"`
}
