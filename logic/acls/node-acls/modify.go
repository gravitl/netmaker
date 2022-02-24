package nodeacls

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
)

// UpsertNodeACL - inserts or updates a node ACL on given network
func UpsertNodeACL(networkID NetworkID, nodeID NodeID, defaultVal byte) (NodeACL, error) {
	if defaultVal != NotAllowed && defaultVal != Allowed {
		defaultVal = NotAllowed
	}
	var currentNetworkACL, err = FetchCurrentACL(networkID)
	if err != nil {
		return nil, err
	}
	var newNodeACL = make(NodeACL)
	for existingNode := range currentNetworkACL {
		currentNetworkACL[existingNode][nodeID] = defaultVal
		newNodeACL[existingNode] = defaultVal
	}
	currentNetworkACL[nodeID] = newNodeACL
	return newNodeACL, nil
}

// UpsertNetworkACL - Inserts or updates a network ACL given the json string of the ACL and the network name
// if nil, create it
func UpsertNetworkACL(networkID NetworkID, networkACL NetworkACL) (NetworkACL, error) {
	if networkACL == nil {
		networkACL = make(NetworkACL)
	}
	return networkACL, database.Insert(string(networkID), string(convertNetworkACLtoACLJson(&networkACL)), database.NODE_ACLS_TABLE_NAME)
}

// RemoveNodeACL - removes a specific Node's ACL, returns the NetworkACL and error
func RemoveNodeACL(networkID NetworkID, nodeID NodeID) (NetworkACL, error) {
	var currentNeworkACL, err = FetchCurrentACL(networkID)
	if err != nil {
		return nil, err
	}
	for currentNodeID := range currentNeworkACL {
		delete(currentNeworkACL[nodeID], currentNodeID)
	}
	delete(currentNeworkACL, nodeID)
	return UpsertNetworkACL(networkID, currentNeworkACL)
}

// RemoveNetworkACL - just delete the network ACL
func RemoveNetworkACL(networkID NetworkID) error {
	return database.DeleteRecord(database.NODE_ACLS_TABLE_NAME, string(networkID))
}

func convertNetworkACLtoACLJson(networkACL *NetworkACL) ACLJson {
	data, err := json.Marshal(networkACL)
	if err != nil {
		return ""
	}
	return ACLJson(data)
}
