package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

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

	if !IsAclExists(fmt.Sprintf("%s.%s", netID, "all-remote-access-gws")) {
		defaultUserAcl := models.Acl{
			ID:          fmt.Sprintf("%s.%s", netID, "all-remote-access-gws"),
			Default:     true,
			Name:        "All Remote Access Gateways",
			NetworkID:   netID,
			Proto:       models.ALL,
			ServiceType: models.Any,
			Port:        []string{},
			RuleType:    models.DevicePolicy,
			Src: []models.AclPolicyTag{
				{
					ID:    models.NodeTagID,
					Value: fmt.Sprintf("%s.%s", netID, models.RemoteAccessTagName),
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

// DeleteDefaultNetworkPolicies - deletes all default network acl policies
func DeleteDefaultNetworkPolicies(netId models.NetworkID) {
	acls, _ := ListAclsByNetwork(netId)
	for _, acl := range acls {
		if acl.NetworkID == netId && acl.Default {
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
func GetEgressRanges(netID models.NetworkID) ([]string, map[string]struct{}, error) {

	var result []string
	resultMap := make(map[string]struct{})
	networkNodes, err := GetNetworkNodes(netID.String())
	if err != nil {
		return []string{}, nil, err
	}
	for _, currentNode := range networkNodes {
		if currentNode.Network != netID.String() {
			continue
		}
		if currentNode.IsEgressGateway { // add the egress gateway range(s) to the result
			if len(currentNode.EgressGatewayRanges) > 0 {
				result = append(result, currentNode.EgressGatewayRanges...)
				for _, egressRangeI := range currentNode.EgressGatewayRanges {
					resultMap[egressRangeI] = struct{}{}
				}
			}
		}
	}
	extclients, _ := GetNetworkExtClients(netID.String())
	for _, extclient := range extclients {
		if len(extclient.ExtraAllowedIPs) > 0 {
			result = append(result, extclient.ExtraAllowedIPs...)
			for _, extraAllowedIP := range extclient.ExtraAllowedIPs {
				resultMap[extraAllowedIP] = struct{}{}
			}
		}
	}
	return result, resultMap, nil
}

func checkIfAclTagisValid(t models.AclPolicyTag, netID models.NetworkID) bool {
	switch t.ID {
	case models.NodeTagID:
		// check if tag is valid
		_, err := GetTag(models.TagID(t.Value))
		if err != nil {
			return false
		}
	case models.NodeID:
		_, nodeErr := GetNodeByID(t.Value)
		if nodeErr != nil {
			_, staticNodeErr := GetExtClient(t.Value, netID.String())
			if staticNodeErr != nil {
				return false
			}
		}
	case models.EgressRange:
		_, rangesMap, err := GetEgressRanges(netID)
		if err != nil {
			return false
		}
		if _, ok := rangesMap[t.Value]; !ok {
			return false
		}
	case models.UserAclID:
		_, err := GetUser(t.Value)
		if err != nil {
			return false
		}
	case models.UserGroupAclID:
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
			if !checkIfAclTagisValid(srcI, acl.NetworkID) {
				return false
			}
		}
		for _, dstI := range acl.Dst {

			if dstI.Value == "*" {
				continue
			}

			// check if user group is valid
			if !checkIfAclTagisValid(dstI, acl.NetworkID) {
				return false
			}
		}
	case models.DevicePolicy:
		for _, srcI := range acl.Src {
			if srcI.Value == "*" {
				continue
			}
			// check if user group is valid
			if !checkIfAclTagisValid(srcI, acl.NetworkID) {
				return false
			}
		}
		for _, dstI := range acl.Dst {

			if dstI.Value == "*" {
				continue
			}
			// check if user group is valid
			if !checkIfAclTagisValid(dstI, acl.NetworkID) {
				return false
			}
		}
	}
	return true
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
	if peer.IsStatic {
		peer = peer.StaticNode.ConvertToStaticNode()
	}
	peer.Tags[models.TagID(peer.ID.String())] = struct{}{}
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
		for tagID := range peer.Tags {
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
	peerTags := maps.Clone(peer.Tags)
	nodeTags := maps.Clone(node.Tags)
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
			for i, srcI := range acl.Src {
				if srcI.ID == models.UserAclID && srcI.Value == userName {
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
			for i, srcI := range acl.Src {
				if srcI.ID == models.NodeID && srcI.Value == nodeID {
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
			for i, dstI := range acl.Dst {
				if dstI.ID == models.NodeID && dstI.Value == nodeID {
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
			for i, dstI := range acl.Dst {
				if dstI.ID == models.NodeID && dstI.Value == nodeID {
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

// IsNodeAllowedToCommunicate - check node is allowed to communicate with the peer
func IsNodeAllowedToCommunicate(node, peer models.Node, checkDefaultPolicy bool) (bool, []models.Acl) {
	var nodeId, peerId string
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
	node.Tags[models.TagID(nodeId)] = struct{}{}
	peer.Tags[models.TagID(peerId)] = struct{}{}
	peerTags := maps.Clone(peer.Tags)
	nodeTags := maps.Clone(node.Tags)
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
		if policy.AllowedDirection == models.TrafficDirectionBi {
			if _, ok := srcMap[node.ID.String()]; ok {
				allowedPolicies = append(allowedPolicies, policy)
				break
			}
		}
		if _, ok := dstMap[node.ID.String()]; ok {
			allowedPolicies = append(allowedPolicies, policy)
			break
		}
		for tagID := range nodeTags {
			allowed := false
			if _, ok := dstMap[tagID.String()]; policy.AllowedDirection == models.TrafficDirectionBi && ok {
				if _, ok := srcMap["*"]; ok {
					allowed = true
					allowedPolicies = append(allowedPolicies, policy)
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
			if _, ok := srcMap[tagID.String()]; ok {
				if _, ok := dstMap["*"]; ok {
					allowed = true
					allowedPolicies = append(allowedPolicies, policy)
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
				allowedPolicies = append(allowedPolicies, policy)
				break
			}
		}
		for tagID := range peerTags {
			allowed := false
			if _, ok := dstMap[tagID.String()]; ok {
				if _, ok := srcMap["*"]; ok {
					allowed = true
					allowedPolicies = append(allowedPolicies, policy)
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
				allowedPolicies = append(allowedPolicies, policy)
				break
			}

			if _, ok := srcMap[tagID.String()]; policy.AllowedDirection == models.TrafficDirectionBi && ok {
				if _, ok := dstMap["*"]; ok {
					allowed = true
					allowedPolicies = append(allowedPolicies, policy)
					break
				}
				for tagID := range nodeTags {
					if _, ok := dstMap[tagID.String()]; ok {
						allowed = true
						break
					}
				}
			}
			if allowed {
				allowedPolicies = append(allowedPolicies, policy)
				break
			}
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
		for i, srcTagI := range acl.Src {
			if srcTagI.ID == models.NodeTagID {
				if tagID.String() == srcTagI.Value {
					acl.Src = append(acl.Src[:i], acl.Src[i+1:]...)
					update = true
				}
			}
		}
		for i, dstTagI := range acl.Dst {
			if dstTagI.ID == models.NodeTagID {
				if tagID.String() == dstTagI.Value {
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

func getUserAclRulesForNode(targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	userNodes := GetStaticUserNodesByNetwork(models.NetworkID(targetnode.Network))
	userGrpMap := GetUserGrpMap()
	allowedUsers := make(map[string][]models.Acl)
	acls := listUserPolicies(models.NetworkID(targetnode.Network))

	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		dstTags := convAclTagToValueMap(acl.Dst)
		_, all := dstTags["*"]
		addUsers := false
		if !all {
			for nodeTag := range targetnode.Tags {
				if _, ok := dstTags[nodeTag.String()]; !ok {
					if _, ok = dstTags[targetnode.ID.String()]; !ok {
						break
					}
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
			dstTags := convAclTagToValueMap(acl.Dst)
			_, all := dstTags["*"]
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
			if targetnode.IsEgressGateway && len(targetnode.EgressGatewayRanges) > 0 {
				for _, egressRangeI := range targetnode.EgressGatewayRanges {
					if _, ok := dstTags[egressRangeI]; ok || all {
						_, egressCidr, err := net.ParseCIDR(egressRangeI)
						if err == nil {
							r.EgressRanges = append(r.EgressRanges, *egressCidr)
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

func checkIfAnyPolicyisUniDirectional(targetNode models.Node) bool {
	targetNode.Tags[models.TagID(targetNode.ID.String())] = struct{}{}
	acls := listDevicePolicies(models.NetworkID(targetNode.Network))
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		if acl.AllowedDirection == models.TrafficDirectionBi {
			continue
		}
		srcTags := convAclTagToValueMap(acl.Src)
		dstTags := convAclTagToValueMap(acl.Dst)
		for nodeTag := range targetNode.Tags {
			if _, ok := srcTags[nodeTag.String()]; ok {
				return true
			}
			if _, ok := srcTags[targetNode.ID.String()]; ok {
				return true
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
	targetnode.Tags[models.TagID(targetnode.ID.String())] = struct{}{}
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
	targetnode.Tags["*"] = struct{}{}

	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcTags := convAclTagToValueMap(acl.Src)
		dstTags := convAclTagToValueMap(acl.Dst)
		aclRule := models.AclRule{
			ID:              acl.ID,
			AllowedProtocol: acl.Proto,
			AllowedPorts:    acl.Port,
			Direction:       acl.AllowedDirection,
			Allowed:         true,
		}
		for nodeTag := range targetnode.Tags {
			if acl.AllowedDirection == models.TrafficDirectionBi {
				var existsInSrcTag bool
				var existsInDstTag bool

				if _, ok := srcTags[nodeTag.String()]; ok {
					existsInSrcTag = true
				}
				if _, ok := srcTags[targetnode.ID.String()]; ok {
					existsInSrcTag = true
				}
				if _, ok := dstTags[nodeTag.String()]; ok {
					existsInDstTag = true
				}
				if _, ok := dstTags[targetnode.ID.String()]; ok {
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

		}
		if targetnode.IsEgressGateway && len(targetnode.EgressGatewayRanges) > 0 {
			for _, egressRangeI := range targetnode.EgressGatewayRanges {
				_, egressCidr, err := net.ParseCIDR(egressRangeI)
				if err != nil {
					continue
				}
				_, all := dstTags["*"]
				if _, ok := dstTags[egressRangeI]; ok || all {
					// get all src tags
					aclRule.EgressRanges = append(aclRule.EgressRanges, *egressCidr)

				}
			}

		}
		if len(aclRule.IPList) > 0 || len(aclRule.IP6List) > 0 {
			rules[acl.ID] = aclRule
		}
	}
	return rules
}
