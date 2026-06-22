package database

import (
	"errors"
)

const (
	// == Table Names ==
	// SERVER_SETTINGS - table for server settings
	SERVER_SETTINGS = "server_settings"

	// == DB Constants ==
	// INSERT - insert into db const
	INSERT = "insert"
	// FETCH_ONE - fetch a single record const
	FETCH_ONE = "fetchone"
)

func getCurrentDB() map[string]interface{} {
	return map[string]interface{}{}
}

// Insert - inserts object into db
func Insert(key string, value string, tableName string) error {
	if key != "" && value != "" {
		return getCurrentDB()[INSERT].(func(string, string, string) error)(key, value, tableName)
	} else {
		return errors.New("invalid insert " + key + " : " + value)
	}
}

// FetchRecord - fetches a single record by key
func FetchRecord(tableName string, key string) (string, error) {
	return getCurrentDB()[FETCH_ONE].(func(string, string) (string, error))(tableName, key)
}
