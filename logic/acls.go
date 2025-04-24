package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
)

/*
TODO: EGRESS
1. allow only selection of egress ranges in a policy
ranges should be replaced by egress identifier

2. check logic required for MAC exit node

3.

*/

var (
	aclCacheMutex = &sync.RWMutex{}
	aclCacheMap   = make(map[string]models.Acl)
)

func MigrateAclPolicies() {
	acls := ListAcls()
	for _, acl := range acls {
		if acl.Proto.String() == "" {
			acl.Proto = models.ALL
			acl.ServiceType = models.Any
			acl.Port = []string{}
			UpsertAcl(acl)
		}
	}

}

// CreateDefaultAclNetworkPolicies - create default acl network policies
func CreateDefaultAclNetworkPolicies(netID models.NetworkID) {
	if netID.String() == "" {
		return
	}
	_, _ = ListAclsByNetwork(netID)
	if !IsAclExists(fmt.Sprintf("%s.%s", netID, "all-nodes")) {
		defaultDeviceAcl := models.Acl{
			ID:          fmt.Sprintf("%s.%s", netID, "all-nodes"),
			Name:        "All Nodes",
			MetaData:    "This Policy allows all nodes in the network to communicate with each other",
			Default:     true,
			NetworkID:   netID,
			Proto:       models.ALL,
			ServiceType: models.Any,
			Port:        []string{},
			RuleType:    models.DevicePolicy,
			Src: []models.AclPolicyTag{
				{
					ID:    models.NodeTagID,
					Value: "*",
				}},
			Dst: []models.AclPolicyTag{
				{
					ID:    models.NodeTagID,
					Value: "*",
				}},
			AllowedDirection: models.TrafficDirectionBi,
			Enabled:          true,
			CreatedBy:        "auto",
			CreatedAt:        time.Now().UTC(),
		}
		InsertAcl(defaultDeviceAcl)
	}
	if !IsAclExists(fmt.Sprintf("%s.%s", netID, "all-users")) {
		defaultUserAcl := models.Acl{
			ID:          fmt.Sprintf("%s.%s", netID, "all-users"),
			Default:     true,
			Name:        "All Users",
			MetaData:    "This policy gives access to everything in the network for an user",
			NetworkID:   netID,
			Proto:       models.ALL,
			ServiceType: models.Any,
			Port:        []string{},
			RuleType:    models.UserPolicy,
			Src: []models.AclPolicyTag{
				{
					ID:    models.UserAclID,
					Value: "*",
				},
			},
			Dst: []models.AclPolicyTag{{
				ID:    models.NodeTagID,
				Value: "*",
			}},
			AllowedDirection: models.TrafficDirectionUni,
			Enabled:          true,
			CreatedBy:        "auto",
			CreatedAt:        time.Now().UTC(),
		}
		InsertAcl(defaultUserAcl)
	}

	if !IsAclExists(fmt.Sprintf("%s.%s", netID, "all-gateways")) {
		defaultUserAcl := models.Acl{
			ID:          fmt.Sprintf("%s.%s", netID, "all-gateways"),
			Default:     true,
			Name:        "All Gateways",
			NetworkID:   netID,
			Proto:       models.ALL,
			ServiceType: models.Any,
			Port:        []string{},
			RuleType:    models.DevicePolicy,
			Src: []models.AclPolicyTag{
				{
					ID:    models.NodeTagID,
					Value: fmt.Sprintf("%s.%s", netID, models.GwTagName),
				},
			},
			Dst: []models.AclPolicyTag{
				{
					ID:    models.NodeTagID,
					Value: "*",
				},
			},
			AllowedDirection: models.TrafficDirectionBi,
			Enabled:          true,
			CreatedBy:        "auto",
			CreatedAt:        time.Now().UTC(),
		}
		InsertAcl(defaultUserAcl)
	}
	CreateDefaultUserPolicies(netID)
}

// DeleteNetworkPolicies - deletes all default network acl policies
func DeleteNetworkPolicies(netId models.NetworkID) {
	acls, _ := ListAclsByNetwork(netId)
	for _, acl := range acls {
		if acl.NetworkID == netId {
			DeleteAcl(acl)
		}
	}
}

// ValidateCreateAclReq - validates create req for acl
func ValidateCreateAclReq(req models.Acl) error {
	// check if acl network exists
	_, err := GetNetwork(req.NetworkID.String())
	if err != nil {
		return errors.New("failed to get network details for " + req.NetworkID.String())
	}
	// err = CheckIDSyntax(req.Name)
	// if err != nil {
	// 	return err
	// }
	return nil
}

func listAclFromCache() (acls []models.Acl) {
	aclCacheMutex.RLock()
	defer aclCacheMutex.RUnlock()
	for _, acl := range aclCacheMap {
		acls = append(acls, acl)
	}
	return
}

func storeAclInCache(a models.Acl) {
	aclCacheMutex.Lock()
	defer aclCacheMutex.Unlock()
	aclCacheMap[a.ID] = a

}

func removeAclFromCache(a models.Acl) {
	aclCacheMutex.Lock()
	defer aclCacheMutex.Unlock()
	delete(aclCacheMap, a.ID)
}

func getAclFromCache(aID string) (a models.Acl, ok bool) {
	aclCacheMutex.RLock()
	defer aclCacheMutex.RUnlock()
	a, ok = aclCacheMap[aID]
	return
}

// InsertAcl - creates acl policy
func InsertAcl(a models.Acl) error {
	d, err := json.Marshal(a)
	if err != nil {
		return err
	}
	err = database.Insert(a.ID, string(d), database.ACLS_TABLE_NAME)
	if err == nil && servercfg.CacheEnabled() {
		storeAclInCache(a)
	}
	return err
}

// GetAcl - gets acl info by id
func GetAcl(aID string) (models.Acl, error) {
	a := models.Acl{}
	if servercfg.CacheEnabled() {
		var ok bool
		a, ok = getAclFromCache(aID)
		if ok {
			return a, nil
		}
	}
	d, err := database.FetchRecord(database.ACLS_TABLE_NAME, aID)
	if err != nil {
		return a, err
	}
	err = json.Unmarshal([]byte(d), &a)
	if err != nil {
		return a, err
	}
	if servercfg.CacheEnabled() {
		storeAclInCache(a)
	}
	return a, nil
}

// IsAclExists - checks if acl exists
func IsAclExists(aclID string) bool {
	_, err := GetAcl(aclID)
	return err == nil
}
func GetEgressRanges(netID models.NetworkID) (map[string][]string, map[string]struct{}, error) {

	resultMap := make(map[string]struct{})
	nodeEgressMap := make(map[string][]string)
	networkNodes, err := GetNetworkNodes(netID.String())
	if err != nil {
		return nil, nil, err
	}
	for _, currentNode := range networkNodes {
		if currentNode.Network != netID.String() {
			continue
		}
		if currentNode.IsEgressGateway { // add the egress gateway range(s) to the result
			if len(currentNode.EgressGatewayRanges) > 0 {
				nodeEgressMap[currentNode.ID.String()] = currentNode.EgressGatewayRanges
				for _, egressRangeI := range currentNode.EgressGatewayRanges {
					resultMap[egressRangeI] = struct{}{}
				}
			}
		}
	}
	extclients, _ := GetNetworkExtClients(netID.String())
	for _, extclient := range extclients {
		if len(extclient.ExtraAllowedIPs) > 0 {
			nodeEgressMap[extclient.ClientID] = extclient.ExtraAllowedIPs
			for _, extraAllowedIP := range extclient.ExtraAllowedIPs {
				resultMap[extraAllowedIP] = struct{}{}
			}
		}
	}
	return nodeEgressMap, resultMap, nil
}

func checkIfAclTagisValid(t models.AclPolicyTag, netID models.NetworkID, policyType models.AclPolicyType, isSrc bool) bool {
	switch t.ID {
	case models.NodeTagID:
		if policyType == models.UserPolicy && isSrc {
			return false
		}
		// check if tag is valid
		_, err := GetTag(models.TagID(t.Value))
		if err != nil {
			return false
		}
	case models.NodeID:
		if policyType == models.UserPolicy && isSrc {
			return false
		}
		_, nodeErr := GetNodeByID(t.Value)
		if nodeErr != nil {
			_, staticNodeErr := GetExtClient(t.Value, netID.String())
			if staticNodeErr != nil {
				return false
			}
		}
	case models.EgressID:
		e := schema.Egress{
			ID: t.Value,
		}
		err := e.Get(db.WithContext(context.TODO()))
		if err != nil {
			return false
		}

	case models.UserAclID:
		if policyType == models.DevicePolicy {
			return false
		}
		if !isSrc {
			return false
		}
		_, err := GetUser(t.Value)
		if err != nil {
			return false
		}
	case models.UserGroupAclID:
		if policyType == models.DevicePolicy {
			return false
		}
		if !isSrc {
			return false
		}
		err := IsGroupValid(models.UserGroupID(t.Value))
		if err != nil {
			return false
		}
		// check if group belongs to this network
		netGrps := GetUserGroupsInNetwork(netID)
		if _, ok := netGrps[models.UserGroupID(t.Value)]; !ok {
			return false
		}
	default:
		return false
	}
	return true
}

// IsAclPolicyValid - validates if acl policy is valid
func IsAclPolicyValid(acl models.Acl) bool {
	//check if src and dst are valid
	if acl.AllowedDirection != models.TrafficDirectionBi &&
		acl.AllowedDirection != models.TrafficDirectionUni {
		return false
	}
	switch acl.RuleType {
	case models.UserPolicy:
		// src list should only contain users
		for _, srcI := range acl.Src {

			if srcI.Value == "*" {
				continue
			}
			// check if user group is valid
			if !checkIfAclTagisValid(srcI, acl.NetworkID, acl.RuleType, true) {
				return false
			}
		}
		for _, dstI := range acl.Dst {

			if dstI.Value == "*" {
				continue
			}

			// check if user group is valid
			if !checkIfAclTagisValid(dstI, acl.NetworkID, acl.RuleType, false) {
				return false
			}
		}
	case models.DevicePolicy:
		for _, srcI := range acl.Src {
			if srcI.Value == "*" {
				continue
			}
			// check if user group is valid
			if !checkIfAclTagisValid(srcI, acl.NetworkID, acl.RuleType, true) {
				return false
			}
		}
		for _, dstI := range acl.Dst {

			if dstI.Value == "*" {
				continue
			}
			// check if user group is valid
			if !checkIfAclTagisValid(dstI, acl.NetworkID, acl.RuleType, false) {
				return false
			}
		}
	}
	return true
}

func UniqueAclPolicyTags(tags []models.AclPolicyTag) []models.AclPolicyTag {
	seen := make(map[string]bool)
	var result []models.AclPolicyTag

	for _, tag := range tags {
		key := fmt.Sprintf("%v-%s", tag.ID, tag.Value)
		if !seen[key] {
			seen[key] = true
			result = append(result, tag)
		}
	}
	return result
}

// UpdateAcl - updates allowed fields on acls and commits to DB
func UpdateAcl(newAcl, acl models.Acl) error {
	if !acl.Default {
		acl.Name = newAcl.Name
		acl.Src = newAcl.Src
		acl.Dst = newAcl.Dst
		acl.AllowedDirection = newAcl.AllowedDirection
		acl.Port = newAcl.Port
		acl.Proto = newAcl.Proto
		acl.ServiceType = newAcl.ServiceType
	}
	if newAcl.ServiceType == models.Any {
		acl.Port = []string{}
		acl.Proto = models.ALL
	}
	acl.Enabled = newAcl.Enabled
	d, err := json.Marshal(acl)
	if err != nil {
		return err
	}
	err = database.Insert(acl.ID, string(d), database.ACLS_TABLE_NAME)
	if err == nil && servercfg.CacheEnabled() {
		storeAclInCache(acl)
	}
	return err
}

// UpsertAcl - upserts acl
func UpsertAcl(acl models.Acl) error {
	d, err := json.Marshal(acl)
	if err != nil {
		return err
	}
	err = database.Insert(acl.ID, string(d), database.ACLS_TABLE_NAME)
	if err == nil && servercfg.CacheEnabled() {
		storeAclInCache(acl)
	}
	return err
}

// DeleteAcl - deletes acl policy
func DeleteAcl(a models.Acl) error {
	err := database.DeleteRecord(database.ACLS_TABLE_NAME, a.ID)
	if err == nil && servercfg.CacheEnabled() {
		removeAclFromCache(a)
	}
	return err
}

// GetDefaultPolicy - fetches default policy in the network by ruleType
func GetDefaultPolicy(netID models.NetworkID, ruleType models.AclPolicyType) (models.Acl, error) {
	aclID := "all-users"
	if ruleType == models.DevicePolicy {
		aclID = "all-nodes"
	}
	acl, err := GetAcl(fmt.Sprintf("%s.%s", netID, aclID))
	if err != nil {
		return models.Acl{}, errors.New("default rule not found")
	}
	if acl.Enabled {
		return acl, nil
	}
	// check if there are any custom all policies
	srcMap := make(map[string]struct{})
	dstMap := make(map[string]struct{})
	defer func() {
		srcMap = nil
		dstMap = nil
	}()
	policies, _ := ListAclsByNetwork(netID)
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		if policy.RuleType == ruleType {
			dstMap = convAclTagToValueMap(policy.Dst)
			srcMap = convAclTagToValueMap(policy.Src)
			if _, ok := srcMap["*"]; ok {
				if _, ok := dstMap["*"]; ok {
					return policy, nil
				}
			}
		}

	}

	return acl, nil
}

func ListAcls() (acls []models.Acl) {
	if servercfg.CacheEnabled() && len(aclCacheMap) > 0 {
		return listAclFromCache()
	}

	data, err := database.FetchRecords(database.ACLS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return []models.Acl{}
	}
	for _, dataI := range data {
		acl := models.Acl{}
		err := json.Unmarshal([]byte(dataI), &acl)
		if err != nil {
			continue
		}
		acls = append(acls, acl)
		if servercfg.CacheEnabled() {
			storeAclInCache(acl)
		}
	}
	return
}

// ListUserPolicies - lists all acl policies enforced on an user
func ListUserPolicies(u models.User) []models.Acl {
	allAcls := ListAcls()
	userAcls := []models.Acl{}
	for _, acl := range allAcls {

		if acl.RuleType == models.UserPolicy {
			srcMap := convAclTagToValueMap(acl.Src)
			if _, ok := srcMap[u.UserName]; ok {
				userAcls = append(userAcls, acl)
			} else {
				// check for user groups
				for gID := range u.UserGroups {
					if _, ok := srcMap[gID.String()]; ok {
						userAcls = append(userAcls, acl)
						break
					}
				}
			}

		}
	}
	return userAcls
}

// listPoliciesOfUser - lists all user acl policies applied to user in an network
func listPoliciesOfUser(user models.User, netID models.NetworkID) []models.Acl {
	allAcls := ListAcls()
	userAcls := []models.Acl{}
	for _, acl := range allAcls {
		if acl.NetworkID == netID && acl.RuleType == models.UserPolicy {
			srcMap := convAclTagToValueMap(acl.Src)
			if _, ok := srcMap[user.UserName]; ok {
				userAcls = append(userAcls, acl)
				continue
			}
			for netRole := range user.NetworkRoles {
				if _, ok := srcMap[netRole.String()]; ok {
					userAcls = append(userAcls, acl)
					continue
				}
			}
			for userG := range user.UserGroups {
				if _, ok := srcMap[userG.String()]; ok {
					userAcls = append(userAcls, acl)
					continue
				}
			}

		}
	}
	return userAcls
}

// listDevicePolicies - lists all device policies in a network
func listDevicePolicies(netID models.NetworkID) []models.Acl {
	allAcls := ListAcls()
	deviceAcls := []models.Acl{}
	for _, acl := range allAcls {
		if acl.NetworkID == netID && acl.RuleType == models.DevicePolicy {
			deviceAcls = append(deviceAcls, acl)
		}
	}
	return deviceAcls
}

// listUserPolicies - lists all user policies in a network
func listUserPolicies(netID models.NetworkID) []models.Acl {
	allAcls := ListAcls()
	deviceAcls := []models.Acl{}
	for _, acl := range allAcls {
		if acl.NetworkID == netID && acl.RuleType == models.UserPolicy {
			deviceAcls = append(deviceAcls, acl)
		}
	}
	return deviceAcls
}

// ListAcls - lists all acl policies
func ListAclsByNetwork(netID models.NetworkID) ([]models.Acl, error) {

	allAcls := ListAcls()
	netAcls := []models.Acl{}
	for _, acl := range allAcls {
		if acl.NetworkID == netID {
			netAcls = append(netAcls, acl)
		}
	}
	return netAcls, nil
}

func convAclTagToValueMap(acltags []models.AclPolicyTag) map[string]struct{} {
	aclValueMap := make(map[string]struct{})
	for _, aclTagI := range acltags {
		aclValueMap[aclTagI.Value] = struct{}{}
	}
	return aclValueMap
}

// IsUserAllowedToCommunicate - check if user is allowed to communicate with peer
func IsUserAllowedToCommunicate(userName string, peer models.Node) (bool, []models.Acl) {
	var peerId string
	if peer.IsStatic {
		peerId = peer.StaticNode.ClientID
		peer = peer.StaticNode.ConvertToStaticNode()
	} else {
		peerId = peer.ID.String()
	}

	var peerTags map[models.TagID]struct{}
	if peer.Mutex != nil {
		peer.Mutex.Lock()
		peerTags = maps.Clone(peer.Tags)
		peer.Mutex.Unlock()
	} else {
		peerTags = peer.Tags
	}
	peerTags[models.TagID(peerId)] = struct{}{}
	acl, _ := GetDefaultPolicy(models.NetworkID(peer.Network), models.UserPolicy)
	if acl.Enabled {
		return true, []models.Acl{acl}
	}
	user, err := GetUser(userName)
	if err != nil {
		return false, []models.Acl{}
	}
	allowedPolicies := []models.Acl{}
	policies := listPoliciesOfUser(*user, models.NetworkID(peer.Network))
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		dstMap := convAclTagToValueMap(policy.Dst)
		if _, ok := dstMap["*"]; ok {
			allowedPolicies = append(allowedPolicies, policy)
			continue
		}
		if _, ok := dstMap[peer.ID.String()]; ok {
			allowedPolicies = append(allowedPolicies, policy)
			continue
		}
		for tagID := range peerTags {
			if _, ok := dstMap[tagID.String()]; ok {
				allowedPolicies = append(allowedPolicies, policy)
				break
			}
		}

	}
	if len(allowedPolicies) > 0 {
		return true, allowedPolicies
	}
	return false, []models.Acl{}
}

// IsPeerAllowed - checks if peer needs to be added to the interface
func IsPeerAllowed(node, peer models.Node, checkDefaultPolicy bool) bool {
	var nodeId, peerId string
	if peer.IsFailOver && node.FailedOverBy != uuid.Nil && node.FailedOverBy == peer.ID {
		return true
	}
	if node.IsFailOver && peer.FailedOverBy != uuid.Nil && peer.FailedOverBy == node.ID {
		return true
	}
	if node.IsStatic {
		nodeId = node.StaticNode.ClientID
		node = node.StaticNode.ConvertToStaticNode()
	} else {
		nodeId = node.ID.String()
	}
	if peer.IsStatic {
		peerId = peer.StaticNode.ClientID
		peer = peer.StaticNode.ConvertToStaticNode()
	} else {
		peerId = peer.ID.String()
	}

	var nodeTags, peerTags map[models.TagID]struct{}
	if node.Mutex != nil {
		node.Mutex.Lock()
		nodeTags = maps.Clone(node.Tags)
		node.Mutex.Unlock()
	} else {
		nodeTags = node.Tags
	}
	if peer.Mutex != nil {
		peer.Mutex.Lock()
		peerTags = maps.Clone(peer.Tags)
		peer.Mutex.Unlock()
	} else {
		peerTags = peer.Tags
	}
	if nodeTags == nil {
		nodeTags = make(map[models.TagID]struct{})
	}
	if peerTags == nil {
		peerTags = make(map[models.TagID]struct{})
	}
	nodeTags[models.TagID(nodeId)] = struct{}{}
	peerTags[models.TagID(peerId)] = struct{}{}
	if checkDefaultPolicy {
		// check default policy if all allowed return true
		defaultPolicy, err := GetDefaultPolicy(models.NetworkID(node.Network), models.DevicePolicy)
		if err == nil {
			if defaultPolicy.Enabled {
				return true
			}
		}

	}
	// list device policies
	policies := listDevicePolicies(models.NetworkID(peer.Network))
	srcMap := make(map[string]struct{})
	dstMap := make(map[string]struct{})
	defer func() {
		srcMap = nil
		dstMap = nil
	}()
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}

		srcMap = convAclTagToValueMap(policy.Src)
		dstMap = convAclTagToValueMap(policy.Dst)
		for _, dst := range policy.Dst {
			if dst.ID == models.EgressID {
				e := schema.Egress{ID: dst.Value}
				err := e.Get(db.WithContext(context.TODO()))
				if err == nil {
					for nodeID := range e.Nodes {
						dstMap[nodeID] = struct{}{}
					}
				}
			}
		}
		if checkTagGroupPolicy(srcMap, dstMap, node, peer, nodeTags, peerTags) {
			return true
		}

	}
	return false
}

func RemoveUserFromAclPolicy(userName string) {
	acls := ListAcls()
	for _, acl := range acls {
		delete := false
		update := false
		if acl.RuleType == models.UserPolicy {
			for i := len(acl.Src) - 1; i >= 0; i-- {
				if acl.Src[i].ID == models.UserAclID && acl.Src[i].Value == userName {
					if len(acl.Src) == 1 {
						// delete policy
						delete = true
						break
					} else {
						acl.Src = append(acl.Src[:i], acl.Src[i+1:]...)
						update = true
					}
				}
			}
			if delete {
				DeleteAcl(acl)
				continue
			}
			if update {
				UpsertAcl(acl)
			}
		}
	}
}

func RemoveNodeFromAclPolicy(node models.Node) {
	var nodeID string
	if node.IsStatic {
		nodeID = node.StaticNode.ClientID
	} else {
		nodeID = node.ID.String()
	}
	acls, _ := ListAclsByNetwork(models.NetworkID(node.Network))
	for _, acl := range acls {
		delete := false
		update := false
		if acl.RuleType == models.DevicePolicy {
			for i := len(acl.Src) - 1; i >= 0; i-- {
				if acl.Src[i].ID == models.NodeID && acl.Src[i].Value == nodeID {
					if len(acl.Src) == 1 {
						// delete policy
						delete = true
						break
					} else {
						acl.Src = append(acl.Src[:i], acl.Src[i+1:]...)
						update = true
					}
				}
			}
			if delete {
				DeleteAcl(acl)
				continue
			}
			for i := len(acl.Dst) - 1; i >= 0; i-- {
				if acl.Dst[i].ID == models.NodeID && acl.Dst[i].Value == nodeID {
					if len(acl.Dst) == 1 {
						// delete policy
						delete = true
						break
					} else {
						acl.Dst = append(acl.Dst[:i], acl.Dst[i+1:]...)
						update = true
					}
				}
			}
			if delete {
				DeleteAcl(acl)
				continue
			}
			if update {
				UpsertAcl(acl)
			}

		}
		if acl.RuleType == models.UserPolicy {
			for i := len(acl.Dst) - 1; i >= 0; i-- {
				if acl.Dst[i].ID == models.NodeID && acl.Dst[i].Value == nodeID {
					if len(acl.Dst) == 1 {
						// delete policy
						delete = true
						break
					} else {
						acl.Dst = append(acl.Dst[:i], acl.Dst[i+1:]...)
						update = true
					}
				}
			}
			if delete {
				DeleteAcl(acl)
				continue
			}
			if update {
				UpsertAcl(acl)
			}
		}
	}
}

func checkTagGroupPolicy(srcMap, dstMap map[string]struct{}, node, peer models.Node,
	nodeTags, peerTags map[models.TagID]struct{}) bool {
	// check for node ID
	if _, ok := srcMap[node.ID.String()]; ok {
		if _, ok = dstMap[peer.ID.String()]; ok {
			return true
		}

	}
	if _, ok := dstMap[node.ID.String()]; ok {
		if _, ok = srcMap[peer.ID.String()]; ok {
			return true
		}
	}

	for tagID := range nodeTags {
		if _, ok := dstMap[tagID.String()]; ok {
			if _, ok := srcMap["*"]; ok {
				return true
			}
			for tagID := range peerTags {
				if _, ok := srcMap[tagID.String()]; ok {
					return true
				}
			}
		}
		if _, ok := srcMap[tagID.String()]; ok {
			if _, ok := dstMap["*"]; ok {
				return true
			}
			for tagID := range peerTags {
				if _, ok := dstMap[tagID.String()]; ok {
					return true
				}
			}
		}
	}
	for tagID := range peerTags {
		if _, ok := dstMap[tagID.String()]; ok {
			if _, ok := srcMap["*"]; ok {
				return true
			}
			for tagID := range nodeTags {

				if _, ok := srcMap[tagID.String()]; ok {
					return true
				}
			}
		}
		if _, ok := srcMap[tagID.String()]; ok {
			if _, ok := dstMap["*"]; ok {
				return true
			}
			for tagID := range nodeTags {
				if _, ok := dstMap[tagID.String()]; ok {
					return true
				}
			}
		}
	}
	return false
}
func uniquePolicies(items []models.Acl) []models.Acl {
	if len(items) == 0 {
		return items
	}
	seen := make(map[string]bool)
	var result []models.Acl
	for _, item := range items {
		if !seen[item.ID] {
			seen[item.ID] = true
			result = append(result, item)
		}
	}

	return result
}

// IsNodeAllowedToCommunicate - check node is allowed to communicate with the peer // ADD ALLOWED DIRECTION - 0 => node -> peer, 1 => peer-> node,
func IsNodeAllowedToCommunicateV1(node, peer models.Node, checkDefaultPolicy bool) (bool, []models.Acl) {
	var nodeId, peerId string
	if peer.IsFailOver && node.FailedOverBy != uuid.Nil && node.FailedOverBy == peer.ID {
		return true, []models.Acl{}
	}
	if node.IsFailOver && peer.FailedOverBy != uuid.Nil && peer.FailedOverBy == node.ID {
		return true, []models.Acl{}
	}
	if node.IsStatic {
		nodeId = node.StaticNode.ClientID
		node = node.StaticNode.ConvertToStaticNode()
	} else {
		nodeId = node.ID.String()
	}
	if peer.IsStatic {
		peerId = peer.StaticNode.ClientID
		peer = peer.StaticNode.ConvertToStaticNode()
	} else {
		peerId = peer.ID.String()
	}

	var nodeTags, peerTags map[models.TagID]struct{}
	if node.Mutex != nil {
		node.Mutex.Lock()
		nodeTags = maps.Clone(node.Tags)
		node.Mutex.Unlock()
	} else {
		nodeTags = node.Tags
	}
	if peer.Mutex != nil {
		peer.Mutex.Lock()
		peerTags = maps.Clone(peer.Tags)
		peer.Mutex.Unlock()
	} else {
		peerTags = peer.Tags
	}
	if nodeTags == nil {
		nodeTags = make(map[models.TagID]struct{})
	}
	if peerTags == nil {
		peerTags = make(map[models.TagID]struct{})
	}
	nodeTags[models.TagID(nodeId)] = struct{}{}
	peerTags[models.TagID(peerId)] = struct{}{}
	if checkDefaultPolicy {
		// check default policy if all allowed return true
		defaultPolicy, err := GetDefaultPolicy(models.NetworkID(node.Network), models.DevicePolicy)
		if err == nil {
			if defaultPolicy.Enabled {
				return true, []models.Acl{defaultPolicy}
			}
		}
	}
	allowedPolicies := []models.Acl{}
	defer func() {
		allowedPolicies = uniquePolicies(allowedPolicies)
	}()
	// list device policies
	policies := listDevicePolicies(models.NetworkID(peer.Network))
	srcMap := make(map[string]struct{})
	dstMap := make(map[string]struct{})
	defer func() {
		srcMap = nil
		dstMap = nil
	}()
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		allowed := false
		srcMap = convAclTagToValueMap(policy.Src)
		dstMap = convAclTagToValueMap(policy.Dst)
		for _, dst := range policy.Dst {
			if dst.ID == models.EgressID {
				e := schema.Egress{ID: dst.Value}
				err := e.Get(db.WithContext(context.TODO()))
				if err == nil {
					for nodeID := range e.Nodes {
						dstMap[nodeID] = struct{}{}
					}
				}
			}
		}
		_, srcAll := srcMap["*"]
		_, dstAll := dstMap["*"]
		if policy.AllowedDirection == models.TrafficDirectionBi {
			if _, ok := srcMap[nodeId]; ok || srcAll {
				if _, ok := dstMap[peerId]; ok || dstAll {
					allowedPolicies = append(allowedPolicies, policy)
					continue
				}

			}
			if _, ok := dstMap[nodeId]; ok || dstAll {
				if _, ok := srcMap[peerId]; ok || srcAll {
					allowedPolicies = append(allowedPolicies, policy)
					continue
				}
			}
		}
		if _, ok := dstMap[peerId]; ok || dstAll {
			if _, ok := srcMap[nodeId]; ok || srcAll {
				allowedPolicies = append(allowedPolicies, policy)
				continue
			}
		}
		if policy.AllowedDirection == models.TrafficDirectionBi {

			for tagID := range nodeTags {

				if _, ok := dstMap[tagID.String()]; ok || dstAll {
					if srcAll {
						allowed = true
						break
					}
					for tagID := range peerTags {
						if _, ok := srcMap[tagID.String()]; ok {
							allowed = true
							break
						}
					}
				}
				if allowed {
					allowedPolicies = append(allowedPolicies, policy)
					break
				}
				if _, ok := srcMap[tagID.String()]; ok || srcAll {
					if dstAll {
						allowed = true
						break
					}
					for tagID := range peerTags {
						if _, ok := dstMap[tagID.String()]; ok {
							allowed = true
							break
						}
					}
				}
				if allowed {
					break
				}
			}
			if allowed {
				allowedPolicies = append(allowedPolicies, policy)
				continue
			}
		}
		for tagID := range peerTags {
			if _, ok := dstMap[tagID.String()]; ok || dstAll {
				if srcAll {
					allowed = true
					break
				}
				for tagID := range nodeTags {
					if _, ok := srcMap[tagID.String()]; ok {
						allowed = true
						break
					}
				}
			}
			if allowed {
				break
			}
		}
		if allowed {
			allowedPolicies = append(allowedPolicies, policy)
		}
	}

	if len(allowedPolicies) > 0 {
		return true, allowedPolicies
	}
	return false, allowedPolicies
}

// SortTagEntrys - Sorts slice of Tag entries by their id
func SortAclEntrys(acls []models.Acl) {
	sort.Slice(acls, func(i, j int) bool {
		return acls[i].Name < acls[j].Name
	})
}

// UpdateDeviceTag - updates device tag on acl policies
func UpdateDeviceTag(OldID, newID models.TagID, netID models.NetworkID) {
	acls := listDevicePolicies(netID)
	update := false
	for _, acl := range acls {
		for i, srcTagI := range acl.Src {
			if srcTagI.ID == models.NodeTagID {
				if OldID.String() == srcTagI.Value {
					acl.Src[i].Value = newID.String()
					update = true
				}
			}
		}
		for i, dstTagI := range acl.Dst {
			if dstTagI.ID == models.NodeTagID {
				if OldID.String() == dstTagI.Value {
					acl.Dst[i].Value = newID.String()
					update = true
				}
			}
		}
		if update {
			UpsertAcl(acl)
		}
	}
}

func CheckIfTagAsActivePolicy(tagID models.TagID, netID models.NetworkID) bool {
	acls := listDevicePolicies(netID)
	for _, acl := range acls {
		for _, srcTagI := range acl.Src {
			if srcTagI.ID == models.NodeTagID {
				if tagID.String() == srcTagI.Value {
					return true
				}
			}
		}
		for _, dstTagI := range acl.Dst {
			if dstTagI.ID == models.NodeTagID {
				if tagID.String() == dstTagI.Value {
					return true
				}
			}
		}
	}
	return false
}

// RemoveDeviceTagFromAclPolicies - remove device tag from acl policies
func RemoveDeviceTagFromAclPolicies(tagID models.TagID, netID models.NetworkID) error {
	acls := listDevicePolicies(netID)
	update := false
	for _, acl := range acls {
		for i := len(acl.Src) - 1; i >= 0; i-- {
			if acl.Src[i].ID == models.NodeTagID {
				if tagID.String() == acl.Src[i].Value {
					acl.Src = append(acl.Src[:i], acl.Src[i+1:]...)
					update = true
				}
			}
		}
		for i := len(acl.Dst) - 1; i >= 0; i-- {
			if acl.Dst[i].ID == models.NodeTagID {
				if tagID.String() == acl.Dst[i].Value {
					acl.Dst = append(acl.Dst[:i], acl.Dst[i+1:]...)
					update = true
				}
			}
		}
		if update {
			UpsertAcl(acl)
		}
	}
	return nil
}

func getEgressUserRulesForNode(targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	userNodes := GetStaticUserNodesByNetwork(models.NetworkID(targetnode.Network))
	userGrpMap := GetUserGrpMap()
	allowedUsers := make(map[string][]models.Acl)
	acls := listUserPolicies(models.NetworkID(targetnode.Network))
	var targetNodeTags = make(map[models.TagID]struct{})
	targetNodeTags["*"] = struct{}{}
	for _, rangeI := range targetnode.EgressGatewayRanges {
		targetNodeTags[models.TagID(rangeI)] = struct{}{}
	}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		dstTags := convAclTagToValueMap(acl.Dst)
		_, all := dstTags["*"]
		addUsers := false
		if !all {
			for nodeTag := range targetNodeTags {
				if _, ok := dstTags[nodeTag.String()]; ok {
					addUsers = true
					break
				}
			}
		} else {
			addUsers = true
		}

		if addUsers {
			// get all src tags
			for _, srcAcl := range acl.Src {
				if srcAcl.ID == models.UserAclID {
					allowedUsers[srcAcl.Value] = append(allowedUsers[srcAcl.Value], acl)
				} else if srcAcl.ID == models.UserGroupAclID {
					// fetch all users in the group
					if usersMap, ok := userGrpMap[models.UserGroupID(srcAcl.Value)]; ok {
						for userName := range usersMap {
							allowedUsers[userName] = append(allowedUsers[userName], acl)
						}
					}
				}
			}
		}

	}

	for _, userNode := range userNodes {
		if !userNode.StaticNode.Enabled {
			continue
		}
		acls, ok := allowedUsers[userNode.StaticNode.OwnerID]
		if !ok {
			continue
		}
		for _, acl := range acls {

			if !acl.Enabled {
				continue
			}
			r := models.AclRule{
				ID:              acl.ID,
				AllowedProtocol: acl.Proto,
				AllowedPorts:    acl.Port,
				Direction:       acl.AllowedDirection,
				Allowed:         true,
			}
			// Get peers in the tags and add allowed rules
			if userNode.StaticNode.Address != "" {
				r.IPList = append(r.IPList, userNode.StaticNode.AddressIPNet4())
			}
			if userNode.StaticNode.Address6 != "" {
				r.IP6List = append(r.IP6List, userNode.StaticNode.AddressIPNet6())
			}
			for _, dstI := range acl.Dst {
				if dstI.ID == models.EgressID {
					e := schema.Egress{ID: dstI.Value}
					err := e.Get(db.WithContext(context.TODO()))
					if err != nil {
						continue
					}
					if e.IsInetGw {
						r.Dst = append(r.Dst, net.IPNet{
							IP:   net.IPv4zero,
							Mask: net.CIDRMask(0, 32),
						})
						r.Dst6 = append(r.Dst6, net.IPNet{
							IP:   net.IPv6zero,
							Mask: net.CIDRMask(0, 128),
						})

					} else {
						ip, cidr, err := net.ParseCIDR(e.Range)
						if err == nil {
							if ip.To4() != nil {
								r.Dst = append(r.Dst, *cidr)
							} else {
								r.Dst6 = append(r.Dst6, *cidr)
							}

						}
					}

				}

			}
			if aclRule, ok := rules[acl.ID]; ok {
				aclRule.IPList = append(aclRule.IPList, r.IPList...)
				aclRule.IP6List = append(aclRule.IP6List, r.IP6List...)
				rules[acl.ID] = aclRule
			} else {
				rules[acl.ID] = r
			}
		}
	}
	return rules
}

func getUserAclRulesForNode(targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	userNodes := GetStaticUserNodesByNetwork(models.NetworkID(targetnode.Network))
	userGrpMap := GetUserGrpMap()
	allowedUsers := make(map[string][]models.Acl)
	acls := listUserPolicies(models.NetworkID(targetnode.Network))
	var targetNodeTags = make(map[models.TagID]struct{})
	if targetnode.Mutex != nil {
		targetnode.Mutex.Lock()
		targetNodeTags = maps.Clone(targetnode.Tags)
		targetnode.Mutex.Unlock()
	} else {
		targetNodeTags = maps.Clone(targetnode.Tags)
	}
	if targetNodeTags == nil {
		targetNodeTags = make(map[models.TagID]struct{})
	}
	targetNodeTags[models.TagID(targetnode.ID.String())] = struct{}{}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		dstTags := convAclTagToValueMap(acl.Dst)
		_, all := dstTags["*"]
		addUsers := false
		if !all {
			for nodeTag := range targetNodeTags {
				if _, ok := dstTags[nodeTag.String()]; ok {
					addUsers = true
					break
				}
			}
		} else {
			addUsers = true
		}

		if addUsers {
			// get all src tags
			for _, srcAcl := range acl.Src {
				if srcAcl.ID == models.UserAclID {
					allowedUsers[srcAcl.Value] = append(allowedUsers[srcAcl.Value], acl)
				} else if srcAcl.ID == models.UserGroupAclID {
					// fetch all users in the group
					if usersMap, ok := userGrpMap[models.UserGroupID(srcAcl.Value)]; ok {
						for userName := range usersMap {
							allowedUsers[userName] = append(allowedUsers[userName], acl)
						}
					}
				}
			}
		}

	}

	for _, userNode := range userNodes {
		if !userNode.StaticNode.Enabled {
			continue
		}
		acls, ok := allowedUsers[userNode.StaticNode.OwnerID]
		if !ok {
			continue
		}
		for _, acl := range acls {

			if !acl.Enabled {
				continue
			}
			r := models.AclRule{
				ID:              acl.ID,
				AllowedProtocol: acl.Proto,
				AllowedPorts:    acl.Port,
				Direction:       acl.AllowedDirection,
				Allowed:         true,
			}
			// Get peers in the tags and add allowed rules
			if userNode.StaticNode.Address != "" {
				r.IPList = append(r.IPList, userNode.StaticNode.AddressIPNet4())
			}
			if userNode.StaticNode.Address6 != "" {
				r.IP6List = append(r.IP6List, userNode.StaticNode.AddressIPNet6())
			}
			if aclRule, ok := rules[acl.ID]; ok {
				aclRule.IPList = append(aclRule.IPList, r.IPList...)
				aclRule.IP6List = append(aclRule.IP6List, r.IP6List...)
				aclRule.IPList = UniqueIPNetList(aclRule.IPList)
				aclRule.IP6List = UniqueIPNetList(aclRule.IP6List)
				rules[acl.ID] = aclRule
			} else {
				r.IPList = UniqueIPNetList(r.IPList)
				r.IP6List = UniqueIPNetList(r.IP6List)
				rules[acl.ID] = r
			}
		}
	}
	return rules
}

func checkIfAnyActiveEgressPolicy(targetNode models.Node) bool {
	if !targetNode.IsEgressGateway {
		return false
	}
	var targetNodeTags = make(map[models.TagID]struct{})
	if targetNode.Mutex != nil {
		targetNode.Mutex.Lock()
		targetNodeTags = maps.Clone(targetNode.Tags)
		targetNode.Mutex.Unlock()
	} else {
		targetNodeTags = maps.Clone(targetNode.Tags)
	}
	if targetNodeTags == nil {
		targetNodeTags = make(map[models.TagID]struct{})
	}
	targetNodeTags[models.TagID(targetNode.ID.String())] = struct{}{}
	targetNodeTags["*"] = struct{}{}
	acls, _ := ListAclsByNetwork(models.NetworkID(targetNode.Network))
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcTags := convAclTagToValueMap(acl.Src)
		dstTags := convAclTagToValueMap(acl.Dst)
		for nodeTag := range targetNodeTags {
			if acl.RuleType == models.DevicePolicy {
				if _, ok := srcTags[nodeTag.String()]; ok {
					return true
				}
				if _, ok := srcTags[targetNode.ID.String()]; ok {
					return true
				}
			}

			if _, ok := dstTags[nodeTag.String()]; ok {
				return true
			}
			if _, ok := dstTags[targetNode.ID.String()]; ok {
				return true
			}
		}
	}
	return false
}

func checkIfAnyPolicyisUniDirectional(targetNode models.Node) bool {
	var targetNodeTags = make(map[models.TagID]struct{})
	if targetNode.Mutex != nil {
		targetNode.Mutex.Lock()
		targetNodeTags = maps.Clone(targetNode.Tags)
		targetNode.Mutex.Unlock()
	} else {
		targetNodeTags = maps.Clone(targetNode.Tags)
	}
	if targetNodeTags == nil {
		targetNodeTags = make(map[models.TagID]struct{})
	}
	targetNodeTags[models.TagID(targetNode.ID.String())] = struct{}{}
	targetNodeTags["*"] = struct{}{}
	acls, _ := ListAclsByNetwork(models.NetworkID(targetNode.Network))
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		if acl.AllowedDirection == models.TrafficDirectionBi {
			continue
		}
		if acl.Proto != models.ALL || acl.ServiceType != models.Any {
			return true
		}
		srcTags := convAclTagToValueMap(acl.Src)
		dstTags := convAclTagToValueMap(acl.Dst)
		for nodeTag := range targetNodeTags {
			if acl.RuleType == models.DevicePolicy {
				if _, ok := srcTags[nodeTag.String()]; ok {
					return true
				}
				if _, ok := srcTags[targetNode.ID.String()]; ok {
					return true
				}
			}

			if _, ok := dstTags[nodeTag.String()]; ok {
				return true
			}
			if _, ok := dstTags[targetNode.ID.String()]; ok {
				return true
			}
		}
	}
	return false
}

func GetAclRulesForNode(targetnodeI *models.Node) (rules map[string]models.AclRule) {
	targetnode := *targetnodeI
	defer func() {
		if !targetnode.IsIngressGateway {
			rules = getUserAclRulesForNode(&targetnode, rules)
		}
	}()
	rules = make(map[string]models.AclRule)
	var taggedNodes map[models.TagID][]models.Node
	if targetnode.IsIngressGateway {
		taggedNodes = GetTagMapWithNodesByNetwork(models.NetworkID(targetnode.Network), false)
	} else {
		taggedNodes = GetTagMapWithNodesByNetwork(models.NetworkID(targetnode.Network), true)
	}

	acls := listDevicePolicies(models.NetworkID(targetnode.Network))
	var targetNodeTags = make(map[models.TagID]struct{})
	if targetnode.Mutex != nil {
		targetnode.Mutex.Lock()
		targetNodeTags = maps.Clone(targetnode.Tags)
		targetnode.Mutex.Unlock()
	} else {
		targetNodeTags = maps.Clone(targetnode.Tags)
	}
	if targetNodeTags == nil {
		targetNodeTags = make(map[models.TagID]struct{})
	}
	targetNodeTags[models.TagID(targetnode.ID.String())] = struct{}{}
	targetNodeTags["*"] = struct{}{}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcTags := convAclTagToValueMap(acl.Src)
		dstTags := convAclTagToValueMap(acl.Dst)
		_, srcAll := srcTags["*"]
		_, dstAll := dstTags["*"]
		aclRule := models.AclRule{
			ID:              acl.ID,
			AllowedProtocol: acl.Proto,
			AllowedPorts:    acl.Port,
			Direction:       acl.AllowedDirection,
			Allowed:         true,
		}
		for nodeTag := range targetNodeTags {
			if acl.AllowedDirection == models.TrafficDirectionBi {
				var existsInSrcTag bool
				var existsInDstTag bool

				if _, ok := srcTags[nodeTag.String()]; ok || srcAll {
					existsInSrcTag = true
				}
				if _, ok := srcTags[targetnode.ID.String()]; ok || srcAll {
					existsInSrcTag = true
				}
				if _, ok := dstTags[nodeTag.String()]; ok || dstAll {
					existsInDstTag = true
				}
				if _, ok := dstTags[targetnode.ID.String()]; ok || dstAll {
					existsInDstTag = true
				}

				if existsInSrcTag && !existsInDstTag {
					// get all dst tags
					for dst := range dstTags {
						if dst == nodeTag.String() {
							continue
						}
						// Get peers in the tags and add allowed rules
						nodes := taggedNodes[models.TagID(dst)]
						if dst != targetnode.ID.String() {
							node, err := GetNodeByID(dst)
							if err == nil {
								nodes = append(nodes, node)
							}
						}

						for _, node := range nodes {
							if node.ID == targetnode.ID {
								continue
							}
							if node.IsStatic && node.StaticNode.IngressGatewayID == targetnode.ID.String() {
								continue
							}
							if node.Address.IP != nil {
								aclRule.IPList = append(aclRule.IPList, node.AddressIPNet4())
							}
							if node.Address6.IP != nil {
								aclRule.IP6List = append(aclRule.IP6List, node.AddressIPNet6())
							}
							if node.IsStatic && node.StaticNode.Address != "" {
								aclRule.IPList = append(aclRule.IPList, node.StaticNode.AddressIPNet4())
							}
							if node.IsStatic && node.StaticNode.Address6 != "" {
								aclRule.IP6List = append(aclRule.IP6List, node.StaticNode.AddressIPNet6())
							}
						}
					}
				}
				if existsInDstTag && !existsInSrcTag {
					// get all src tags
					for src := range srcTags {
						if src == nodeTag.String() {
							continue
						}
						// Get peers in the tags and add allowed rules
						nodes := taggedNodes[models.TagID(src)]
						if src != targetnode.ID.String() {
							node, err := GetNodeByID(src)
							if err == nil {
								nodes = append(nodes, node)
							}
						}
						for _, node := range nodes {
							if node.ID == targetnode.ID {
								continue
							}
							if node.IsStatic && node.StaticNode.IngressGatewayID == targetnode.ID.String() {
								continue
							}
							if node.Address.IP != nil {
								aclRule.IPList = append(aclRule.IPList, node.AddressIPNet4())
							}
							if node.Address6.IP != nil {
								aclRule.IP6List = append(aclRule.IP6List, node.AddressIPNet6())
							}
							if node.IsStatic && node.StaticNode.Address != "" {
								aclRule.IPList = append(aclRule.IPList, node.StaticNode.AddressIPNet4())
							}
							if node.IsStatic && node.StaticNode.Address6 != "" {
								aclRule.IP6List = append(aclRule.IP6List, node.StaticNode.AddressIPNet6())
							}
						}
					}
				}
				if existsInDstTag && existsInSrcTag {
					nodes := taggedNodes[nodeTag]
					for srcID := range srcTags {
						if srcID == targetnode.ID.String() {
							continue
						}
						node, err := GetNodeByID(srcID)
						if err == nil {
							nodes = append(nodes, node)
						}
					}
					for dstID := range dstTags {
						if dstID == targetnode.ID.String() {
							continue
						}
						node, err := GetNodeByID(dstID)
						if err == nil {
							nodes = append(nodes, node)
						}
					}
					for _, node := range nodes {
						if node.ID == targetnode.ID {
							continue
						}
						if node.IsStatic && node.StaticNode.IngressGatewayID == targetnode.ID.String() {
							continue
						}
						if node.Address.IP != nil {
							aclRule.IPList = append(aclRule.IPList, node.AddressIPNet4())
						}
						if node.Address6.IP != nil {
							aclRule.IP6List = append(aclRule.IP6List, node.AddressIPNet6())
						}
						if node.IsStatic && node.StaticNode.Address != "" {
							aclRule.IPList = append(aclRule.IPList, node.StaticNode.AddressIPNet4())
						}
						if node.IsStatic && node.StaticNode.Address6 != "" {
							aclRule.IP6List = append(aclRule.IP6List, node.StaticNode.AddressIPNet6())
						}
					}
				}
			} else {
				_, all := dstTags["*"]
				if _, ok := dstTags[nodeTag.String()]; ok || all {
					// get all src tags
					for src := range srcTags {
						if src == nodeTag.String() {
							continue
						}
						// Get peers in the tags and add allowed rules
						nodes := taggedNodes[models.TagID(src)]
						for _, node := range nodes {
							if node.ID == targetnode.ID {
								continue
							}
							if node.IsStatic && node.StaticNode.IngressGatewayID == targetnode.ID.String() {
								continue
							}
							if node.Address.IP != nil {
								aclRule.IPList = append(aclRule.IPList, node.AddressIPNet4())
							}
							if node.Address6.IP != nil {
								aclRule.IP6List = append(aclRule.IP6List, node.AddressIPNet6())
							}
							if node.IsStatic && node.StaticNode.Address != "" {
								aclRule.IPList = append(aclRule.IPList, node.StaticNode.AddressIPNet4())
							}
							if node.IsStatic && node.StaticNode.Address6 != "" {
								aclRule.IP6List = append(aclRule.IP6List, node.StaticNode.AddressIPNet6())
							}
						}
					}
				}
			}

		}

		if len(aclRule.IPList) > 0 || len(aclRule.IP6List) > 0 {
			aclRule.IPList = UniqueIPNetList(aclRule.IPList)
			aclRule.IP6List = UniqueIPNetList(aclRule.IP6List)
			rules[acl.ID] = aclRule
		}
	}
	return rules
}

func GetEgressRulesForNode(targetnode models.Node) (rules map[string]models.AclRule) {
	rules = make(map[string]models.AclRule)
	defer func() {
		rules = getEgressUserRulesForNode(&targetnode, rules)
	}()
	taggedNodes := GetTagMapWithNodesByNetwork(models.NetworkID(targetnode.Network), true)

	acls := listDevicePolicies(models.NetworkID(targetnode.Network))
	var targetNodeTags = make(map[models.TagID]struct{})
	targetNodeTags["*"] = struct{}{}

	/*
		 if target node is egress gateway
			if acl policy has egress route and it is present in target node egress ranges
			fetch all the nodes in that policy and add rules
	*/

	for _, rangeI := range targetnode.EgressGatewayRanges {
		targetNodeTags[models.TagID(rangeI)] = struct{}{}
	}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcTags := convAclTagToValueMap(acl.Src)
		dstTags := convAclTagToValueMap(acl.Dst)
		_, srcAll := srcTags["*"]
		_, dstAll := dstTags["*"]
		for nodeTag := range targetNodeTags {
			aclRule := models.AclRule{
				ID:              acl.ID,
				AllowedProtocol: acl.Proto,
				AllowedPorts:    acl.Port,
				Direction:       acl.AllowedDirection,
				Allowed:         true,
			}
			if nodeTag != "*" {
				ip, cidr, err := net.ParseCIDR(nodeTag.String())
				if err != nil {
					continue
				}
				if ip.To4() != nil {
					aclRule.Dst = append(aclRule.Dst, *cidr)
				} else {
					aclRule.Dst6 = append(aclRule.Dst6, *cidr)
				}

			} else {
				aclRule.Dst = append(aclRule.Dst, net.IPNet{
					IP:   net.IPv4zero,        // 0.0.0.0
					Mask: net.CIDRMask(0, 32), // /0 means match all IPv4
				})
				aclRule.Dst6 = append(aclRule.Dst6, net.IPNet{
					IP:   net.IPv6zero,         // ::
					Mask: net.CIDRMask(0, 128), // /0 means match all IPv6
				})
			}
			if acl.AllowedDirection == models.TrafficDirectionBi {
				var existsInSrcTag bool
				var existsInDstTag bool

				if _, ok := srcTags[nodeTag.String()]; ok || srcAll {
					existsInSrcTag = true
				}
				if _, ok := dstTags[nodeTag.String()]; ok || dstAll {
					existsInDstTag = true
				}

				if existsInSrcTag && !existsInDstTag {
					// get all dst tags
					for dst := range dstTags {
						if dst == nodeTag.String() {
							continue
						}
						// Get peers in the tags and add allowed rules
						nodes := taggedNodes[models.TagID(dst)]
						if dst != targetnode.ID.String() {
							node, err := GetNodeByID(dst)
							if err == nil {
								nodes = append(nodes, node)
							}
						}

						for _, node := range nodes {
							if node.ID == targetnode.ID {
								continue
							}
							if node.Address.IP != nil {
								aclRule.IPList = append(aclRule.IPList, node.AddressIPNet4())
							}
							if node.Address6.IP != nil {
								aclRule.IP6List = append(aclRule.IP6List, node.AddressIPNet6())
							}
							if node.IsStatic && node.StaticNode.Address != "" {
								aclRule.IPList = append(aclRule.IPList, node.StaticNode.AddressIPNet4())
							}
							if node.IsStatic && node.StaticNode.Address6 != "" {
								aclRule.IP6List = append(aclRule.IP6List, node.StaticNode.AddressIPNet6())
							}
						}
					}
				}
				if existsInDstTag && !existsInSrcTag {
					// get all src tags
					for src := range srcTags {
						if src == nodeTag.String() {
							continue
						}
						// Get peers in the tags and add allowed rules
						nodes := taggedNodes[models.TagID(src)]
						if src != targetnode.ID.String() {
							node, err := GetNodeByID(src)
							if err == nil {
								nodes = append(nodes, node)
							}
						}
						for _, node := range nodes {
							if node.ID == targetnode.ID {
								continue
							}
							if node.Address.IP != nil {
								aclRule.IPList = append(aclRule.IPList, node.AddressIPNet4())
							}
							if node.Address6.IP != nil {
								aclRule.IP6List = append(aclRule.IP6List, node.AddressIPNet6())
							}
							if node.IsStatic && node.StaticNode.Address != "" {
								aclRule.IPList = append(aclRule.IPList, node.StaticNode.AddressIPNet4())
							}
							if node.IsStatic && node.StaticNode.Address6 != "" {
								aclRule.IP6List = append(aclRule.IP6List, node.StaticNode.AddressIPNet6())
							}
						}
					}
				}
				if existsInDstTag && existsInSrcTag {
					nodes := taggedNodes[nodeTag]
					for srcID := range srcTags {
						if srcID == targetnode.ID.String() {
							continue
						}
						node, err := GetNodeByID(srcID)
						if err == nil {
							nodes = append(nodes, node)
						}
					}
					for dstID := range dstTags {
						if dstID == targetnode.ID.String() {
							continue
						}
						node, err := GetNodeByID(dstID)
						if err == nil {
							nodes = append(nodes, node)
						}
					}
					for _, node := range nodes {
						if node.ID == targetnode.ID {
							continue
						}
						if node.Address.IP != nil {
							aclRule.IPList = append(aclRule.IPList, node.AddressIPNet4())
						}
						if node.Address6.IP != nil {
							aclRule.IP6List = append(aclRule.IP6List, node.AddressIPNet6())
						}
						if node.IsStatic && node.StaticNode.Address != "" {
							aclRule.IPList = append(aclRule.IPList, node.StaticNode.AddressIPNet4())
						}
						if node.IsStatic && node.StaticNode.Address6 != "" {
							aclRule.IP6List = append(aclRule.IP6List, node.StaticNode.AddressIPNet6())
						}
					}
				}
			} else {
				_, all := dstTags["*"]
				if _, ok := dstTags[nodeTag.String()]; ok || all {
					// get all src tags
					for src := range srcTags {
						if src == nodeTag.String() {
							continue
						}
						// Get peers in the tags and add allowed rules
						nodes := taggedNodes[models.TagID(src)]
						for _, node := range nodes {
							if node.ID == targetnode.ID {
								continue
							}
							if node.Address.IP != nil {
								aclRule.IPList = append(aclRule.IPList, node.AddressIPNet4())
							}
							if node.Address6.IP != nil {
								aclRule.IP6List = append(aclRule.IP6List, node.AddressIPNet6())
							}
							if node.IsStatic && node.StaticNode.Address != "" {
								aclRule.IPList = append(aclRule.IPList, node.StaticNode.AddressIPNet4())
							}
							if node.IsStatic && node.StaticNode.Address6 != "" {
								aclRule.IP6List = append(aclRule.IP6List, node.StaticNode.AddressIPNet6())
							}
						}
					}
				}
			}
			if len(aclRule.IPList) > 0 || len(aclRule.IP6List) > 0 {
				aclRule.IPList = UniqueIPNetList(aclRule.IPList)
				aclRule.IP6List = UniqueIPNetList(aclRule.IP6List)
				rules[acl.ID] = aclRule
			}

		}

	}
	return
}

// Compare two IPs and return true if ip1 < ip2
func lessIP(ip1, ip2 net.IP) bool {
	ip1 = ip1.To16() // Ensure IPv4 is converted to IPv6-mapped format
	ip2 = ip2.To16()
	return string(ip1) < string(ip2)
}

// Sort by IP first, then by prefix length
func sortIPNets(ipNets []net.IPNet) {
	sort.Slice(ipNets, func(i, j int) bool {
		ip1, ip2 := ipNets[i].IP, ipNets[j].IP
		mask1, _ := ipNets[i].Mask.Size()
		mask2, _ := ipNets[j].Mask.Size()

		// Compare IPs first
		if ip1.Equal(ip2) {
			return mask1 < mask2 // If same IP, sort by subnet mask size
		}
		return lessIP(ip1, ip2)
	})
}

func UniqueIPNetList(ipnets []net.IPNet) []net.IPNet {
	uniqueMap := make(map[string]net.IPNet)

	for _, ipnet := range ipnets {
		key := ipnet.String() // Uses CIDR notation as a unique key
		if _, exists := uniqueMap[key]; !exists {
			uniqueMap[key] = ipnet
		}
	}

	// Convert map back to slice
	uniqueList := make([]net.IPNet, 0, len(uniqueMap))
	for _, ipnet := range uniqueMap {
		uniqueList = append(uniqueList, ipnet)
	}
	sortIPNets(uniqueList)
	return uniqueList
}
