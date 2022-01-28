package logic

import (
	"crypto/rsa"
	"encoding/gob"
	"fmt"
)

// RetrieveTrafficKey - retrieves public key based on node
func RetrieveTrafficKey() (rsa.PrivateKey, error) {
	var telRecord, err = fetchTelemetryRecord()
	if err != nil {
		return rsa.PrivateKey{}, err
	}
	var key = rsa.PrivateKey{}
	if err = gob.NewDecoder(&telRecord.TrafficKey).Decode(&key); err != nil {
		return rsa.PrivateKey{}, err
	}
	fmt.Printf("retrieved key: %v \n", key.PublicKey)

	return key, nil
}
