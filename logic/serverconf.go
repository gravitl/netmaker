package logic

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
)

var (
	// Node_Limit - dummy var for community
	Node_Limit = 1000000000
	// Networks_Limit - dummy var for community
	Networks_Limit = 1000000000
	// Users_Limit - dummy var for community
	Users_Limit = 1000000000
	// Clients_Limit - dummy var for community
	Clients_Limit = 1000000000
	// Free_Tier - specifies if free tier
	Free_Tier = false
)

// constant for database key for storing server ids
const server_id_key = "nm-server-id"

type serverData struct {
	PrivateKey string `json:"privatekey,omitempty" bson:"privatekey,omitempty"`
}

// StorePrivKey - stores server client WireGuard privatekey if needed
func StorePrivKey(serverID string, privateKey string) error {
	var newData = serverData{}
	var err error
	var data []byte
	newData.PrivateKey = privateKey
	data, err = json.Marshal(&newData)
	if err != nil {
		return err
	}
	return database.Insert(serverID, string(data), database.SERVERCONF_TABLE_NAME)
}

// FetchPrivKey - fetches private key
func FetchPrivKey(serverID string) (string, error) {
	var dbData string
	var err error
	var fetchedData = serverData{}
	dbData, err = database.FetchRecord(database.SERVERCONF_TABLE_NAME, serverID)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal([]byte(dbData), &fetchedData)
	if err != nil {
		return "", err
	}
	return fetchedData.PrivateKey, nil
}

// RemovePrivKey - removes a private key
func RemovePrivKey(serverID string) error {
	return database.DeleteRecord(database.SERVERCONF_TABLE_NAME, serverID)
}

// FetchJWTSecret - fetches jwt secret from db
func FetchJWTSecret() (string, error) {
	var dbData string
	var err error
	var fetchedData = serverData{}
	dbData, err = database.FetchRecord(database.SERVERCONF_TABLE_NAME, "nm-jwt-secret")
	if err != nil {
		return "", err
	}
	err = json.Unmarshal([]byte(dbData), &fetchedData)
	if err != nil {
		return "", err
	}
	return fetchedData.PrivateKey, nil
}

// StoreJWTSecret - stores server jwt secret if needed
func StoreJWTSecret(privateKey string) error {
	var newData = serverData{}
	var err error
	var data []byte
	newData.PrivateKey = privateKey
	data, err = json.Marshal(&newData)
	if err != nil {
		return err
	}
	return database.Insert("nm-jwt-secret", string(data), database.SERVERCONF_TABLE_NAME)
}
