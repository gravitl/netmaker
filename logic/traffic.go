package logic

import (
	"crypto/rsa"
	"fmt"
	"math/big"
)

// RetrievePrivateTrafficKey - retrieves private key of server
func RetrievePrivateTrafficKey() (rsa.PrivateKey, error) {
	var telRecord, err = fetchTelemetryRecord()
	if err != nil {
		return rsa.PrivateKey{}, err
	}
	fmt.Printf("fetched priv key %v \n", telRecord.TrafficKeyPriv)

	return telRecord.TrafficKeyPriv, nil
}

// RetrievePublicTrafficKey - retrieves public key of server
func RetrievePublicTrafficKey() (rsa.PublicKey, big.Int, error) {
	var telRecord, err = fetchTelemetryRecord()
	if err != nil {
		return rsa.PublicKey{}, big.Int{}, err
	}
	fmt.Printf("fetched pub key %v \n", telRecord.TrafficKeyPub)

	return telRecord.TrafficKeyPub, telRecord.PubMod, nil
}
