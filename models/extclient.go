package models

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
)

// ExtClient - struct for external clients
type ExtClient struct {
	ClientID               string `json:"clientid" bson:"clientid"`
	Description            string `json:"description" bson:"description"`
	PrivateKey             string `json:"privatekey" bson:"privatekey"`
	PublicKey              string `json:"publickey" bson:"publickey"`
	Network                string `json:"network" bson:"network"`
	Address                string `json:"address" bson:"address"`
	IngressGatewayID       string `json:"ingressgatewayid" bson:"ingressgatewayid"`
	IngressGatewayEndpoint string `json:"ingressgatewayendpoint" bson:"ingressgatewayendpoint"`
	LastModified           int64  `json:"lastmodified" bson:"lastmodified"`
}

// ExtClient.GetEgressRangesOnNetwork - returns the egress ranges on network of ext client
func (client *ExtClient) GetEgressRangesOnNetwork() ([]string, error) {

	var result []string
	nodesData, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		return []string{}, err
	}
	for _, nodeData := range nodesData {
		var currentNode Node
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
