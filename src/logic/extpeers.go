package logic

import (
	"encoding/json"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// GetExtPeersList - gets the ext peers lists
func GetExtPeersList(node *models.Node) ([]models.ExtPeersResponse, error) {

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
		if extClient.Network == node.Network && extClient.IngressGatewayID == node.ID {
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

// DeleteExtClient - deletes an existing ext client
func DeleteExtClient(network string, clientid string) error {
	key, err := GetRecordKey(clientid, network)
	if err != nil {
		return err
	}
	err = database.DeleteRecord(database.EXT_CLIENT_TABLE_NAME, key)
	return err
}

// GetNetworkExtClients - gets the ext clients of given network
func GetNetworkExtClients(network string) ([]models.ExtClient, error) {
	var extclients []models.ExtClient

	records, err := database.FetchRecords(database.EXT_CLIENT_TABLE_NAME)
	if err != nil {
		return extclients, err
	}
	for _, value := range records {
		var extclient models.ExtClient
		err = json.Unmarshal([]byte(value), &extclient)
		if err != nil {
			continue
		}
		if extclient.Network == network {
			extclients = append(extclients, extclient)
		}
	}
	return extclients, err
}

// GetExtClient - gets a single ext client on a network
func GetExtClient(clientid string, network string) (models.ExtClient, error) {
	var extclient models.ExtClient
	key, err := GetRecordKey(clientid, network)
	if err != nil {
		return extclient, err
	}
	data, err := database.FetchRecord(database.EXT_CLIENT_TABLE_NAME, key)
	if err != nil {
		return extclient, err
	}
	err = json.Unmarshal([]byte(data), &extclient)

	return extclient, err
}

// CreateExtClient - creates an extclient
func CreateExtClient(extclient *models.ExtClient) error {
	if extclient.PrivateKey == "" {
		privateKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}

		extclient.PrivateKey = privateKey.String()
		extclient.PublicKey = privateKey.PublicKey().String()
	}

	if extclient.Address == "" {
		newAddress, err := UniqueAddress(extclient.Network)
		if err != nil {
			return err
		}
		extclient.Address = newAddress
	}

	if extclient.ClientID == "" {
		extclient.ClientID = models.GenerateNodeName()
	}

	extclient.LastModified = time.Now().Unix()

	key, err := GetRecordKey(extclient.ClientID, extclient.Network)
	if err != nil {
		return err
	}
	data, err := json.Marshal(&extclient)
	if err != nil {
		return err
	}
	if err = database.Insert(key, string(data), database.EXT_CLIENT_TABLE_NAME); err != nil {
		return err
	}
	err = SetNetworkNodesLastModified(extclient.Network)
	return err
}

// UpdateExtClient - only supports name changes right now
func UpdateExtClient(newclientid string, network string, client *models.ExtClient) (*models.ExtClient, error) {

	err := DeleteExtClient(network, client.ClientID)
	if err != nil {
		return client, err
	}
	client.ClientID = newclientid
	CreateExtClient(client)
	return client, err
}
