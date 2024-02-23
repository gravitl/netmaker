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
	var allowed bool
	acls.AclMutex.RLock()
	allowed = currentNetworkACL[acls.AclID(node1)].IsAllowed(acls.AclID(node2)) && currentNetworkACL[acls.AclID(node2)].IsAllowed(acls.AclID(node1))
	defer acls.AclMutex.RUnlock()
	return allowed
}

// FetchNodeACL - fetches a specific node's ACL in a given network
func FetchNodeACL(networkID NetworkID, nodeID NodeID) (acls.ACL, error) {
	var currentNetworkACL, err = FetchAllACLs(networkID)
	if err != nil {
		return nil, err
	}
	var acl acls.ACL
	acls.AclMutex.RLock()
	if currentNetworkACL[acls.AclID(nodeID)] == nil {
		acls.AclMutex.RUnlock()
		return nil, fmt.Errorf("no node ACL present for node %s", nodeID)
	}
	acl = currentNetworkACL[acls.AclID(nodeID)]
	acls.AclMutex.RUnlock()
	return acl, nil
}

// FetchNodeACLJson - fetches a node's acl in given network except returns the json string
func FetchNodeACLJson(networkID NetworkID, nodeID NodeID) (acls.ACLJson, error) {
	currentNodeACL, err := FetchNodeACL(networkID, nodeID)
	if err != nil {
		return "", err
	}
	acls.AclMutex.RLock()
	defer acls.AclMutex.RUnlock()
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
