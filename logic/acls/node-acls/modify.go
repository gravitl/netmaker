package nodeacls

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
)

// CreateNodeACL - inserts or updates a node ACL on given network
func CreateNodeACL(networkID NetworkID, nodeID NodeID, defaultVal byte) (NodeACL, error) {
	if defaultVal != NotAllowed && defaultVal != Allowed {
		defaultVal = NotAllowed
	}
	var currentNetworkACL, err = FetchCurrentACL(networkID)
	if err != nil {
		return nil, err
	}
	var newNodeACL = make(NodeACL)
	for existingNodeID := range currentNetworkACL {
		currentNetworkACL[existingNodeID][nodeID] = defaultVal // set the old nodes to default value for new node
		newNodeACL[existingNodeID] = defaultVal                // set the old nodes in new node ACL to default value
	}
	currentNetworkACL[nodeID] = newNodeACL                               // append the new node's ACL
	retNetworkACL, err := upsertNetworkACL(networkID, currentNetworkACL) // insert into db, return result
	if err != nil {
		return nil, err
	}
	return retNetworkACL[nodeID], nil
}

// CreateNetworkACL - creates an empty ACL list in a given network
func CreateNetworkACL(networkID NetworkID) (NetworkACL, error) {
	var networkACL = make(NetworkACL)
	return networkACL, database.Insert(string(networkID), string(convertNetworkACLtoACLJson(&networkACL)), database.NODE_ACLS_TABLE_NAME)
}

// RemoveNodeACL - removes a specific Node's ACL, returns the NetworkACL and error
func RemoveNodeACL(networkID NetworkID, nodeID NodeID) (NetworkACL, error) {
	var currentNeworkACL, err = FetchCurrentACL(networkID)
	if err != nil {
		return nil, err
	}
	for currentNodeID := range currentNeworkACL {
		if currentNodeID != nodeID {
			currentNeworkACL[currentNodeID].RemoveNode(nodeID)
		}
	}
	delete(currentNeworkACL, nodeID)
	return currentNeworkACL.Save(networkID)
}

// RemoveNetworkACL - just delete the network ACL
func RemoveNetworkACL(networkID NetworkID) error {
	return database.DeleteRecord(database.NODE_ACLS_TABLE_NAME, string(networkID))
}

// NodeACL.AllowNode - allows a node by ID in memory
func (nodeACL NodeACL) AllowNode(nodeID NodeID) {
	nodeACL[nodeID] = Allowed
}

// NodeACL.DisallowNode - disallows a node access by ID in memory
func (nodeACL NodeACL) DisallowNode(nodeID NodeID) {
	nodeACL[nodeID] = NotAllowed
}

// NodeACL.RemoveNode - removes a node from a NodeACL
func (nodeACL NodeACL) RemoveNode(nodeID NodeID) {
	delete(nodeACL, nodeID)
}

// NodeACL.Update - updates a nodeACL in DB
func (nodeACL NodeACL) Save(networkID NetworkID, nodeID NodeID) (NodeACL, error) {
	return upsertNodeACL(networkID, nodeID, nodeACL)
}

// NodeACL.IsNodeAllowed - sees if nodeID is allowed in referring NodeACL
func (nodeACL NodeACL) IsNodeAllowed(nodeID NodeID) bool {
	return nodeACL[nodeID] == Allowed
}

// NetworkACL.UpdateNodeACL - saves the state of a NodeACL in the NetworkACL in memory
func (networkACL NetworkACL) UpdateNodeACL(nodeID NodeID, nodeACL NodeACL) NetworkACL {
	networkACL[nodeID] = nodeACL
	return networkACL
}

// NetworkACL.RemoveNodeACL - removes the state of a NodeACL in the NetworkACL in memory
func (networkACL NetworkACL) RemoveNodeACL(nodeID NodeID) NetworkACL {
	delete(networkACL, nodeID)
	return networkACL
}

// NetworkACL.ChangeNodesAccess - changes the relationship between two nodes in memory
func (networkACL NetworkACL) ChangeNodesAccess(nodeID1, nodeID2 NodeID, value byte) {
	networkACL[nodeID1][nodeID2] = value
	networkACL[nodeID2][nodeID1] = value
}

// NetworkACL.Save - saves the state of a NetworkACL to the db
func (networkACL NetworkACL) Save(networkID NetworkID) (NetworkACL, error) {
	return upsertNetworkACL(networkID, networkACL)
}

// == private ==

// upsertNodeACL - applies a NodeACL to the db, overwrites or creates
func upsertNodeACL(networkID NetworkID, nodeID NodeID, nodeACL NodeACL) (NodeACL, error) {
	currentNetACL, err := FetchCurrentACL(networkID)
	if err != nil {
		return nodeACL, err
	}
	currentNetACL[nodeID] = nodeACL
	_, err = upsertNetworkACL(networkID, currentNetACL)
	return nodeACL, err
}

// upsertNetworkACL - Inserts or updates a network ACL given the json string of the ACL and the network name
// if nil, create it
func upsertNetworkACL(networkID NetworkID, networkACL NetworkACL) (NetworkACL, error) {
	if networkACL == nil {
		networkACL = make(NetworkACL)
	}
	return networkACL, database.Insert(string(networkID), string(convertNetworkACLtoACLJson(&networkACL)), database.NODE_ACLS_TABLE_NAME)
}

func convertNetworkACLtoACLJson(networkACL *NetworkACL) ACLJson {
	data, err := json.Marshal(networkACL)
	if err != nil {
		return ""
	}
	return ACLJson(data)
}
