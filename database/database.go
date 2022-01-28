package database

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// NETWORKS_TABLE_NAME - networks table
const NETWORKS_TABLE_NAME = "networks"

// NODES_TABLE_NAME - nodes table
const NODES_TABLE_NAME = "nodes"

// DELETED_NODES_TABLE_NAME - deleted nodes table
const DELETED_NODES_TABLE_NAME = "deletednodes"

// USERS_TABLE_NAME - users table
const USERS_TABLE_NAME = "users"

// DNS_TABLE_NAME - dns table
const DNS_TABLE_NAME = "dns"

// EXT_CLIENT_TABLE_NAME - ext client table
const EXT_CLIENT_TABLE_NAME = "extclients"

// INT_CLIENTS_TABLE_NAME - int client table
const INT_CLIENTS_TABLE_NAME = "intclients"

// PEERS_TABLE_NAME - peers table
const PEERS_TABLE_NAME = "peers"

// SERVERCONF_TABLE_NAME - stores server conf
const SERVERCONF_TABLE_NAME = "serverconf"

// SERVER_UUID_TABLE_NAME - stores
const SERVER_UUID_TABLE_NAME = "serveruuid"

// TRAFFIC_TABLE_NAME - stores stuff to control traffic
const TRAFFIC_TABLE_NAME = "traffic-table"

// SERVER_UUID_RECORD_KEY - telemetry thing
const SERVER_UUID_RECORD_KEY = "serveruuid"

// DATABASE_FILENAME - database file name
const DATABASE_FILENAME = "netmaker.db"

// GENERATED_TABLE_NAME - stores server generated k/v
const GENERATED_TABLE_NAME = "generated"

// == ERROR CONSTS ==

// NO_RECORD - no singular result found
const NO_RECORD = "no result found"

// NO_RECORDS - no results found
const NO_RECORDS = "could not find any records"

// == Constants ==

// INIT_DB - initialize db
const INIT_DB = "init"

// CREATE_TABLE - create table const
const CREATE_TABLE = "createtable"

// INSERT - insert into db const
const INSERT = "insert"

// INSERT_PEER - insert peer into db const
const INSERT_PEER = "insertpeer"

// DELETE - delete db record const
const DELETE = "delete"

// DELETE_ALL - delete a table const
const DELETE_ALL = "deleteall"

// FETCH_ALL - fetch table contents const
const FETCH_ALL = "fetchall"

// CLOSE_DB - graceful close of db const
const CLOSE_DB = "closedb"

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
	err := initializeUUID()
	return err
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
	createTable(SERVERCONF_TABLE_NAME)
	createTable(SERVER_UUID_TABLE_NAME)
	createTable(GENERATED_TABLE_NAME)
	createTable(TRAFFIC_TABLE_NAME)
}

func createTable(tableName string) error {
	return getCurrentDB()[CREATE_TABLE].(func(string) error)(tableName)
}

// IsJSONString - checks if valid json
func IsJSONString(value string) bool {
	var jsonInt interface{}
	return json.Unmarshal([]byte(value), &jsonInt) == nil
}

// Insert - inserts object into db
func Insert(key string, value string, tableName string) error {
	if key != "" && value != "" && IsJSONString(value) {
		return getCurrentDB()[INSERT].(func(string, string, string) error)(key, value, tableName)
	} else {
		return errors.New("invalid insert " + key + " : " + value)
	}
}

// InsertPeer - inserts peer into db
func InsertPeer(key string, value string) error {
	if key != "" && value != "" && IsJSONString(value) {
		return getCurrentDB()[INSERT_PEER].(func(string, string) error)(key, value)
	} else {
		return errors.New("invalid peer insert " + key + " : " + value)
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
	err = createTable(tableName)
	if err != nil {
		return err
	}
	return nil
}

// FetchRecord - fetches a record
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

// FetchRecords - fetches all records in given table
func FetchRecords(tableName string) (map[string]string, error) {
	return getCurrentDB()[FETCH_ALL].(func(string) (map[string]string, error))(tableName)
}

// initializeUUID - create a UUID record for server if none exists
func initializeUUID() error {
	records, err := FetchRecords(SERVER_UUID_TABLE_NAME)
	if err != nil {
		if !strings.Contains("could not find any records", err.Error()) {
			return err
		}
	} else if len(records) > 0 {
		return nil
	}
	var rsaPrivKey, keyErr = rsa.GenerateKey(rand.Reader, 32)
	if keyErr != nil {
		return keyErr
	}

	fmt.Printf("key generated: %v \n", rsaPrivKey)
	fmt.Printf("pub key generate: %v \n", rsaPrivKey.PublicKey)

	telemetry := models.Telemetry{UUID: uuid.NewString(), TrafficKey: *rsaPrivKey}
	telJSON, err := json.Marshal(&telemetry)
	if err != nil {
		return err
	}

	return Insert(SERVER_UUID_RECORD_KEY, string(telJSON), SERVER_UUID_TABLE_NAME)
}

// CloseDB - closes a database gracefully
func CloseDB() {
	getCurrentDB()[CLOSE_DB].(func())()
}
