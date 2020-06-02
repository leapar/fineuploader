package util

import (
	"fineuploader/server/model"

	"fmt"
	"github.com/gin-gonic/gin"
	"reflect"
)

func GetUser(ctx *gin.Context) *model.Login {
	cached,exist := ctx.Get("user")

	if exist && cached != nil {
		return cached.(*model.Login)
	}

	user := &model.Login{}
	session := GetSession(ctx)

	cacheUser := session.Get("user")
	//fmt.Println("GetUser:%v",&session)
	if cacheUser == nil {
		return user
	}

	cuser,ok := cacheUser.(*model.Login)
	if !ok {
		fmt.Println(reflect.TypeOf(cacheUser))
		return user
	}
	return cuser
}



