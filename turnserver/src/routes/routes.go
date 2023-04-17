package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gravitl/netmaker/turnserver/config"
	"github.com/gravitl/netmaker/turnserver/internal/host"
)

func Init(r *gin.Engine) *gin.Engine {
	api := r.Group("/api")
	v1 := api.Group("/v1")
	registerRoutes(v1)
	return r
}

func registerRoutes(r *gin.RouterGroup) {
	r.POST("/host/register", gin.BasicAuth(gin.Accounts{
		config.GetUserName(): config.GetPassword(),
	}), host.Register)
	r.DELETE("/host/deregister", gin.BasicAuth(gin.Accounts{
		config.GetUserName(): config.GetPassword(),
	}), host.Remove)
	r.GET("/status", gin.BasicAuth(gin.Accounts{
		config.GetUserName(): config.GetPassword(),
	}), status)
}

func status(c *gin.Context) {
	c.JSON(http.StatusOK, struct {
		Msg string `json:"msg"`
	}{Msg: "hello"})
}
