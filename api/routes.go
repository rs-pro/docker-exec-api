package api

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	dea "github.com/rs-pro/docker-exec-api"
)

var r *gin.Engine

func GetRouter() *gin.Engine {
	if r == nil {
		//r := gin.Default()
		r = gin.New()

		f, err := os.OpenFile("dea.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Println("WARN unable to open log for writing", err)
		} else {
			gin.DefaultWriter = io.MultiWriter(f)
		}
		r.Use(gin.Logger())
		r.Use(gin.Recovery())

		pool := dea.NewPool()

		r.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{})
			return
		})

		r.GET("/robots.txt", func(c *gin.Context) {
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.Write([]byte("User-agent: *\nDisallow: /"))
		})

		s := r.Group("/sessions")
		s.Use(CheckApiKey())

		s.POST("", func(c *gin.Context) {
			config := dea.ExecParams{}
			err := c.BindJSON(&config)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "error",
					"message": "failed to parse json",
					"error":   err.Error(),
				})
				return
			}
			container, err := pool.Exec(&config)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "error",
					"message": "failed to execute container",
					"error":   err.Error(),
				})
				return
			}

			c.JSON(http.StatusOK, container)
		})
		s.GET("", func(c *gin.Context) {
			c.JSON(http.StatusOK, pool.GetAllContainers())
		})

		r.GET("/sessions/:id/ws", func(c *gin.Context) {
			id := c.Param("id")
			container := pool.GetContainerByToken(id)
			if container == nil {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "session with id " + id + " not found.",
				})
				return
			}

			ConnectToContainer(c, container)
		})

		r.GET("/sessions/:id/output", func(c *gin.Context) {
			id := c.Param("id")
			container := pool.GetContainerByToken(id)
			if container == nil {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "session with id " + id + " not found.",
				})
				return
			}

			c.JSON(http.StatusOK, container.GetCommands())
		})

		r.GET("/health", func(c *gin.Context) {
			_, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "error",
					"message": "failed to connect to docker daemon",
					"error":   err.Error(),
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{"status": "ok"})
			return
		})
	}

	return r
}
