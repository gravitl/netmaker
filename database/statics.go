package database

import (
	"encoding/json"
	"strings"
)

func SetPeers(newPeers map[string]string, networkName string) bool {
	areEqual := PeersAreEqual(newPeers, networkName)
	if !areEqual {
		jsonData, err := json.Marshal(newPeers)
		if err != nil {
			return false
		}
		InsertPeer(networkName, string(jsonData))
		return true
	}
	return !areEqual
}
func GetPeers(networkName string) (map[string]string, error) {
	record, err := FetchRecord(PEERS_TABLE_NAME, networkName)
	if err != nil && !IsEmptyRecord(err) {
		return nil, err
	}
	currentDataMap := make(map[string]string)
	if IsEmptyRecord(err) {
		return currentDataMap, nil
	}
	err = json.Unmarshal([]byte(record), &currentDataMap)
	return currentDataMap, err
}

func PeersAreEqual(toCompare map[string]string, networkName string) bool {
	currentDataMap, err := GetPeers(networkName)
	if err != nil {
		return false
	}
	if len(currentDataMap) != len(toCompare) {
		return false
	}
	for k := range currentDataMap {
		if toCompare[k] != currentDataMap[k] {
			return false
		}
	}
	return true
}

func IsEmptyRecord(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), NO_RECORD) || strings.Contains(err.Error(), NO_RECORDS)
}
