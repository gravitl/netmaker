package nodeacls

import (
	"encoding/json"
	"fmt"

	"github.com/gravitl/netmaker/logic/acls"
)

// AreNodesAllowed - checks if nodes are allowed to communicate in their network ACL
func AreNodesAllowed(networkID NetworkID, node1, node2 NodeID) bool {
	var currentNetworkACL, err = FetchAllACLs(networkID)
	if err != nil {
		return false
	}
	return currentNetworkACL[acls.AclID(node1)].IsAllowed(acls.AclID(node2)) && currentNetworkACL[acls.AclID(node2)].IsAllowed(acls.AclID(node1))
}

// FetchNodeACL - fetches a specific node's ACL in a given network
func FetchNodeACL(networkID NetworkID, nodeID NodeID) (acls.ACL, error) {
	var currentNetworkACL, err = FetchAllACLs(networkID)
	if err != nil {
		return nil, err
	}
	if currentNetworkACL[acls.AclID(nodeID)] == nil {
		return nil, fmt.Errorf("no node ACL present for node %s", nodeID)
	}
	return currentNetworkACL[acls.AclID(nodeID)], nil
}

// FetchNodeACLJson - fetches a node's acl in given network except returns the json string
func FetchNodeACLJson(networkID NetworkID, nodeID NodeID) (acls.ACLJson, error) {
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

// FetchAllACLs - fetchs all node
func FetchAllACLs(networkID NetworkID) (acls.ACLContainer, error) {
	var err error
	var currentNetworkACL acls.ACLContainer
	currentNetworkACL, err = currentNetworkACL.Get(acls.ContainerID(networkID))
	if err != nil {
		return nil, err
	}
	return currentNetworkACL, nil
}
