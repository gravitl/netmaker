package acls

import (
	"encoding/json"
	"sync"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

var (
	aclCacheMutex = &sync.RWMutex{}
	aclCacheMap   = make(map[ContainerID]ACLContainer)
	AclMutex      = &sync.RWMutex{}
)

func fetchAclContainerFromCache(containerID ContainerID) (aclCont ACLContainer, ok bool) {
	aclCacheMutex.RLock()
	aclCont, ok = aclCacheMap[containerID]
	aclCacheMutex.RUnlock()
	return
}

func storeAclContainerInCache(containerID ContainerID, aclContainer ACLContainer) {
	aclCacheMutex.Lock()
	aclCacheMap[containerID] = aclContainer
	aclCacheMutex.Unlock()
}

func DeleteAclFromCache(containerID ContainerID) {
	aclCacheMutex.Lock()
	delete(aclCacheMap, containerID)
	aclCacheMutex.Unlock()
}

// == type functions ==

// ACL.Allow - allows access by ID in memory
func (acl ACL) Allow(ID AclID) {
	AclMutex.Lock()
	defer AclMutex.Unlock()
	acl[ID] = Allowed
}

// ACL.DisallowNode - disallows access by ID in memory
func (acl ACL) Disallow(ID AclID) {
	AclMutex.Lock()
	defer AclMutex.Unlock()
	acl[ID] = NotAllowed
}

// ACL.Remove - removes a node from a ACL in memory
func (acl ACL) Remove(ID AclID) {
	AclMutex.Lock()
	defer AclMutex.Unlock()
	delete(acl, ID)
}

// ACL.Update - updates a ACL in DB
func (acl ACL) Save(containerID ContainerID, ID AclID) (ACL, error) {
	return upsertACL(containerID, ID, acl)
}

// ACL.IsAllowed - sees if ID is allowed in referring ACL
func (acl ACL) IsAllowed(ID AclID) (allowed bool) {
	AclMutex.RLock()
	allowed = acl[ID] == Allowed
	AclMutex.RUnlock()
	return
}

// ACLContainer.UpdateACL - saves the state of a ACL in the ACLContainer in memory
func (aclContainer ACLContainer) UpdateACL(ID AclID, acl ACL) ACLContainer {
	AclMutex.Lock()
	defer AclMutex.Unlock()
	aclContainer[ID] = acl
	return aclContainer
}

// ACLContainer.RemoveACL - removes the state of a ACL in the ACLContainer in memory
func (aclContainer ACLContainer) RemoveACL(ID AclID) ACLContainer {
	AclMutex.Lock()
	defer AclMutex.Unlock()
	delete(aclContainer, ID)
	return aclContainer
}

// ACLContainer.ChangeAccess - changes the relationship between two nodes in memory
func (networkACL ACLContainer) ChangeAccess(ID1, ID2 AclID, value byte) {
	if _, ok := networkACL[ID1]; !ok {
		slog.Error("ACL missing for ", "id", ID1)
		return
	}
	if _, ok := networkACL[ID2]; !ok {
		slog.Error("ACL missing for ", "id", ID2)
		return
	}
	if _, ok := networkACL[ID1][ID2]; !ok {
		slog.Error("ACL missing for ", "id1", ID1, "id2", ID2)
		return
	}
	if _, ok := networkACL[ID2][ID1]; !ok {
		slog.Error("ACL missing for ", "id2", ID2, "id1", ID1)
		return
	}
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

// fetchACLContainer - fetches all current rules in given ACL container
func fetchACLContainer(containerID ContainerID) (ACLContainer, error) {
	AclMutex.RLock()
	defer AclMutex.RUnlock()
	if servercfg.CacheEnabled() {
		if aclContainer, ok := fetchAclContainerFromCache(containerID); ok {
			return aclContainer, nil
		}
	}
	aclJson, err := fetchACLContainerJson(ContainerID(containerID))
	if err != nil {
		return nil, err
	}
	var currentNetworkACL ACLContainer
	if err := json.Unmarshal([]byte(aclJson), &currentNetworkACL); err != nil {
		return nil, err
	}
	if servercfg.CacheEnabled() {
		storeAclContainerInCache(containerID, currentNetworkACL)
	}
	return currentNetworkACL, nil
}

// fetchACLContainerJson - fetch the current ACL of given container except in json string
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

// upsertACLContainer - Inserts or updates a network ACL given the json string of the ACL and the container ID
// if nil, create it
func upsertACLContainer(containerID ContainerID, aclContainer ACLContainer) (ACLContainer, error) {
	AclMutex.Lock()
	defer AclMutex.Unlock()
	if aclContainer == nil {
		aclContainer = make(ACLContainer)
	}

	err := database.Insert(string(containerID), string(convertNetworkACLtoACLJson(aclContainer)), database.NODE_ACLS_TABLE_NAME)
	if err != nil {
		return aclContainer, err
	}
	if servercfg.CacheEnabled() {
		storeAclContainerInCache(containerID, aclContainer)
	}
	return aclContainer, nil
}

func convertNetworkACLtoACLJson(networkACL ACLContainer) ACLJson {
	data, err := json.Marshal(networkACL)
	if err != nil {
		return ""
	}
	return ACLJson(data)
}
