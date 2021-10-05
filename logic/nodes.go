package logic

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
)

func GetNetworkNodes(network string) ([]models.Node, error) {
	var nodes []models.Node
	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return []models.Node{}, nil
		}
		return nodes, err
	}
	for _, value := range collection {

		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			continue
		}
		if node.Network == network {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func GetSortedNetworkServerNodes(network string) ([]models.Node, error) {
	var nodes []models.Node
	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return []models.Node{}, nil
		}
		return nodes, err
	}
	for _, value := range collection {

		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			continue
		}
		if node.Network == network && node.IsServer == "yes" {
			nodes = append(nodes, node)
		}
	}
	sort.Sort(models.NodesArray(nodes))
	return nodes, nil
}

func GetPeers(node models.Node) ([]models.Node, error) {
	if node.IsServer == "yes" && IsLeader(&node) {
		SetNetworkServerPeers(&node)
	}
	excludeIsRelayed := node.IsRelay != "yes"
	var relayedNode string
	if node.IsRelayed == "yes" {
		relayedNode = node.Address
	}
	peers, err := GetPeersList(node.Network, excludeIsRelayed, relayedNode)
	if err != nil {
		return nil, err
	}
	return peers, nil
}

func IsLeader(node *models.Node) bool {
	nodes, err := GetSortedNetworkServerNodes(node.Network)
	if err != nil {
		functions.PrintUserLog("[netmaker]", "ERROR: COULD NOT RETRIEVE SERVER NODES. THIS WILL BREAK HOLE PUNCHING.", 0)
		return false
	}
	for _, n := range nodes {
		if n.LastModified > time.Now().Add(-1*time.Minute).Unix() {
			return n.Address == node.Address
		}
	}
	return len(nodes) <= 1 || nodes[1].Address == node.Address
}
