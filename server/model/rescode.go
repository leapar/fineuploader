package model

type ResponCode int

const (
	UNKNOW_ERROR = -999 //未知错误
	LOGIN_ERROR = -5//2 登录失败//1+iota
	PARAM_ERROR = -4//参数错误
	UNAUTHORIZED = -3//未登录
	NETWORK_ERROR = -2//网络故障
	ERROR = -1//一般错误
	OK = 0//3 正常
)

type NormalRes struct {
	Code ResponCode
	Message string
	Data interface{}
}

type UploadResponse struct {
	Success      bool   `json:"success"`
	Error        string `json:"error,omitempty"`
	PreventRetry bool   `json:"preventRetry"`
}

