package database

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3" // need to blank import this package
)

// == sqlite ==
const dbFilename = "netmaker.db"

// SqliteDB is the db object for sqlite database connections
var SqliteDB *sql.DB

// SQLITE_FUNCTIONS - contains a map of the functions for sqlite
var SQLITE_FUNCTIONS = map[string]interface{}{
	INIT_DB:      initSqliteDB,
	CREATE_TABLE: sqliteCreateTable,
	INSERT:       sqliteInsert,
	INSERT_PEER:  sqliteInsertPeer,
	DELETE:       sqliteDeleteRecord,
	DELETE_ALL:   sqliteDeleteAllRecords,
	FETCH_ALL:    sqliteFetchRecords,
	CLOSE_DB:     sqliteCloseDB,
}

func initSqliteDB() error {
	// == create db file if not present ==
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		os.Mkdir("data", 0700)
	}
	dbFilePath := filepath.Join("data", dbFilename)
	if _, err := os.Stat(dbFilePath); os.IsNotExist(err) {
		os.Create(dbFilePath)
	}
	// == "connect" the database ==
	var dbOpenErr error
	SqliteDB, dbOpenErr = sql.Open("sqlite3", dbFilePath)
	if dbOpenErr != nil {
		return dbOpenErr
	}
	SqliteDB.SetMaxOpenConns(1)
	return nil
}

func sqliteCreateTable(tableName string) error {
	statement, err := SqliteDB.Prepare("CREATE TABLE IF NOT EXISTS " + tableName + " (key TEXT NOT NULL UNIQUE PRIMARY KEY, value TEXT)")
	if err != nil {
		return err
	}
	defer statement.Close()
	_, err = statement.Exec()
	if err != nil {
		return err
	}
	return nil
}

func sqliteInsert(key string, value string, tableName string) error {
	if key != "" && value != "" && IsJSONString(value) {
		insertSQL := "INSERT OR REPLACE INTO " + tableName + " (key, value) VALUES (?, ?)"
		statement, err := SqliteDB.Prepare(insertSQL)
		if err != nil {
			return err
		}
		defer statement.Close()
		_, err = statement.Exec(key, value)
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("invalid insert " + key + " : " + value)
}

func sqliteInsertPeer(key string, value string) error {
	if key != "" && value != "" && IsJSONString(value) {
		err := sqliteInsert(key, value, PEERS_TABLE_NAME)
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("invalid peer insert " + key + " : " + value)
}

func sqliteDeleteRecord(tableName string, key string) error {
	deleteSQL := "DELETE FROM " + tableName + " WHERE key = \"" + key + "\""
	statement, err := SqliteDB.Prepare(deleteSQL)
	if err != nil {
		return err
	}
	defer statement.Close()
	if _, err = statement.Exec(); err != nil {
		return err
	}
	return nil
}

func sqliteDeleteAllRecords(tableName string) error {
	deleteSQL := "DELETE FROM " + tableName
	statement, err := SqliteDB.Prepare(deleteSQL)
	if err != nil {
		return err
	}
	defer statement.Close()
	if _, err = statement.Exec(); err != nil {
		return err
	}
	return nil
}

func sqliteFetchRecords(tableName string) (map[string]string, error) {
	row, err := SqliteDB.Query("SELECT * FROM " + tableName + " ORDER BY key")
	if err != nil {
		return nil, err
	}
	records := make(map[string]string)
	defer row.Close()
	for row.Next() { // Iterate and fetch the records from result cursor
		var key string
		var value string
		row.Scan(&key, &value)
		records[key] = value
	}
	if len(records) == 0 {
		return nil, errors.New(NO_RECORDS)
	}
	return records, nil
}

func sqliteCloseDB() {
	SqliteDB.Close()
}
