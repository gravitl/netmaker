package logic

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
)

// GetExtPeersList - gets the ext peers lists
func GetExtPeersList(macaddress string, networkName string) ([]models.ExtPeersResponse, error) {

	var peers []models.ExtPeersResponse
	records, err := database.FetchRecords(database.EXT_CLIENT_TABLE_NAME)

	if err != nil {
		return peers, err
	}

	for _, value := range records {
		var peer models.ExtPeersResponse
		var extClient models.ExtClient
		err = json.Unmarshal([]byte(value), &peer)
		if err != nil {
			functions.PrintUserLog(models.NODE_SERVER_NAME, "failed to unmarshal peer", 2)
			continue
		}
		err = json.Unmarshal([]byte(value), &extClient)
		if err != nil {
			functions.PrintUserLog(models.NODE_SERVER_NAME, "failed to unmarshal ext client", 2)
			continue
		}
		if extClient.Network == networkName && extClient.IngressGatewayID == macaddress {
			peers = append(peers, peer)
		}
	}
	return peers, err
}
