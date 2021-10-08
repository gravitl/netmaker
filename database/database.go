package database

import (
	"encoding/json"
	"errors"
	"github.com/gravitl/netmaker/servercfg"
	"log"
	"time"
)

const NETWORKS_TABLE_NAME = "networks"
const NODES_TABLE_NAME = "nodes"
const DELETED_NODES_TABLE_NAME = "deletednodes"
const USERS_TABLE_NAME = "users"
const DNS_TABLE_NAME = "dns"
const EXT_CLIENT_TABLE_NAME = "extclients"
const INT_CLIENTS_TABLE_NAME = "intclients"
const PEERS_TABLE_NAME = "peers"
const DATABASE_FILENAME = "netmaker.db"

// == ERROR CONSTS ==
const NO_RECORD = "no result found"
const NO_RECORDS = "could not find any records"

// == Constants ==
const INIT_DB = "init"
const CREATE_TABLE = "createtable"
const INSERT = "insert"
const INSERT_PEER = "insertpeer"
const DELETE = "delete"
const DELETE_ALL = "deleteall"
const FETCH_ALL = "fetchall"
const CLOSE_DB = "closedb"

func getCurrentDB() map[string]interface{} {
	switch servercfg.GetDB() {
	case "rqlite":
		return RQLITE_FUNCTIONS
	case "sqlite":
		return SQLITE_FUNCTIONS
	default:
		return SQLITE_FUNCTIONS
	}
}

func InitializeDatabase() error {
	log.Println("connecting to", servercfg.GetDB())
	tperiod := time.Now().Add(10 * time.Second)
	for {
		if err := getCurrentDB()[INIT_DB].(func() error)(); err != nil {
			log.Println("unable to connect to db, retrying . . .")
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
	createTable(NETWORKS_TABLE_NAME)
	createTable(NODES_TABLE_NAME)
	createTable(DELETED_NODES_TABLE_NAME)
	createTable(USERS_TABLE_NAME)
	createTable(DNS_TABLE_NAME)
	createTable(EXT_CLIENT_TABLE_NAME)
	createTable(INT_CLIENTS_TABLE_NAME)
	createTable(PEERS_TABLE_NAME)
}

func createTable(tableName string) error {
	return getCurrentDB()[CREATE_TABLE].(func(string) error)(tableName)
}

func IsJSONString(value string) bool {
	var jsonInt interface{}
	return json.Unmarshal([]byte(value), &jsonInt) == nil
}

func Insert(key string, value string, tableName string) error {
	if key != "" && value != "" && IsJSONString(value) {
		return getCurrentDB()[INSERT].(func(string, string, string) error)(key, value, tableName)
	} else {
		return errors.New("invalid insert " + key + " : " + value)
	}
}

func InsertPeer(key string, value string) error {
	if key != "" && value != "" && IsJSONString(value) {
		return getCurrentDB()[INSERT_PEER].(func(string, string) error)(key, value)
	} else {
		return errors.New("invalid peer insert " + key + " : " + value)
	}
}

func DeleteRecord(tableName string, key string) error {
	return getCurrentDB()[DELETE].(func(string, string) error)(tableName, key)
}

func DeleteAllRecords(tableName string) error {
	err := getCurrentDB()[DELETE_ALL].(func(string) error)(tableName)
	if err != nil {
		return err
	}
	err = createTable(tableName)
	if err != nil {
		return err
	}
	return nil
}

func FetchRecord(tableName string, key string) (string, error) {
	results, err := FetchRecords(tableName)
	if err != nil {
		return "", err
	}
	if results[key] == "" {
		return "", errors.New(NO_RECORD)
	}
	return results[key], nil
}

func FetchRecords(tableName string) (map[string]string, error) {
	return getCurrentDB()[FETCH_ALL].(func(string) (map[string]string, error))(tableName)
}

func CloseDB() {
	getCurrentDB()[CLOSE_DB].(func())()
}
