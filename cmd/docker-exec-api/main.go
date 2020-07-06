package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/rs-pro/docker-exec-api/api"
)

func main() {
	listen := os.Getenv("LISTEN")
	if listen == "" {
		listen = "127.0.0.1:12010"
	}
	r := api.GetRouter()
	fmt.Fprintf(gin.DefaultWriter, "[dea] starting HTTP api at %s\n", listen)
	log.Fatal(r.Run(listen))
}
