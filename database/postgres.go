package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/gravitl/netmaker/db"
	_ "github.com/lib/pq"
)

// PGDB - database object for PostGreSQL
var PGDB *sql.DB

// PG_FUNCTIONS - map of db functions for PostGreSQL
var PG_FUNCTIONS = map[string]interface{}{
	INIT_DB:      initPGDB,
	CREATE_TABLE: pgCreateTable,
	INSERT:       pgInsert,
	INSERT_PEER:  pgInsertPeer,
	DELETE:       pgDeleteRecord,
	DELETE_ALL:   pgDeleteAllRecords,
	FETCH_ALL:    pgFetchRecords,
	CLOSE_DB:     pgCloseDB,
	isConnected:  pgIsConnected,
}

func initPGDB() error {
	gormDB := db.FromContext(db.WithContext(context.TODO()))

	var dbOpenErr error
	PGDB, dbOpenErr = gormDB.DB()
	if dbOpenErr != nil {
		return dbOpenErr
	}

	return PGDB.Ping()
}

func pgCreateTable(tableName string) error {
	statement, err := PGDB.Prepare("CREATE TABLE IF NOT EXISTS " + tableName + " (key TEXT NOT NULL UNIQUE PRIMARY KEY, value TEXT)")
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

func pgInsert(key string, value string, tableName string) error {
	if key != "" && value != "" {
		insertSQL := "INSERT INTO " + tableName + " (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = $3;"
		statement, err := PGDB.Prepare(insertSQL)
		if err != nil {
			return err
		}
		defer statement.Close()
		_, err = statement.Exec(key, value, value)
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("invalid insert " + key + " : " + value)
	}
}

func pgInsertPeer(key string, value string) error {
	if key != "" && value != "" {
		err := pgInsert(key, value, PEERS_TABLE_NAME)
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("invalid peer insert " + key + " : " + value)
	}
}

func pgDeleteRecord(tableName string, key string) error {
	deleteSQL := "DELETE FROM " + tableName + " WHERE key = $1;"
	statement, err := PGDB.Prepare(deleteSQL)
	if err != nil {
		return err
	}
	defer statement.Close()
	if _, err = statement.Exec(key); err != nil {
		return err
	}
	return nil
}

func pgDeleteAllRecords(tableName string) error {
	deleteSQL := "DELETE FROM " + tableName
	statement, err := PGDB.Prepare(deleteSQL)
	if err != nil {
		return err
	}
	defer statement.Close()
	if _, err = statement.Exec(); err != nil {
		return err
	}
	return nil
}

func pgFetchRecords(tableName string) (map[string]string, error) {
	row, err := PGDB.Query("SELECT * FROM " + tableName + " ORDER BY key")
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

func pgCloseDB() {
	//PGDB.Close()
}

func pgIsConnected() bool {
	stats := PGDB.Stats()
	return stats.OpenConnections > 0
}
