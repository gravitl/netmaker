package logic

import (
	"crypto/rsa"
	"fmt"
)

// RetrieveTrafficKey - retrieves public key based on node
func RetrieveTrafficKey() (rsa.PrivateKey, error) {
	var telRecord, err = fetchTelemetryRecord()
	if err != nil {
		return rsa.PrivateKey{}, err
	}
	fmt.Printf("fetched key %v \n", telRecord.TrafficKey)

	return telRecord.TrafficKey, nil
}
