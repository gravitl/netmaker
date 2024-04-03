package database

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/nacl/box"
)

const (
	// == Table Names ==
	// NETWORKS_TABLE_NAME - networks table
	NETWORKS_TABLE_NAME = "networks"
	// NODES_TABLE_NAME - nodes table
	NODES_TABLE_NAME = "nodes"
	// DELETED_NODES_TABLE_NAME - deleted nodes table
	DELETED_NODES_TABLE_NAME = "deletednodes"
	// USERS_TABLE_NAME - users table
	USERS_TABLE_NAME = "users"
	// CERTS_TABLE_NAME - certificates table
	CERTS_TABLE_NAME = "certs"
	// DNS_TABLE_NAME - dns table
	DNS_TABLE_NAME = "dns"
	// EXT_CLIENT_TABLE_NAME - ext client table
	EXT_CLIENT_TABLE_NAME = "extclients"
	// PEERS_TABLE_NAME - peers table
	PEERS_TABLE_NAME = "peers"
	// SERVERCONF_TABLE_NAME - stores server conf
	SERVERCONF_TABLE_NAME = "serverconf"
	// SERVER_UUID_TABLE_NAME - stores unique netmaker server data
	SERVER_UUID_TABLE_NAME = "serveruuid"
	// SERVER_UUID_RECORD_KEY - telemetry thing
	SERVER_UUID_RECORD_KEY = "serveruuid"
	// DATABASE_FILENAME - database file name
	DATABASE_FILENAME = "netmaker.db"
	// GENERATED_TABLE_NAME - stores server generated k/v
	GENERATED_TABLE_NAME = "generated"
	// NODE_ACLS_TABLE_NAME - stores the node ACL rules
	NODE_ACLS_TABLE_NAME = "nodeacls"
	// SSO_STATE_CACHE - holds sso session information for OAuth2 sign-ins
	SSO_STATE_CACHE = "ssostatecache"
	// METRICS_TABLE_NAME - stores network metrics
	METRICS_TABLE_NAME = "metrics"
	// NETWORK_USER_TABLE_NAME - network user table tracks stats for a network user per network
	NETWORK_USER_TABLE_NAME = "networkusers"
	// USER_GROUPS_TABLE_NAME - table for storing usergroups
	USER_GROUPS_TABLE_NAME = "usergroups"
	// CACHE_TABLE_NAME - caching table
	CACHE_TABLE_NAME = "cache"
	// HOSTS_TABLE_NAME - the table name for hosts
	HOSTS_TABLE_NAME = "hosts"
	// ENROLLMENT_KEYS_TABLE_NAME - table name for enrollmentkeys
	ENROLLMENT_KEYS_TABLE_NAME = "enrollmentkeys"
	// HOST_ACTIONS_TABLE_NAME - table name for enrollmentkeys
	HOST_ACTIONS_TABLE_NAME = "hostactions"
	// PENDING_USERS_TABLE_NAME - table name for pending users
	PENDING_USERS_TABLE_NAME = "pending_users"
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
	// INSERT_PEER - insert peer into db const
	INSERT_PEER = "insertpeer"
	// DELETE - delete db record const
	DELETE = "delete"
	// DELETE_ALL - delete a table const
	DELETE_ALL = "deleteall"
	// FETCH_ALL - fetch table contents const
	FETCH_ALL = "fetchall"
	// CLOSE_DB - graceful close of db const
	CLOSE_DB = "closedb"
	// isconnected
	isConnected = "isconnected"
)

var dbMutex sync.RWMutex

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
	return initializeUUID()
}

func createTables() {
	CreateTable(NETWORKS_TABLE_NAME)
	CreateTable(NODES_TABLE_NAME)
	CreateTable(CERTS_TABLE_NAME)
	CreateTable(DELETED_NODES_TABLE_NAME)
	CreateTable(USERS_TABLE_NAME)
	CreateTable(DNS_TABLE_NAME)
	CreateTable(EXT_CLIENT_TABLE_NAME)
	CreateTable(PEERS_TABLE_NAME)
	CreateTable(SERVERCONF_TABLE_NAME)
	CreateTable(SERVER_UUID_TABLE_NAME)
	CreateTable(GENERATED_TABLE_NAME)
	CreateTable(NODE_ACLS_TABLE_NAME)
	CreateTable(SSO_STATE_CACHE)
	CreateTable(METRICS_TABLE_NAME)
	CreateTable(NETWORK_USER_TABLE_NAME)
	CreateTable(USER_GROUPS_TABLE_NAME)
	CreateTable(CACHE_TABLE_NAME)
	CreateTable(HOSTS_TABLE_NAME)
	CreateTable(ENROLLMENT_KEYS_TABLE_NAME)
	CreateTable(HOST_ACTIONS_TABLE_NAME)
	CreateTable(PENDING_USERS_TABLE_NAME)
}

func CreateTable(tableName string) error {
	return getCurrentDB()[CREATE_TABLE].(func(string) error)(tableName)
}

// IsJSONString - checks if valid json
func IsJSONString(value string) bool {
	var jsonInt interface{}
	var nodeInt models.Node
	return json.Unmarshal([]byte(value), &jsonInt) == nil || json.Unmarshal([]byte(value), &nodeInt) == nil
}

// Insert - inserts object into db
func Insert(key string, value string, tableName string) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()
	if key != "" && value != "" && IsJSONString(value) {
		return getCurrentDB()[INSERT].(func(string, string, string) error)(key, value, tableName)
	} else {
		return errors.New("invalid insert " + key + " : " + value)
	}
}

// InsertPeer - inserts peer into db
func InsertPeer(key string, value string) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()
	if key != "" && value != "" && IsJSONString(value) {
		return getCurrentDB()[INSERT_PEER].(func(string, string) error)(key, value)
	} else {
		return errors.New("invalid peer insert " + key + " : " + value)
	}
}

// DeleteRecord - deletes a record from db
func DeleteRecord(tableName string, key string) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()
	return getCurrentDB()[DELETE].(func(string, string) error)(tableName, key)
}

// DeleteAllRecords - removes a table and remakes
func DeleteAllRecords(tableName string) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()
	err := getCurrentDB()[DELETE_ALL].(func(string) error)(tableName)
	if err != nil {
		return err
	}
	err = CreateTable(tableName)
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
	dbMutex.RLock()
	defer dbMutex.RUnlock()
	return getCurrentDB()[FETCH_ALL].(func(string) (map[string]string, error))(tableName)
}

// initializeUUID - create a UUID record for server if none exists
func initializeUUID() error {
	records, err := FetchRecords(SERVER_UUID_TABLE_NAME)
	if err != nil {
		if !IsEmptyRecord(err) {
			return err
		}
	} else if len(records) > 0 {
		return nil
	}
	// setup encryption keys
	var trafficPubKey, trafficPrivKey, errT = box.GenerateKey(rand.Reader) // generate traffic keys
	if errT != nil {
		return errT
	}
	tPriv, err := ncutils.ConvertKeyToBytes(trafficPrivKey)
	if err != nil {
		return err
	}

	tPub, err := ncutils.ConvertKeyToBytes(trafficPubKey)
	if err != nil {
		return err
	}

	telemetry := models.Telemetry{
		UUID:           uuid.NewString(),
		TrafficKeyPriv: tPriv,
		TrafficKeyPub:  tPub,
	}
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

// IsConnected - tell if the database is connected or not
func IsConnected() bool {
	return getCurrentDB()[isConnected].(func() bool)()
}
