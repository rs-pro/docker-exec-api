package api

import (
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"github.com/lithammer/dedent"
	dea "github.com/rs-pro/docker-exec-api"
	"github.com/rs-pro/docker-exec-api/config"
)

var r *gin.Engine

func init() {
	if config.Config.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
}

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

		if config.Config.StatusPage {
			body := `
					<h3>DockerExecApi Status page</h3>
					{{range $container := .Containers}}
					<h5>{{$container.ID}}</h5>
					{{ range $command := $container.GetCommands }}
					<pre><code>
					> {{$command.Command}}
					{{$command.GetOutput}}
					</code></pre>
					{{end}}
					{{end}}
			`
			body = strings.TrimSpace(dedent.Dedent(body))
			tpl := template.Must(template.New("status").Parse(body))
			r.GET("/status", func(c *gin.Context) {
				c.Status(http.StatusOK)
				c.Header("Content-Type", "text/html; charset=utf-8")
				err := tpl.Execute(c.Writer, gin.H{
					"Containers": pool.GetAllContainers(),
				})
				if err != nil {
					log.Println(err)
				}
			})
		}

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
