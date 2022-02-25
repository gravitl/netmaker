package acls

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
)

// CreateACLContainer - creates an empty ACL list in a given network
func CreateACLContainer(networkID ContainerID) (ACLContainer, error) {
	var aclContainer = make(ACLContainer)
	return aclContainer, database.Insert(string(networkID), string(convertNetworkACLtoACLJson(aclContainer)), database.NODE_ACLS_TABLE_NAME)
}

// FetchACLContainer - fetches all current node rules in given network ACL
func FetchACLContainer(networkID ContainerID) (ACLContainer, error) {
	aclJson, err := FetchACLContainerJson(ContainerID(networkID))
	if err != nil {
		return nil, err
	}
	var currentNetworkACL ACLContainer
	if err := json.Unmarshal([]byte(aclJson), &currentNetworkACL); err != nil {
		return nil, err
	}
	return currentNetworkACL, nil
}

// FetchACLContainerJson - fetch the current ACL of given network except in json string
func FetchACLContainerJson(networkID ContainerID) (ACLJson, error) {
	currentACLs, err := database.FetchRecord(database.NODE_ACLS_TABLE_NAME, string(networkID))
	if err != nil {
		return ACLJson(""), err
	}
	return ACLJson(currentACLs), nil
}

// == type functions ==

// ACL.AllowNode - allows a node by ID in memory
func (acl ACL) Allow(ID AclID) {
	acl[ID] = Allowed
}

// ACL.DisallowNode - disallows a node access by ID in memory
func (acl ACL) Disallow(ID AclID) {
	acl[ID] = NotAllowed
}

// ACL.Remove - removes a node from a ACL
func (acl ACL) Remove(ID AclID) {
	delete(acl, ID)
}

// ACL.Update - updates a ACL in DB
func (acl ACL) Save(networkID ContainerID, ID AclID) (ACL, error) {
	return upsertACL(networkID, ID, acl)
}

// ACL.IsNodeAllowed - sees if ID is allowed in referring ACL
func (acl ACL) IsNodeAllowed(ID AclID) bool {
	return acl[ID] == Allowed
}

// ACLContainer.UpdateNodeACL - saves the state of a ACL in the ACLContainer in memory
func (aclContainer ACLContainer) UpdateNodeACL(ID AclID, acl ACL) ACLContainer {
	aclContainer[ID] = acl
	return aclContainer
}

// ACLContainer.RemoveNodeACL - removes the state of a ACL in the ACLContainer in memory
func (aclContainer ACLContainer) RemoveNodeACL(ID AclID) ACLContainer {
	delete(aclContainer, ID)
	return aclContainer
}

// ACLContainer.ChangeNodesAccess - changes the relationship between two nodes in memory
func (networkACL ACLContainer) ChangeNodesAccess(ID1, ID2 AclID, value byte) {
	networkACL[ID1][ID2] = value
	networkACL[ID2][ID1] = value
}

// ACLContainer.Save - saves the state of a ACLContainer to the db
func (aclContainer ACLContainer) Save(networkID ContainerID) (ACLContainer, error) {
	return upsertACLContainer(networkID, aclContainer)
}

// == private ==

// upsertACL - applies a ACL to the db, overwrites or creates
func upsertACL(networkID ContainerID, ID AclID, acl ACL) (ACL, error) {
	currentNetACL, err := FetchACLContainer(networkID)
	if err != nil {
		return acl, err
	}
	currentNetACL[ID] = acl
	_, err = upsertACLContainer(networkID, currentNetACL)
	return acl, err
}

// upsertACLContainer - Inserts or updates a network ACL given the json string of the ACL and the network name
// if nil, create it
func upsertACLContainer(networkID ContainerID, aclContainer ACLContainer) (ACLContainer, error) {
	if aclContainer == nil {
		aclContainer = make(ACLContainer)
	}
	return aclContainer, database.Insert(string(networkID), string(convertNetworkACLtoACLJson(aclContainer)), database.NODE_ACLS_TABLE_NAME)
}

func convertNetworkACLtoACLJson(networkACL ACLContainer) ACLJson {
	data, err := json.Marshal(networkACL)
	if err != nil {
		return ""
	}
	return ACLJson(data)
}
