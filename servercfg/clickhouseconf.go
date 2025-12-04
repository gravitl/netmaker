package servercfg

import (
	"os"
	"strconv"

	"github.com/gravitl/netmaker/config"
)

func GetClickHouseConfig() config.ClickHouseConfig {
	return config.ClickHouseConfig{
		Host:     GetClickHouseHost(),
		Port:     GetClickHousePort(),
		Database: GetClickHouseDB(),
		Username: GetClickHouseUser(),
		Password: GetClickHousePassword(),
	}
}

func GetClickHouseHost() string {
	host := "localhost"
	if os.Getenv("CLICKHOUSE_HOST") != "" {
		host = os.Getenv("CLICKHOUSE_HOST")
	} else if config.Config.ClickHouse.Host != "" {
		host = config.Config.ClickHouse.Host
	}
	return host
}

func GetClickHousePort() int32 {
	port := int32(9000)
	envport, err := strconv.Atoi(os.Getenv("CLICKHOUSE_PORT"))
	if err == nil && envport != 0 {
		port = int32(envport)
	} else if config.Config.ClickHouse.Port != 0 {
		port = config.Config.ClickHouse.Port
	}
	return port
}

func GetClickHouseDB() string {
	db := "netmaker"
	if os.Getenv("CLICKHOUSE_DB") != "" {
		db = os.Getenv("CLICKHOUSE_DB")
	} else if config.Config.ClickHouse.Database != "" {
		db = config.Config.ClickHouse.Database
	}
	return db
}

func GetClickHouseUser() string {
	user := "netmaker"
	if os.Getenv("CLICKHOUSE_USER") != "" {
		user = os.Getenv("CLICKHOUSE_USER")
	} else if config.Config.ClickHouse.Username != "" {
		user = config.Config.ClickHouse.Username
	}
	return user
}

func GetClickHousePassword() string {
	password := "netmaker"
	if os.Getenv("CLICKHOUSE_PASS") != "" {
		password = os.Getenv("CLICKHOUSE_PASS")
	} else if config.Config.ClickHouse.Password != "" {
		password = config.Config.ClickHouse.Password
	}
	return password
}
