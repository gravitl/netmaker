package nodeacls

import (
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic/acls"
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
	var newNodeACL = make(acls.ACL)
	for existingNodeID := range currentNetworkACL {
		currentNetworkACL[existingNodeID][acls.AclID(nodeID)] = defaultVal // set the old nodes to default value for new node
		newNodeACL[existingNodeID] = defaultVal                            // set the old nodes in new node ACL to default value
	}
	currentNetworkACL[acls.AclID(nodeID)] = newNodeACL                        // append the new node's ACL
	retNetworkACL, err := currentNetworkACL.Save(acls.ContainerID(networkID)) // insert into db
	if err != nil {
		return nil, err
	}
	return retNetworkACL[acls.AclID(nodeID)], nil
}

// ChangeNodesAccess - changes relationship between two individual nodes in given network in memory
func ChangeNodesAccess(networkID NetworkID, node1, node2 NodeID, value byte) (acls.ACLContainer, error) {
	var currentNetworkACL, err = FetchAllACLs(networkID)
	if err != nil {
		return nil, err
	}
	currentNetworkACL.ChangeAccess(acls.AclID(node1), acls.AclID(node2), value)
	return currentNetworkACL, nil
}

// UpdateNodeACL - updates a node's ACL in state
func UpdateNodeACL(networkID NetworkID, nodeID NodeID, acl acls.ACL) (acls.ACL, error) {
	var currentNetworkACL, err = FetchAllACLs(networkID)
	if err != nil {
		return nil, err
	}
	currentNetworkACL[acls.AclID(nodeID)] = acl
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
