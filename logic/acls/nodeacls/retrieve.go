package nodeacls

import (
	"encoding/json"
	"fmt"
	"maps"
	"sync"

	"github.com/gravitl/netmaker/logic/acls"
)

var NodesAllowedACLMutex = &sync.Mutex{}

// AreNodesAllowed - checks if nodes are allowed to communicate in their network ACL
func AreNodesAllowed(networkID NetworkID, node1, node2 NodeID) bool {
	NodesAllowedACLMutex.Lock()
	defer NodesAllowedACLMutex.Unlock()
	var currentNetworkACL, err = FetchAllACLs(networkID)
	if err != nil {
		return false
	}
	var allowed bool
	acls.AclMutex.Lock()
	currNetAclCopy := maps.Clone(currentNetworkACL)
	currNetworkACLNode1 := currNetAclCopy[acls.AclID(node1)]
	currNetworkACLNode2 := currNetAclCopy[acls.AclID(node2)]
	acls.AclMutex.Unlock()
	allowed = currNetworkACLNode1.IsAllowed(acls.AclID(node2)) && currNetworkACLNode2.IsAllowed(acls.AclID(node1))
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
