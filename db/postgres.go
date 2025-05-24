package db

import (
	"fmt"
	"os"
	"strconv"

	"github.com/gravitl/netmaker/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// postgresConnector for initializing and
// connecting to a postgres database.
type postgresConnector struct{}

// postgresConnector.connect connects and
// initializes a connection to postgres.
func (pg *postgresConnector) connect() (*gorm.DB, error) {
	pgConf := GetSQLConf()
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
		pgConf.Host,
		pgConf.Port,
		pgConf.Username,
		pgConf.Password,
		pgConf.DB,
		pgConf.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	// ensure netmaker_v1 schema exists.
	err = db.Exec("CREATE SCHEMA IF NOT EXISTS netmaker_v1").Error
	if err != nil {
		return nil, err
	}

	// set the netmaker_v1 schema as the default schema.
	err = db.Exec("SET search_path TO netmaker_v1").Error
	if err != nil {
		return nil, err
	}

	return db, nil
}
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
