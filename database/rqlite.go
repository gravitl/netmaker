package database

import (
	"errors"

	"github.com/gravitl/netmaker/servercfg"
	"github.com/rqlite/gorqlite"
)

// RQliteDatabase - the rqlite db connection
var RQliteDatabase gorqlite.Connection

// RQLITE_FUNCTIONS - all the functions to run with rqlite
var RQLITE_FUNCTIONS = map[string]interface{}{
	INIT_DB:      initRqliteDatabase,
	CREATE_TABLE: rqliteCreateTable,
	INSERT:       rqliteInsert,
	INSERT_PEER:  rqliteInsertPeer,
	DELETE:       rqliteDeleteRecord,
	DELETE_ALL:   rqliteDeleteAllRecords,
	FETCH_ALL:    rqliteFetchRecords,
	CLOSE_DB:     rqliteCloseDB,
}

func initRqliteDatabase() error {

	conn, err := gorqlite.Open(servercfg.GetSQLConn())
	if err != nil {
		return err
	}
	RQliteDatabase = conn
	RQliteDatabase.SetConsistencyLevel("strong")
	return nil
}

func rqliteCreateTable(tableName string) error {
	_, err := RQliteDatabase.WriteOne("CREATE TABLE IF NOT EXISTS " + tableName + " (key TEXT NOT NULL UNIQUE PRIMARY KEY, value TEXT)")
	if err != nil {
		return err
	}
	return nil
}

func rqliteInsert(key string, value string, tableName string) error {
	if key != "" && value != "" && IsJSONString(value) {
		_, err := RQliteDatabase.WriteOne("INSERT OR REPLACE INTO " + tableName + " (key, value) VALUES ('" + key + "', '" + value + "')")
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("invalid insert " + key + " : " + value)
}

func rqliteInsertPeer(key string, value string) error {
	if key != "" && value != "" && IsJSONString(value) {
		_, err := RQliteDatabase.WriteOne("INSERT OR REPLACE INTO " + PEERS_TABLE_NAME + " (key, value) VALUES ('" + key + "', '" + value + "')")
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("invalid peer insert " + key + " : " + value)
}

func rqliteDeleteRecord(tableName string, key string) error {
	_, err := RQliteDatabase.WriteOne("DELETE FROM " + tableName + " WHERE key = \"" + key + "\"")
	if err != nil {
		return err
	}
	return nil
}

func rqliteDeleteAllRecords(tableName string) error {
	_, err := RQliteDatabase.WriteOne("DELETE TABLE " + tableName)
	if err != nil {
		return err
	}
	err = rqliteCreateTable(tableName)
	if err != nil {
		return err
	}
	return nil
}

func rqliteFetchRecords(tableName string) (map[string]string, error) {
	row, err := RQliteDatabase.QueryOne("SELECT * FROM " + tableName + " ORDER BY key")
	if err != nil {
		return nil, err
	}
	records := make(map[string]string)
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

func rqliteCloseDB() {
	RQliteDatabase.Close()
}
