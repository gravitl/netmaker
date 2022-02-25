package nodeacls

import (
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic/acls"
)

// CreateNodeACL - inserts or updates a node ACL on given network
func CreateNodeACL(networkID NetworkID, nodeID NodeID, defaultVal byte) (acls.ACL, error) {
	if defaultVal != acls.NotAllowed && defaultVal != acls.Allowed {
		defaultVal = acls.NotAllowed
	}
	var currentNetworkACL, err = acls.FetchACLContainer(acls.ContainerID(networkID))
	if err != nil {
		return nil, err
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

// RemoveNodeACL - removes a specific Node's ACL, returns the NetworkACL and error
func RemoveNodeACL(networkID NetworkID, nodeID NodeID) (acls.ACLContainer, error) {
	var currentNeworkACL, err = acls.FetchACLContainer(acls.ContainerID(networkID))
	if err != nil {
		return nil, err
	}
	for currentNodeID := range currentNeworkACL {
		if NodeID(currentNodeID) != nodeID {
			currentNeworkACL[currentNodeID].Remove(acls.AclID(nodeID))
		}
	}
	delete(currentNeworkACL, acls.AclID(nodeID))
	return currentNeworkACL.Save(acls.ContainerID(networkID))
}

// RemoveNetworkACL - just delete the network ACL
func RemoveNetworkACL(networkID NetworkID) error {
	return database.DeleteRecord(database.NODE_ACLS_TABLE_NAME, string(networkID))
}
