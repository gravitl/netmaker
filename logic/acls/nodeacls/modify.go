package nodeacls

import (
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/servercfg"
)

// CreateNodeACL - inserts or updates a node ACL on given network and adds to state
func CreateNodeACL(networkID NetworkID, nodeID NodeID, defaultVal byte) (acls.ACL, error) {
	if defaultVal != acls.NotAllowed && defaultVal != acls.Allowed {
		defaultVal = acls.NotAllowed
	}
	var currentNetworkACL, err = FetchAllACLs(networkID)
	if err != nil {
		if database.IsEmptyRecord(err) {
			currentNetworkACL, err = currentNetworkACL.New(acls.ContainerID(networkID))
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	acls.AclMutex.Lock()
	var newNodeACL = make(acls.ACL)
	for existingNodeID := range currentNetworkACL {
		if currentNetworkACL[existingNodeID] == nil {
			currentNetworkACL[existingNodeID] = make(acls.ACL)
		}
		currentNetworkACL[existingNodeID][acls.AclID(nodeID)] = defaultVal // set the old nodes to default value for new node
		newNodeACL[existingNodeID] = defaultVal                            // set the old nodes in new node ACL to default value
	}
	currentNetworkACL[acls.AclID(nodeID)] = newNodeACL // append the new node's ACL
	acls.AclMutex.Unlock()
	retNetworkACL, err := currentNetworkACL.Save(acls.ContainerID(networkID)) // insert into db
	if err != nil {
		return nil, err
	}
	return retNetworkACL[acls.AclID(nodeID)], nil
}

// AllowNode - allow access between two nodes in memory
func AllowNodes(networkID NetworkID, node1, node2 NodeID) (acls.ACLContainer, error) {
	container, err := FetchAllACLs(networkID)
	if err != nil {
		return nil, err
	}
	container[acls.AclID(node1)].Allow(acls.AclID(node2))
	container[acls.AclID(node2)].Allow(acls.AclID(node1))
	return container, nil
}

// DisallowNodes - deny access between two nodes
func DisallowNodes(networkID NetworkID, node1, node2 NodeID) (acls.ACLContainer, error) {
	container, err := FetchAllACLs(networkID)
	if err != nil {
		return nil, err
	}
	container[acls.AclID(node1)].Disallow(acls.AclID(node2))
	container[acls.AclID(node2)].Disallow(acls.AclID(node1))
	return container, nil
}

// UpdateNodeACL - updates a node's ACL in state
func UpdateNodeACL(networkID NetworkID, nodeID NodeID, acl acls.ACL) (acls.ACL, error) {
	var currentNetworkACL, err = FetchAllACLs(networkID)
	if err != nil {
		return nil, err
	}
	acls.AclMutex.Lock()
	currentNetworkACL[acls.AclID(nodeID)] = acl
	acls.AclMutex.Unlock()
	return currentNetworkACL[acls.AclID(nodeID)].Save(acls.ContainerID(networkID), acls.AclID(nodeID))
}

// RemoveNodeACL - removes a specific Node's ACL, returns the NetworkACL and error
func RemoveNodeACL(networkID NetworkID, nodeID NodeID) (acls.ACLContainer, error) {
	var currentNetworkACL, err = FetchAllACLs(networkID)
	if err != nil {
		return nil, err
	}
	for currentNodeID := range currentNetworkACL {
		if NodeID(currentNodeID) != nodeID {
			currentNetworkACL[currentNodeID].Remove(acls.AclID(nodeID))
		}
	}
	delete(currentNetworkACL, acls.AclID(nodeID))
	return currentNetworkACL.Save(acls.ContainerID(networkID))
}

// DeleteACLContainer - removes an ACLContainer state from db
func DeleteACLContainer(network NetworkID) error {
	err := database.DeleteRecord(database.NODE_ACLS_TABLE_NAME, string(network))
	if err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		acls.DeleteAclFromCache(acls.ContainerID(network))
	}
	return nil
}
