package restapi

import (
	"encoding/json"
	"fineuploader/config"
	"fineuploader/restapi/restmodel"
	"fmt"
	"github.com/go-resty/resty/v2"
	"net/http"
)

type RestApi struct {
	config config.Config
}

func New(config config.Config) *RestApi {
	return &RestApi{
		config: config,
	}
}

func (this *RestApi) Login(username string, password string) (*restmodel.AuthSuccess, error) {
	auth := restmodel.AuthSuccess{}
	client := resty.New()
	_, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetBasicAuth("client", "secret").
		SetFormData(map[string]string{
		"grant_type": "password",
		"username":   username,
		"password":   password,
	}).
		SetResult(&auth). // or SetResult(&AuthSuccess{}).
		Post(fmt.Sprintf("%s/rest/v2/oauth/token", this.config.RestServer))

	return &auth, err
}

func (this *RestApi) GetUserInfo(auth *restmodel.AuthSuccess) (*restmodel.UserInfo, error) {
	client := resty.New()
	resp, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetAuthToken(auth.AccessToken).
		Get(fmt.Sprintf("%s/rest/v2/userInfo", this.config.RestServer))

	if err != nil {
		return nil, err
	}

	userInfo := restmodel.UserInfo{}
	err = json.Unmarshal([]byte(resp.String()), &userInfo)
	if err != nil {
		return nil, err
	}

	return &userInfo, nil
}

func (this *RestApi) SaveBatchInfo(auth *restmodel.AuthSuccess,batch *restmodel.BatchColorboard) ( *restmodel.JMError) {
	client := resty.New()
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetAuthToken(auth.AccessToken).
		SetBody(batch).
		Post(fmt.Sprintf("%s/rest/v2/entities/mpp$ProjectBatch", this.config.RestServer))

	if err != nil {
		fmt.Println(err)
		return &restmodel.JMError{err.Error(),http.StatusInternalServerError}
	}

	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusBadRequest {
		return &restmodel.JMError{"",resp.StatusCode() }
		//return fmt.Errorf("%d",resp.StatusCode())
	}

	entityRes := restmodel.EntityRes{}
	err = json.Unmarshal([]byte(resp.String()), &entityRes)
	if err != nil {
		fmt.Println(err)
		return &restmodel.JMError{err.Error(),http.StatusInternalServerError}
	}

	return nil
}

func (this *RestApi) GetExtFileInfo(auth *restmodel.AuthSuccess,checkSum string ) ( *restmodel.ExtFile, *restmodel.JMError) {
	//http://localhost:8080/mpp/rest/v2/entities/mpp$ProjectBatch/e32b26f7-7b2a-9516-38e5-a1a8852d7a2b?view=projectBatch-view-rest

	condition := restmodel.Condition{Property:"checkSum",Operator:restmodel.OP_equal,Value:checkSum}

	var conditions restmodel.Conditions

	conditions.Conditions = append(conditions.Conditions,condition)
	/*
	{
    "conditions": [
        {
            "group": "OR",
            "conditions": [
                {
                    "property": "stringField",
                    "operator": "startsWith",
                    "value": "ABC"
                },
                {
                    "property": "relatedEntity.intField",
                    "operator": "&gt;",
                    "value": 100
                }
            ]
        },
        {
            "property": "booleanField",
            "operator": "=",
            "value": true
        }
    ]
}
	*/
	filter,err := json.Marshal(conditions)
	if err != nil {
		return  nil,&restmodel.JMError{err.Error(),http.StatusInternalServerError}
	}

	//fmt.Println(string(filter))
	client := resty.New()
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetAuthToken(auth.AccessToken).
		SetQueryParam("filter",string(filter)).
		SetQueryParam("view","extFile-view-rest").
		SetQueryParam("limit","1").
		Get(fmt.Sprintf("%s/rest/v2/entities/mpp$ExtFile/search", this.config.RestServer))

	if err != nil {
		fmt.Println(err)
		return nil,&restmodel.JMError{err.Error(),http.StatusInternalServerError}
	}

	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusBadRequest {
		return nil,&restmodel.JMError{"",resp.StatusCode() }
		//return fmt.Errorf("%d",resp.StatusCode())
	}

	var entityRes  []restmodel.ExtFile
	err = json.Unmarshal([]byte(resp.String()), &entityRes)
	if err != nil {
		fmt.Println(err)
		return nil,&restmodel.JMError{err.Error(),http.StatusInternalServerError}
	}

	if len(entityRes) == 0 {
		fmt.Printf("no mpp$ExtFile %s",checkSum)
		return nil,&restmodel.JMError{fmt.Sprintf("no mpp$ExtFile %s",checkSum),http.StatusInternalServerError}
	}

	return &entityRes[0],nil
}


func (this *RestApi) UpdateExtFileStatus(auth *restmodel.AuthSuccess,extFile *restmodel.ExtFile) ( *restmodel.JMError) {
	client := resty.New()
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetAuthToken(auth.AccessToken).
		SetBody(extFile).
		Post(fmt.Sprintf("%s/rest/v2/entities/mpp$ExtFile", this.config.RestServer))

	if err != nil {
		fmt.Println(err)
		return &restmodel.JMError{err.Error(),http.StatusInternalServerError}
	}

	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusBadRequest {
		return &restmodel.JMError{"",resp.StatusCode() }
		//return fmt.Errorf("%d",resp.StatusCode())
	}

	entityRes := restmodel.EntityRes{}
	err = json.Unmarshal([]byte(resp.String()), &entityRes)
	if err != nil {
		fmt.Println(err)
		return &restmodel.JMError{err.Error(),http.StatusInternalServerError}
	}

	return nil
}

func (this *RestApi) SaveModelPackageInfo(auth *restmodel.AuthSuccess,modelPackage *restmodel.ProjectModel) ( *restmodel.JMError) {
	client := resty.New()
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetAuthToken(auth.AccessToken).
		SetBody(modelPackage).
		Post(fmt.Sprintf("%s/rest/v2/entities/mpp$ProjectModel", this.config.RestServer))

	if err != nil {
		fmt.Println(err)
		return &restmodel.JMError{err.Error(),http.StatusInternalServerError}
	}

	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusBadRequest {
		return &restmodel.JMError{"",resp.StatusCode() }
		//return fmt.Errorf("%d",resp.StatusCode())
	}


	entityRes := restmodel.EntityRes{}
	err = json.Unmarshal([]byte(resp.String()), &entityRes)
	if err != nil {
		fmt.Println(err)
		return &restmodel.JMError{err.Error(),http.StatusInternalServerError}
	}

	return nil
}

func (this *RestApi) SaveTaskPackageInfo(auth *restmodel.AuthSuccess,modelPackage *restmodel.ProjectModelTaskPlan) ( *restmodel.JMError) {
	client := resty.New()
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetAuthToken(auth.AccessToken).
		SetBody(modelPackage).
		Post(fmt.Sprintf("%s/rest/v2/entities/mpp$ProjectModelTaskPlan", this.config.RestServer))

	if err != nil {
		fmt.Println(err)
		return &restmodel.JMError{err.Error(),http.StatusInternalServerError}
	}

	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusBadRequest {
		return &restmodel.JMError{"",resp.StatusCode() }
		//return fmt.Errorf("%d",resp.StatusCode())
	}

	entityRes := restmodel.EntityRes{}
	err = json.Unmarshal([]byte(resp.String()), &entityRes)
	if err != nil {
		fmt.Println(err)
		return &restmodel.JMError{err.Error(),http.StatusInternalServerError}
	}

	return nil
}

