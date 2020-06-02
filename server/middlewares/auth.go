package middlewares

import (
	"fineuploader/server/model"
	"fineuploader/server/util"
	"github.com/gin-gonic/gin"
	"net/http"
)

func Authentication(ctx *gin.Context) {
	user := util.GetUser(ctx)
	ctx.Set("user", user)
	ctx.Next()
}

func RedirectIfAuthenticated(ctx *gin.Context) {
	user := util.GetUser(ctx)

	if user.Auth.AccessToken == "" {
		ctx.Redirect(http.StatusMovedPermanently,"/")
		return
	}

	ctx.Next()
}

func RequireAuthentication(ctx *gin.Context) {
	user := util.GetUser(ctx)

	if user.ID == "" || user.Auth == nil || user.Auth.AccessToken == "" {
		//ctx.Redirect("/login")
		r := model.NormalRes{
			Code:model.UNAUTHORIZED,
			Message:"未登录",
		}
		ctx.AbortWithStatusJSON(http.StatusUnauthorized,r)

		return
	}

	ctx.Set("user",user)

	ctx.Next()
}

