package logic

// RetrievePrivateTrafficKey - retrieves private key of server
func RetrievePrivateTrafficKey() ([]byte, error) {
	var telRecord, err = fetchTelemetryRecord()
	if err != nil {
		return nil, err
	}

	return telRecord.TrafficKeyPriv, nil
}

// RetrievePublicTrafficKey - retrieves public key of server
func RetrievePublicTrafficKey() ([]byte, error) {
	var telRecord, err = fetchTelemetryRecord()
	if err != nil {
		return nil, err
	}

	return telRecord.TrafficKeyPub, nil
}
