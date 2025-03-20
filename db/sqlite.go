package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"path/filepath"
)

// sqliteConnector for initializing and
// connecting to a sqlite database.
type sqliteConnector struct{}

// sqliteConnector.connect connects and
// initializes a connection to sqlite.
func (s *sqliteConnector) connect() (*gorm.DB, error) {
	// ensure data dir exists.
	_, err := os.Stat("data")
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir("data", 0700)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	dbFilePath := filepath.Join("data", "netmaker_v1.db")

	// ensure netmaker_v1.db exists.
	_, err = os.Stat(dbFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			file, err := os.Create(dbFilePath)
			if err != nil {
				return nil, err
			}

			err = file.Close()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return gorm.Open(sqlite.Open(dbFilePath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}
