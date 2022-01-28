package logic

import (
	"crypto/rsa"
	"encoding/json"

	"github.com/gravitl/netmaker/database"
)

type trafficKey struct {
	Key rsa.PrivateKey `json:"key" bson:"key"`
}

// RetrieveTrafficKey - retrieves key based on node
func RetrieveTrafficKey(nodeid string) (rsa.PrivateKey, error) {
	var record, err = database.FetchRecord(database.TRAFFIC_TABLE_NAME, nodeid)
	if err != nil {
		return rsa.PrivateKey{}, err
	}
	var result trafficKey
	if err = json.Unmarshal([]byte(record), &result); err != nil {
		return rsa.PrivateKey{}, err
	}
	return result.Key, nil
}

// StoreTrafficKey - stores key based on node
func StoreTrafficKey(nodeid string, key rsa.PrivateKey) error {
	var data, err = json.Marshal(trafficKey{
		Key: key,
	})
	if err != nil {
		return err
	}
	return database.Insert(nodeid, string(data), database.TRAFFIC_TABLE_NAME)
}
