package servercfg

import (
	"github.com/gravitl/netmaker/config"
	"os"
	"strconv"
)

func GetSQLConf() config.SQLConfig {
	var cfg config.SQLConfig
	cfg.Host = GetSQLHost()
	cfg.Port = GetSQLPort()
	cfg.Username = GetSQLUser()
	cfg.Password = GetSQLPass()
	cfg.DB = GetSQLDB()
	cfg.SSLMode = GetSQLSSLMode()
	return cfg
}
func GetSQLHost() string {
	host := "localhost"
	if os.Getenv("SQL_HOST") != "" {
		host = os.Getenv("SQL_HOST")
	} else if config.Config.SQL.Host != "" {
		host = config.Config.SQL.Host
	}
	return host
}
func GetSQLPort() int32 {
	port := int32(5432)
	envport, err := strconv.Atoi(os.Getenv("SQL_PORT"))
	if err == nil && envport != 0 {
		port = int32(envport)
	} else if config.Config.SQL.Port != 0 {
		port = config.Config.SQL.Port
	}
	return port
}
func GetSQLUser() string {
	user := "postgres"
	if os.Getenv("SQL_USER") != "" {
		user = os.Getenv("SQL_USER")
	} else if config.Config.SQL.Username != "" {
		user = config.Config.SQL.Username
	}
	return user
}
func GetSQLPass() string {
	pass := "nopass"
	if os.Getenv("SQL_PASS") != "" {
		pass = os.Getenv("SQL_PASS")
	} else if config.Config.SQL.Password != "" {
		pass = config.Config.SQL.Password
	}
	return pass
}
func GetSQLDB() string {
	db := "netmaker"
	if os.Getenv("SQL_DB") != "" {
		db = os.Getenv("SQL_DB")
	} else if config.Config.SQL.DB != "" {
		db = config.Config.SQL.DB
	}
	return db
}
func GetSQLSSLMode() string {
	sslmode := "disable"
	if os.Getenv("SQL_SSL_MODE") != "" {
		sslmode = os.Getenv("SQL_SSL_MODE")
	} else if config.Config.SQL.SSLMode != "" {
		sslmode = config.Config.SQL.SSLMode
	}
	return sslmode
}
