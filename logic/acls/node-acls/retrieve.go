package nodeacls

import (
	"encoding/json"
	"fmt"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic/acls"
)

// AreNodesAllowed - checks if nodes are allowed to communicate in their network ACL
func AreNodesAllowed(networkID acls.NetworkID, node1, node2 acls.NodeID) bool {
	var currentNetworkACL, err = FetchCurrentACL(networkID)
	if err != nil {
		return false
	}
	return currentNetworkACL[node1][node2] == acls.Allowed && currentNetworkACL[node2][node1] == acls.Allowed
}

// FetchNodeACL - fetches a specific node's ACL in a given network
func FetchNodeACL(networkID acls.NetworkID, nodeID acls.NodeID) (acls.NodeACL, error) {
	currentNetACL, err := FetchCurrentACL(networkID)
	if err != nil {
		return nil, err
	}
	if currentNetACL[nodeID] == nil {
		return nil, fmt.Errorf("no node ACL present for node %s", nodeID)
	}
	return currentNetACL[nodeID], nil
}

// FetchNodeACLJson - fetches a node's acl in given network except returns the json string
func FetchNodeACLJson(networkID acls.NetworkID, nodeID acls.NodeID) (acls.ACLJson, error) {
	currentNodeACL, err := FetchNodeACL(networkID, nodeID)
	if err != nil {
		return "", err
	}
	jsonData, err := json.Marshal(&currentNodeACL)
	if err != nil {
		return "", err
	}
	return acls.ACLJson(jsonData), nil
}

// FetchCurrentACL - fetches all current node rules in given network ACL
func FetchCurrentACL(networkID acls.NetworkID) (acls.NetworkACL, error) {
	aclJson, err := FetchCurrentACLJson(acls.NetworkID(networkID))
	if err != nil {
		return nil, err
	}
	var currentNetworkACL acls.NetworkACL
	if err := json.Unmarshal([]byte(aclJson), &currentNetworkACL); err != nil {
		return nil, err
	}
	return currentNetworkACL, nil
}

// FetchCurrentACLJson - fetch the current ACL of given network except in json string
func FetchCurrentACLJson(networkID acls.NetworkID) (acls.ACLJson, error) {
	currentACLs, err := database.FetchRecord(database.NODE_ACLS_TABLE_NAME, string(networkID))
	if err != nil {
		return acls.ACLJson(""), err
	}
	return acls.ACLJson(currentACLs), nil
}
