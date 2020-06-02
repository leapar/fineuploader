package middlewares

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

var SessionStore gin.HandlerFunc

func init() {
	store := cookie.NewStore([]byte("secret"))


	SessionStore = sessions.Sessions("jmyd-sid", store)
	//sessions.Sessions("jmyd-file-server-sid", store)
}


