package model

import (
	"encoding/json"
	restmodel "fineuploader/restapi/restmodel"
)

type Login struct {
	ID       string
	Username string
	Password string
	IP       string
	DeviceID string
	Auth *restmodel.AuthSuccess
	User *restmodel.UserInfo
}

func (this *Login)GobEncode() ([]byte, error) {
	enc, err := json.Marshal(this)
	if err != nil {
		return nil,err
	}
	return enc,nil
}

func (this *Login)GobDecode(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	err := json.Unmarshal(data,this)
	return err
}

type LoginRes struct {
	Code    int
	Message string
}
