package functions

import (
	"io"
	"log"
	"net/http"

	"github.com/gravitl/netmaker/cli/config"
	cfg "github.com/gravitl/netmaker/config"
	"github.com/gravitl/netmaker/models"
)

func GetLogs() string {
	ctx := config.GetCurrentContext()
	req, err := http.NewRequest(http.MethodGet, ctx.Endpoint+"/api/logs", nil)
	if err != nil {
		log.Fatal(err)
	}
	if ctx.MasterKey != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.MasterKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+getAuthToken(ctx))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	return string(bodyBytes)
}

func GetServerInfo() *models.ServerConfig {
	return request[models.ServerConfig](http.MethodGet, "/api/server/getserverinfo", nil)
}

func GetServerConfig() *cfg.ServerConfig {
	return request[cfg.ServerConfig](http.MethodGet, "/api/server/getconfig", nil)
}

func GetServerHealth() string {
	ctx := config.GetCurrentContext()
	req, err := http.NewRequest(http.MethodGet, ctx.Endpoint+"/api/server/health", nil)
	if err != nil {
		log.Fatal(err)
	}
	if ctx.MasterKey != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.MasterKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+getAuthToken(ctx))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	return string(bodyBytes)
}
