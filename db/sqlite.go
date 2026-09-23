package db

import (
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

	dbFilePath := filepath.Join("data", "netmaker.db")

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

	// WAL + immediate lock + longer busy wait reduce lock storms under scale.
	// Keep MaxOpenConns(1): concurrent writers with go-sqlite3 still deadlock easily.
	dsn := dbFilePath + "?_journal_mode=WAL&_busy_timeout=30000&_txlock=immediate&_synchronous=NORMAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: newGormLogger(),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	return db, nil
}
