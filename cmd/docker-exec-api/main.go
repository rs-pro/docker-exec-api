package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/rs-pro/docker-exec-api/api"
	"github.com/rs-pro/docker-exec-api/config"
)

func main() {
	listen := config.Config.Listen
	if listen == "" {
		listen = "127.0.0.1:12010"
	}
	r := api.GetRouter()
	fmt.Fprintf(gin.DefaultWriter, "[dea] starting HTTP api at %s\n", listen)
	log.Fatal(r.Run(listen))
}
