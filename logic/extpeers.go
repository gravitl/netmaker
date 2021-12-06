package logic

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
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
			logger.Log(2, "failed to unmarshal peer when getting ext peer list")
			continue
		}
		err = json.Unmarshal([]byte(value), &extClient)
		if err != nil {
			logger.Log(2, "failed to unmarshal ext client")
			continue
		}
		if extClient.Network == networkName && extClient.IngressGatewayID == macaddress {
			peers = append(peers, peer)
		}
	}
	return peers, err
}

// ExtClient.GetEgressRangesOnNetwork - returns the egress ranges on network of ext client
func GetEgressRangesOnNetwork(client *models.ExtClient) ([]string, error) {

	var result []string
	nodesData, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		return []string{}, err
	}
	for _, nodeData := range nodesData {
		var currentNode models.Node
		if err = json.Unmarshal([]byte(nodeData), &currentNode); err != nil {
			continue
		}
		if currentNode.Network != client.Network {
			continue
		}
		if currentNode.IsEgressGateway == "yes" { // add the egress gateway range(s) to the result
			if len(currentNode.EgressGatewayRanges) > 0 {
				result = append(result, currentNode.EgressGatewayRanges...)
			}
		}
	}

	return result, nil
}
