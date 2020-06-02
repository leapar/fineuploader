package util

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/sessions"
)

func GetSession(ctx *gin.Context) sessions.Session {
	return sessions.Default(ctx)
}

