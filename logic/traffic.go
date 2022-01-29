package logic

import (
	"fmt"
)

// RetrievePrivateTrafficKey - retrieves private key of server
func RetrievePrivateTrafficKey() ([]byte, error) {
	var telRecord, err = fetchTelemetryRecord()
	if err != nil {
		return nil, err
	}
	fmt.Printf("fetched priv key %v \n", string(telRecord.TrafficKeyPriv))

	return telRecord.TrafficKeyPriv, nil
}

// RetrievePublicTrafficKey - retrieves public key of server
func RetrievePublicTrafficKey() ([]byte, error) {
	var telRecord, err = fetchTelemetryRecord()
	if err != nil {
		return nil, err
	}
	fmt.Printf("fetched pub key %v \n", string(telRecord.TrafficKeyPub))

	return telRecord.TrafficKeyPub, nil
}
