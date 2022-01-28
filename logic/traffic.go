package logic

import (
	"crypto/rsa"
)

// RetrieveTrafficKey - retrieves key based on node
func RetrieveTrafficKey() (rsa.PrivateKey, error) {
	var telRecord, err = fetchTelemetryRecord()
	if err != nil {
		return rsa.PrivateKey{}, err
	}
	return telRecord.TrafficKey, nil
}
