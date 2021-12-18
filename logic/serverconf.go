package logic

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
)

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
