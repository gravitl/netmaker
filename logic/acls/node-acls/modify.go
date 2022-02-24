package nodeacls

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic/acls"
)

// ChangeNodeACL - takes in two node IDs of a given network and changes them to specified allowed or not value
// returns the total network's ACL and error
func ChangeNodeACL(networkID acls.NetworkID, node1, node2 acls.NodeID, value byte) (acls.NetworkACL, error) {
	if value != acls.NotAllowed && value != acls.Allowed { // if invalid option make not allowed
		value = acls.NotAllowed
	}
	currentACL, err := FetchCurrentACL(networkID)
	if err != nil {
		return nil, err
	}
	// == make the access control change ==
	currentACL[node1][node2] = value
	currentACL[node2][node1] = value
	return UpsertNetworkACL(networkID, currentACL)
}

// CreateNodeACL - inserts or updates a node ACL on given network
func CreateNodeACL(networkID acls.NetworkID, nodeID acls.NodeID, defaultVal byte) (acls.NodeACL, error) {
	if defaultVal != acls.NotAllowed && defaultVal != acls.Allowed {
		defaultVal = acls.NotAllowed
	}
	var currentNetworkACL, err = FetchCurrentACL(networkID)
	if err != nil {
		return nil, err
	}
	var newNodeACL = make(acls.NodeACL)
	for existingNodeID := range currentNetworkACL {
		currentNetworkACL[existingNodeID][nodeID] = defaultVal // set the old nodes to default value for new node
		newNodeACL[existingNodeID] = defaultVal                // set the old nodes in new node ACL to default value
	}
	currentNetworkACL[nodeID] = newNodeACL                               // append the new node's ACL
	retNetworkACL, err := UpsertNetworkACL(networkID, currentNetworkACL) // insert into db, return result
	if err != nil {
		return nil, err
	}
	return retNetworkACL[nodeID], nil
}

// CreateNetworkACL - creates an empty ACL list in a given network
func CreateNetworkACL(networkID acls.NetworkID) (acls.NetworkACL, error) {
	var networkACL = make(acls.NetworkACL)
	return networkACL, database.Insert(string(networkID), string(convertNetworkACLtoACLJson(&networkACL)), database.NODE_ACLS_TABLE_NAME)
}

// UpsertNodeACL - applies a NodeACL to the db, overwrites or creates
func UpsertNodeACL(networkID acls.NetworkID, nodeID acls.NodeID, nodeACL acls.NodeACL) (acls.NodeACL, error) {
	currentNetACL, err := FetchCurrentACL(networkID)
	if err != nil {
		return nodeACL, err
	}
	currentNetACL[nodeID] = nodeACL
	_, err = UpsertNetworkACL(networkID, currentNetACL)
	return nodeACL, err
}

// UpsertNetworkACL - Inserts or updates a network ACL given the json string of the ACL and the network name
// if nil, create it
func UpsertNetworkACL(networkID acls.NetworkID, networkACL acls.NetworkACL) (acls.NetworkACL, error) {
	if networkACL == nil {
		networkACL = make(acls.NetworkACL)
	}
	return networkACL, database.Insert(string(networkID), string(convertNetworkACLtoACLJson(&networkACL)), database.NODE_ACLS_TABLE_NAME)
}

// RemoveNodeACL - removes a specific Node's ACL, returns the NetworkACL and error
func RemoveNodeACL(networkID acls.NetworkID, nodeID acls.NodeID) (acls.NetworkACL, error) {
	var currentNeworkACL, err = FetchCurrentACL(networkID)
	if err != nil {
		return nil, err
	}
	for currentNodeID := range currentNeworkACL {
		if currentNodeID != nodeID {
			delete(currentNeworkACL[currentNodeID], nodeID)
		}
	}
	delete(currentNeworkACL, nodeID)
	return UpsertNetworkACL(networkID, currentNeworkACL)
}

// RemoveNetworkACL - just delete the network ACL
func RemoveNetworkACL(networkID acls.NetworkID) error {
	return database.DeleteRecord(database.NODE_ACLS_TABLE_NAME, string(networkID))
}

func convertNetworkACLtoACLJson(networkACL *acls.NetworkACL) acls.ACLJson {
	data, err := json.Marshal(networkACL)
	if err != nil {
		return ""
	}
	return acls.ACLJson(data)
}
