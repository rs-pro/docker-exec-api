package api

import (
	"github.com/gin-gonic/gin"
	"github.com/rs-pro/docker-exec-api/config"
)

func init() {
	if config.Config.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
}
