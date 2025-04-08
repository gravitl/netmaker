package db

import (
	"fmt"
	"github.com/gravitl/netmaker/servercfg"
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
	pgConf := servercfg.GetSQLConf()
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s search_path=netmaker_v1 connect_timeout=5",
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

	return db, nil
}
