package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

	// ensure netmaker.db exists.
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

	// By default, SQLite transactions may keep writes invisible to other readers
	// until the transaction commits and the connection closes. This is especially
	// noticeable when using multiple connections from a pool.
	//
	// We configure the DSN to improve visibility and concurrency:
	//
	// 1. _journal_mode=wal: Use Write-Ahead Logging so committed writes are visible
	// to other connections immediately, and readers don't block writers. The default
	// journal mode is DELETE, which is not suitable for concurrent access.
	//
	// 2. _txlock=immediate: Acquire a RESERVED write lock as soon as the transaction
	// begins, avoiding mid-transaction "database is locked" errors. The default is
	// deferred, which may cause a transaction to rollback if another transaction has
	// already acquired a RESERVED lock.
	//
	// 3. _busy_timeout=5000: Wait up to 5 seconds for a locked database before failing.
	//
	// See discussion: https://github.com/mattn/go-sqlite3/issues/1022#issuecomment-1067353980

	dsn := "file:" + dbFilePath + "?_journal_mode=wal&_txlock=immediate&_busy_timeout=5000"

	sqliteDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	sqliteDB.SetMaxOpenConns(1)
	sqliteDB.SetConnMaxLifetime(time.Hour)
	sqliteDB.SetMaxIdleConns(1)

	return gorm.Open(sqlite.Dialector{Conn: sqliteDB}, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}
