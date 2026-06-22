package database

import (
	"errors"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/servercfg"
)

const (
	// == Table Names ==
	// ACLS_TABLE_NAME - table for acls v2
	ACLS_TABLE_NAME = "acls"
	// SSO_STATE_CACHE - holds sso session information for OAuth2 sign-ins
	SSO_STATE_CACHE = "ssostatecache"
	// CACHE_TABLE_NAME - caching table
	CACHE_TABLE_NAME = "cache"
	// SERVER_SETTINGS - table for server settings
	SERVER_SETTINGS = "server_settings"
	// == ERROR CONSTS ==
	// NO_RECORD - no singular result found
	NO_RECORD = "no result found"
	// NO_RECORDS - no results found
	NO_RECORDS = "could not find any records"

	// == DB Constants ==
	// INIT_DB - initialize db
	INIT_DB = "init"
	// CREATE_TABLE - create table const
	CREATE_TABLE = "createtable"
	// INSERT - insert into db const
	INSERT = "insert"
	// DELETE - delete db record const
	DELETE = "delete"
	// DELETE_ALL - delete a table const
	DELETE_ALL = "deleteall"
	// FETCH_ALL - fetch table contents const
	FETCH_ALL = "fetchall"
	// FETCH_ONE - fetch a single record const
	FETCH_ONE = "fetchone"
	// CLOSE_DB - graceful close of db const
	CLOSE_DB = "closedb"
	// isconnected
	isConnected = "isconnected"
)

var Tables = []string{
	SSO_STATE_CACHE,
	CACHE_TABLE_NAME,
	ACLS_TABLE_NAME,
	SERVER_SETTINGS,
}

func getCurrentDB() map[string]interface{} {
	switch servercfg.GetDB() {
	case "rqlite":
		return RQLITE_FUNCTIONS
	case "sqlite":
		return SQLITE_FUNCTIONS
	case "postgres":
		return PG_FUNCTIONS
	default:
		return SQLITE_FUNCTIONS
	}
}

// InitializeDatabase - initializes database
func InitializeDatabase() error {
	logger.Log(0, "connecting to", servercfg.GetDB())
	tperiod := time.Now().Add(10 * time.Second)
	for {
		if err := getCurrentDB()[INIT_DB].(func() error)(); err != nil {
			logger.Log(0, "unable to connect to db, retrying . . .")
			if time.Now().After(tperiod) {
				return err
			}
		} else {
			break
		}
		time.Sleep(2 * time.Second)
	}
	createTables()
	return nil
}

func createTables() {
	for _, table := range Tables {
		_ = CreateTable(table)
	}
}

func CreateTable(tableName string) error {
	return getCurrentDB()[CREATE_TABLE].(func(string) error)(tableName)
}

// Insert - inserts object into db
func Insert(key string, value string, tableName string) error {
	if key != "" && value != "" {
		return getCurrentDB()[INSERT].(func(string, string, string) error)(key, value, tableName)
	} else {
		return errors.New("invalid insert " + key + " : " + value)
	}
}

// DeleteRecord - deletes a record from db
func DeleteRecord(tableName string, key string) error {
	return getCurrentDB()[DELETE].(func(string, string) error)(tableName, key)
}

// DeleteAllRecords - removes a table and remakes
func DeleteAllRecords(tableName string) error {
	err := getCurrentDB()[DELETE_ALL].(func(string) error)(tableName)
	if err != nil {
		return err
	}
	return CreateTable(tableName)
}

// FetchRecord - fetches a single record by key
func FetchRecord(tableName string, key string) (string, error) {
	return getCurrentDB()[FETCH_ONE].(func(string, string) (string, error))(tableName, key)
}

// FetchRecords - fetches all records in given table
func FetchRecords(tableName string) (map[string]string, error) {
	return getCurrentDB()[FETCH_ALL].(func(string) (map[string]string, error))(tableName)
}

// CloseDB - closes a database gracefully
func CloseDB() {
	getCurrentDB()[CLOSE_DB].(func())()
}

// IsConnected - tell if the database is connected or not
func IsConnected() bool {
	return getCurrentDB()[isConnected].(func() bool)()
}
