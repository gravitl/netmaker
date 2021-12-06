package logic

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
)

// DecrimentKey - decriments key uses
func DecrimentKey(networkName string, keyvalue string) {

	var network models.Network

	network, err := GetParentNetwork(networkName)
	if err != nil {
		return
	}

	for i := len(network.AccessKeys) - 1; i >= 0; i-- {

		currentkey := network.AccessKeys[i]
		if currentkey.Value == keyvalue {
			network.AccessKeys[i].Uses--
			if network.AccessKeys[i].Uses < 1 {
				network.AccessKeys = append(network.AccessKeys[:i],
					network.AccessKeys[i+1:]...)
				break
			}
		}
	}

	if newNetworkData, err := json.Marshal(&network); err != nil {
		logger.Log(2, "failed to decrement key")
		return
	} else {
		database.Insert(network.NetID, string(newNetworkData), database.NETWORKS_TABLE_NAME)
	}
}

// IsKeyValid - check if key is valid
func IsKeyValid(networkname string, keyvalue string) bool {

	network, _ := GetParentNetwork(networkname)
	var key models.AccessKey
	foundkey := false
	isvalid := false

	for i := len(network.AccessKeys) - 1; i >= 0; i-- {
		currentkey := network.AccessKeys[i]
		if currentkey.Value == keyvalue {
			key = currentkey
			foundkey = true
		}
	}
	if foundkey {
		if key.Uses > 0 {
			isvalid = true
		}
	}
	return isvalid
}

func RemoveKeySensitiveInfo(keys []models.AccessKey) []models.AccessKey {
	var returnKeys []models.AccessKey
	for _, key := range keys {
		key.Value = models.PLACEHOLDER_KEY_TEXT
		key.AccessString = models.PLACEHOLDER_TOKEN_TEXT
		returnKeys = append(returnKeys, key)
	}
	return returnKeys
}
