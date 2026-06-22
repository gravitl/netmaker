package database

import (
	"errors"
)

const (
	// == Table Names ==
	// DNS_TABLE_NAME - dns table
	DNS_TABLE_NAME = "dns"
	// EXT_CLIENT_TABLE_NAME - ext client table
	EXT_CLIENT_TABLE_NAME = "extclients"
	// ACLS_TABLE_NAME - table for acls v2
	ACLS_TABLE_NAME = "acls"
	// SSO_STATE_CACHE - holds sso session information for OAuth2 sign-ins
	SSO_STATE_CACHE = "ssostatecache"
	// METRICS_TABLE_NAME - stores network metrics
	METRICS_TABLE_NAME = "metrics"
	// CACHE_TABLE_NAME - caching table
	CACHE_TABLE_NAME = "cache"
	// TAG_TABLE_NAME - table for tags
	TAG_TABLE_NAME = "tags"
	// SERVER_SETTINGS - table for server settings
	SERVER_SETTINGS = "server_settings"

	// == DB Constants ==
	// INSERT - insert into db const
	INSERT = "insert"
	// DELETE - delete db record const
	DELETE = "delete"
	// FETCH_ALL - fetch table contents const
	FETCH_ALL = "fetchall"
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

// DeleteRecord - deletes a record from db
func DeleteRecord(tableName string, key string) error {
	return getCurrentDB()[DELETE].(func(string, string) error)(tableName, key)
}

// FetchRecord - fetches a single record by key
func FetchRecord(tableName string, key string) (string, error) {
	return getCurrentDB()[FETCH_ONE].(func(string, string) (string, error))(tableName, key)
}

// FetchRecords - fetches all records in given table
func FetchRecords(tableName string) (map[string]string, error) {
	return getCurrentDB()[FETCH_ALL].(func(string) (map[string]string, error))(tableName)
}
