package functions

import (
	"net/http"

	cfg "github.com/gravitl/netmaker/config"
	"github.com/gravitl/netmaker/models"
)

// GetLogs - fetch Netmaker server logs
func GetLogs() string {
	return get("/api/logs")
}

// GetServerInfo - fetch minimal server info
func GetServerInfo() *models.ServerConfig {
	return request[models.ServerConfig](http.MethodGet, "/api/server/getserverinfo", nil)
}

// GetServerConfig - fetch entire server config including secrets
func GetServerConfig() *cfg.ServerConfig {
	return request[cfg.ServerConfig](http.MethodGet, "/api/server/getconfig", nil)
}

// GetServerHealth - fetch server current health status
func GetServerHealth() string {
	return get("/api/server/health")
}
