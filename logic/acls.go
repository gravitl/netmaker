package logic

import (
	"encoding/json"
	"errors"
	"fmt"
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

// CreateDefaultAclNetworkPolicies - create default acl network policies
func CreateDefaultAclNetworkPolicies(netID models.NetworkID) {
	if netID.String() == "" {
		return
	}
	if !IsAclExists(fmt.Sprintf("%s.%s", netID, "all-nodes")) {
		defaultDeviceAcl := models.Acl{
			ID:        fmt.Sprintf("%s.%s", netID, "all-nodes"),
			Name:      "All Nodes",
			MetaData:  "This Policy allows all nodes in the network to communicate with each other",
			Default:   true,
			NetworkID: netID,
			RuleType:  models.DevicePolicy,
			Src: []models.AclPolicyTag{
				{
					ID:    models.DeviceAclID,
					Value: "*",
				}},
			Dst: []models.AclPolicyTag{
				{
					ID:    models.DeviceAclID,
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
			ID:        fmt.Sprintf("%s.%s", netID, "all-users"),
			Default:   true,
			Name:      "All Users",
			MetaData:  "This policy gives access to everything in the network for an user",
			NetworkID: netID,
			RuleType:  models.UserPolicy,
			Src: []models.AclPolicyTag{
				{
					ID:    models.UserAclID,
					Value: "*",
				},
			},
			Dst: []models.AclPolicyTag{{
				ID:    models.DeviceAclID,
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
			ID:        fmt.Sprintf("%s.%s", netID, "all-remote-access-gws"),
			Default:   true,
			Name:      "All Remote Access Gateways",
			NetworkID: netID,
			RuleType:  models.DevicePolicy,
			Src: []models.AclPolicyTag{
				{
					ID:    models.DeviceAclID,
					Value: fmt.Sprintf("%s.%s", netID, models.RemoteAccessTagName),
				},
			},
			Dst: []models.AclPolicyTag{
				{
					ID:    models.DeviceAclID,
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
	acls, _ := ListAcls(netId)
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

// IsAclPolicyValid - validates if acl policy is valid
func IsAclPolicyValid(acl models.Acl) bool {
	//check if src and dst are valid

	switch acl.RuleType {
	case models.UserPolicy:
		// src list should only contain users
		for _, srcI := range acl.Src {

			if srcI.ID == "" || srcI.Value == "" {
				return false
			}
			if srcI.Value == "*" {
				continue
			}
			if srcI.ID != models.UserAclID && srcI.ID != models.UserGroupAclID {
				return false
			}
			// check if user group is valid
			if srcI.ID == models.UserAclID {
				_, err := GetUser(srcI.Value)
				if err != nil {
					return false
				}

			} else if srcI.ID == models.UserGroupAclID {
				err := IsGroupValid(models.UserGroupID(srcI.Value))
				if err != nil {
					return false
				}
				// check if group belongs to this network
				netGrps := GetUserGroupsInNetwork(acl.NetworkID)
				if _, ok := netGrps[models.UserGroupID(srcI.Value)]; !ok {
					return false
				}
			}

		}
		for _, dstI := range acl.Dst {

			if dstI.ID == "" || dstI.Value == "" {
				return false
			}
			if dstI.ID != models.DeviceAclID {
				return false
			}
			if dstI.Value == "*" {
				continue
			}
			// check if tag is valid
			_, err := GetTag(models.TagID(dstI.Value))
			if err != nil {
				return false
			}
		}
	case models.DevicePolicy:
		for _, srcI := range acl.Src {
			if srcI.ID == "" || srcI.Value == "" {
				return false
			}
			if srcI.ID != models.DeviceAclID {
				return false
			}
			if srcI.Value == "*" {
				continue
			}
			// check if tag is valid
			_, err := GetTag(models.TagID(srcI.Value))
			if err != nil {
				return false
			}
		}
		for _, dstI := range acl.Dst {

			if dstI.ID == "" || dstI.Value == "" {
				return false
			}
			if dstI.ID != models.DeviceAclID {
				return false
			}
			if dstI.Value == "*" {
				continue
			}
			// check if tag is valid
			_, err := GetTag(models.TagID(dstI.Value))
			if err != nil {
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
	policies, _ := ListAcls(netID)
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

func listAcls() (acls []models.Acl) {
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
	allAcls := listAcls()
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
	allAcls := listAcls()
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
	allAcls := listAcls()
	deviceAcls := []models.Acl{}
	for _, acl := range allAcls {
		if acl.NetworkID == netID && acl.RuleType == models.DevicePolicy {
			deviceAcls = append(deviceAcls, acl)
		}
	}
	return deviceAcls
}

// ListAcls - lists all acl policies
func ListAcls(netID models.NetworkID) ([]models.Acl, error) {

	allAcls := listAcls()
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
func IsUserAllowedToCommunicate(userName string, peer models.Node) bool {
	if peer.IsStatic {
		peer = peer.StaticNode.ConvertToStaticNode()
	}
	acl, _ := GetDefaultPolicy(models.NetworkID(peer.Network), models.UserPolicy)
	if acl.Enabled {
		return true
	}
	user, err := GetUser(userName)
	if err != nil {
		return false
	}

	policies := listPoliciesOfUser(*user, models.NetworkID(peer.Network))
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		dstMap := convAclTagToValueMap(policy.Dst)
		if _, ok := dstMap["*"]; ok {
			return true
		}
		for tagID := range peer.Tags {
			if _, ok := dstMap[tagID.String()]; ok {
				return true
			}
		}

	}
	return false
}

// IsNodeAllowedToCommunicate - check node is allowed to communicate with the peer
func IsNodeAllowedToCommunicate(node, peer models.Node) bool {
	if node.IsStatic {
		node = node.StaticNode.ConvertToStaticNode()
	}
	if peer.IsStatic {
		peer = peer.StaticNode.ConvertToStaticNode()
	}
	// check default policy if all allowed return true
	defaultPolicy, err := GetDefaultPolicy(models.NetworkID(node.Network), models.DevicePolicy)
	if err == nil {
		if defaultPolicy.Enabled {
			return true
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
		// fmt.Printf("\n======> SRCMAP: %+v\n", srcMap)
		// fmt.Printf("\n======> DSTMAP: %+v\n", dstMap)
		// fmt.Printf("\n======> node Tags: %+v\n", node.Tags)
		// fmt.Printf("\n======> peer Tags: %+v\n", peer.Tags)
		for tagID := range node.Tags {
			if _, ok := dstMap[tagID.String()]; ok {
				if _, ok := srcMap["*"]; ok {
					return true
				}
				for tagID := range peer.Tags {
					if _, ok := srcMap[tagID.String()]; ok {
						return true
					}
				}
			}
			if _, ok := srcMap[tagID.String()]; ok {
				if _, ok := dstMap["*"]; ok {
					return true
				}
				for tagID := range peer.Tags {
					if _, ok := dstMap[tagID.String()]; ok {
						return true
					}
				}
			}
		}
		for tagID := range peer.Tags {
			if _, ok := dstMap[tagID.String()]; ok {
				if _, ok := srcMap["*"]; ok {
					return true
				}
				for tagID := range node.Tags {

					if _, ok := srcMap[tagID.String()]; ok {
						return true
					}
				}
			}
			if _, ok := srcMap[tagID.String()]; ok {
				if _, ok := dstMap["*"]; ok {
					return true
				}
				for tagID := range node.Tags {
					if _, ok := dstMap[tagID.String()]; ok {
						return true
					}
				}
			}
		}
	}
	return false
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
			if srcTagI.ID == models.DeviceAclID {
				if OldID.String() == srcTagI.Value {
					acl.Src[i].Value = newID.String()
					update = true
				}
			}
		}
		for i, dstTagI := range acl.Dst {
			if dstTagI.ID == models.DeviceAclID {
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
			if srcTagI.ID == models.DeviceAclID {
				if tagID.String() == srcTagI.Value {
					return true
				}
			}
		}
		for _, dstTagI := range acl.Dst {
			if dstTagI.ID == models.DeviceAclID {
				return true
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
			if srcTagI.ID == models.DeviceAclID {
				if tagID.String() == srcTagI.Value {
					acl.Src = append(acl.Src[:i], acl.Src[i+1:]...)
					update = true
				}
			}
		}
		for i, dstTagI := range acl.Dst {
			if dstTagI.ID == models.DeviceAclID {
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
