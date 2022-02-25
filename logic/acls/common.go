package acls

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
)

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
func (acl ACL) Save(containerID ContainerID, ID AclID) (ACL, error) {
	return upsertACL(containerID, ID, acl)
}

// ACL.IsAllowed - sees if ID is allowed in referring ACL
func (acl ACL) IsAllowed(ID AclID) bool {
	return acl[ID] == Allowed
}

// ACLContainer.UpdateACL - saves the state of a ACL in the ACLContainer in memory
func (aclContainer ACLContainer) UpdateACL(ID AclID, acl ACL) ACLContainer {
	aclContainer[ID] = acl
	return aclContainer
}

// ACLContainer.RemoveACL - removes the state of a ACL in the ACLContainer in memory
func (aclContainer ACLContainer) RemoveACL(ID AclID) ACLContainer {
	delete(aclContainer, ID)
	return aclContainer
}

// ACLContainer.ChangeAccess - changes the relationship between two nodes in memory
func (networkACL ACLContainer) ChangeAccess(ID1, ID2 AclID, value byte) {
	networkACL[ID1][ID2] = value
	networkACL[ID2][ID1] = value
}

// ACLContainer.Save - saves the state of a ACLContainer to the db
func (aclContainer ACLContainer) Save(containerID ContainerID) (ACLContainer, error) {
	return upsertACLContainer(containerID, aclContainer)
}

// ACLContainer.New - saves the state of a ACLContainer to the db
func (aclContainer ACLContainer) New(containerID ContainerID) (ACLContainer, error) {
	return upsertACLContainer(containerID, nil)
}

// ACLContainer.Get - saves the state of a ACLContainer to the db
func (aclContainer ACLContainer) Get(containerID ContainerID) (ACLContainer, error) {
	return fetchACLContainer(containerID)
}

// == private ==

// fetchACLContainer - fetches all current node rules in given network ACL
func fetchACLContainer(containerID ContainerID) (ACLContainer, error) {
	aclJson, err := fetchACLContainerJson(ContainerID(containerID))
	if err != nil {
		return nil, err
	}
	var currentNetworkACL ACLContainer
	if err := json.Unmarshal([]byte(aclJson), &currentNetworkACL); err != nil {
		return nil, err
	}
	return currentNetworkACL, nil
}

// fetchACLContainerJson - fetch the current ACL of given network except in json string
func fetchACLContainerJson(containerID ContainerID) (ACLJson, error) {
	currentACLs, err := database.FetchRecord(database.NODE_ACLS_TABLE_NAME, string(containerID))
	if err != nil {
		return ACLJson(""), err
	}
	return ACLJson(currentACLs), nil
}

// upsertACL - applies a ACL to the db, overwrites or creates
func upsertACL(containerID ContainerID, ID AclID, acl ACL) (ACL, error) {
	currentNetACL, err := fetchACLContainer(containerID)
	if err != nil {
		return acl, err
	}
	currentNetACL[ID] = acl
	_, err = upsertACLContainer(containerID, currentNetACL)
	return acl, err
}

// upsertACLContainer - Inserts or updates a network ACL given the json string of the ACL and the network name
// if nil, create it
func upsertACLContainer(containerID ContainerID, aclContainer ACLContainer) (ACLContainer, error) {
	if aclContainer == nil {
		aclContainer = make(ACLContainer)
	}
	return aclContainer, database.Insert(string(containerID), string(convertNetworkACLtoACLJson(aclContainer)), database.NODE_ACLS_TABLE_NAME)
}

func convertNetworkACLtoACLJson(networkACL ACLContainer) ACLJson {
	data, err := json.Marshal(networkACL)
	if err != nil {
		return ""
	}
	return ACLJson(data)
}
