package logic

import (
	"crypto/rsa"
	"encoding/json"
)

// RetrieveTrafficKey - retrieves public key based on node
func RetrieveTrafficKey() (rsa.PrivateKey, error) {
	var telRecord, err = fetchTelemetryRecord()
	if err != nil {
		return rsa.PrivateKey{}, err
	}
	var key rsa.PrivateKey
	json.Unmarshal([]byte(telRecord.TrafficKey), &key)

	return key, nil
}
