package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
)

// CreateDefaultAclNetworkPolicies - create default acl network policies
func CreateDefaultAclNetworkPolicies(netID models.NetworkID) {
	if netID.String() == "" {
		return
	}
	if !IsAclExists(models.AclID(fmt.Sprintf("%s.%s", netID, "all-nodes"))) {
		defaultDeviceAcl := models.Acl{
			ID:        models.AclID(fmt.Sprintf("%s.%s", netID, "all-nodes")),
			Name:      "all-nodes",
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
	if !IsAclExists(models.AclID(fmt.Sprintf("%s.%s", netID, "all-users"))) {
		defaultUserAcl := models.Acl{
			ID:        models.AclID(fmt.Sprintf("%s.%s", netID, "all-users")),
			Default:   true,
			Name:      "all-users",
			NetworkID: netID,
			RuleType:  models.UserPolicy,
			Src: []models.AclPolicyTag{
				{
					ID:    models.UserAclID,
					Value: "*",
				},
				{
					ID:    models.UserGroupAclID,
					Value: "*",
				},
				{
					ID:    models.UserRoleAclID,
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

	if !IsAclExists(models.AclID(fmt.Sprintf("%s.%s", netID, "all-remote-access-gws"))) {
		defaultUserAcl := models.Acl{
			ID:        models.AclID(fmt.Sprintf("%s.%s", netID, "all-remote-access-gws")),
			Default:   true,
			Name:      "all-remote-access-gws",
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
	err = CheckIDSyntax(req.Name)
	if err != nil {
		return err
	}
	req.GetID(req.NetworkID, req.Name)
	_, err = GetAcl(req.ID)
	if err == nil {
		return errors.New("acl exists already with name " + req.Name)
	}
	return nil
}

// InsertAcl - creates acl policy
func InsertAcl(a models.Acl) error {
	d, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return database.Insert(a.ID.String(), string(d), database.ACLS_TABLE_NAME)
}

// GetAcl - gets acl info by id
func GetAcl(aID models.AclID) (models.Acl, error) {
	a := models.Acl{}
	d, err := database.FetchRecord(database.ACLS_TABLE_NAME, aID.String())
	if err != nil {
		return a, err
	}
	err = json.Unmarshal([]byte(d), &a)
	if err != nil {
		return a, err
	}
	return a, nil
}

// IsAclExists - checks if acl exists
func IsAclExists(aclID models.AclID) bool {
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
			if srcI.ID != models.UserAclID &&
				srcI.ID != models.UserGroupAclID && srcI.ID != models.UserRoleAclID {
				return false
			}
			// check if user group is valid
			if srcI.ID == models.UserAclID {
				_, err := GetUser(srcI.Value)
				if err != nil {
					return false
				}
			} else if srcI.ID == models.UserRoleAclID {

				_, err := GetRole(models.UserRoleID(srcI.Value))
				if err != nil {
					return false
				}

			} else if srcI.ID == models.UserGroupAclID {
				err := IsGroupValid(models.UserGroupID(srcI.Value))
				if err != nil {
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
	if acl.ID != newAcl.ID {
		database.DeleteRecord(database.ACLS_TABLE_NAME, acl.ID.String())
		acl.ID = newAcl.ID
	}
	d, err := json.Marshal(acl)
	if err != nil {
		return err
	}
	return database.Insert(acl.ID.String(), string(d), database.ACLS_TABLE_NAME)
}

// UpsertAcl - upserts acl
func UpsertAcl(acl models.Acl) error {
	d, err := json.Marshal(acl)
	if err != nil {
		return err
	}
	return database.Insert(acl.ID.String(), string(d), database.ACLS_TABLE_NAME)
}

// DeleteAcl - deletes acl policy
func DeleteAcl(a models.Acl) error {
	return database.DeleteRecord(database.ACLS_TABLE_NAME, a.ID.String())
}

// GetDefaultPolicy - fetches default policy in the network by ruleType
func GetDefaultPolicy(netID models.NetworkID, ruleType models.AclPolicyType) (models.Acl, error) {
	aclID := "all-users"
	if ruleType == models.DevicePolicy {
		aclID = "all-nodes"
	}
	acl, err := GetAcl(models.AclID(fmt.Sprintf("%s.%s", netID, aclID)))
	if err != nil {
		return models.Acl{}, errors.New("default rule not found")
	}
	return acl, nil
}

// ListUserPolicies - lists all acl policies enforced on an user
func ListUserPolicies(u models.User) []models.Acl {
	data, err := database.FetchRecords(database.ACLS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return []models.Acl{}
	}
	acls := []models.Acl{}
	for _, dataI := range data {
		acl := models.Acl{}
		err := json.Unmarshal([]byte(dataI), &acl)
		if err != nil {
			continue
		}

		if acl.RuleType == models.UserPolicy {
			srcMap := convAclTagToValueMap(acl.Src)
			if _, ok := srcMap[u.UserName]; ok {
				acls = append(acls, acl)
			} else {
				// check for user groups
				for gID := range u.UserGroups {
					if _, ok := srcMap[gID.String()]; ok {
						acls = append(acls, acl)
						break
					}
				}
			}

		}
	}
	return acls
}

// listPoliciesOfUser - lists all user acl policies applied to user in an network
func listPoliciesOfUser(user models.User, netID models.NetworkID) []models.Acl {
	data, err := database.FetchRecords(database.ACLS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return []models.Acl{}
	}
	acls := []models.Acl{}
	for _, dataI := range data {
		acl := models.Acl{}
		err := json.Unmarshal([]byte(dataI), &acl)
		if err != nil {
			continue
		}
		if acl.NetworkID == netID && acl.RuleType == models.UserPolicy {
			srcMap := convAclTagToValueMap(acl.Src)
			if _, ok := srcMap[user.UserName]; ok {
				acls = append(acls, acl)
				continue
			}
			for netRole := range user.NetworkRoles {
				if _, ok := srcMap[netRole.String()]; ok {
					acls = append(acls, acl)
					continue
				}
			}
			for userG := range user.UserGroups {
				if _, ok := srcMap[userG.String()]; ok {
					acls = append(acls, acl)
					continue
				}
			}

		}
	}
	return acls
}

// listUserPoliciesByNetwork - lists all acl user policies in a network
func listUserPoliciesByNetwork(netID models.NetworkID) []models.Acl {
	data, err := database.FetchRecords(database.ACLS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return []models.Acl{}
	}
	acls := []models.Acl{}
	for _, dataI := range data {
		acl := models.Acl{}
		err := json.Unmarshal([]byte(dataI), &acl)
		if err != nil {
			continue
		}
		if acl.NetworkID == netID && acl.RuleType == models.UserPolicy {
			acls = append(acls, acl)
		}
	}
	return acls
}

// listDevicePolicies - lists all device policies in a network
func listDevicePolicies(netID models.NetworkID) []models.Acl {
	data, err := database.FetchRecords(database.ACLS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return []models.Acl{}
	}
	acls := []models.Acl{}
	for _, dataI := range data {
		acl := models.Acl{}
		err := json.Unmarshal([]byte(dataI), &acl)
		if err != nil {
			continue
		}
		if acl.NetworkID == netID && acl.RuleType == models.DevicePolicy {
			acls = append(acls, acl)
		}
	}
	return acls
}

// ListAcls - lists all acl policies
func ListAcls(netID models.NetworkID) ([]models.Acl, error) {
	data, err := database.FetchRecords(database.ACLS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return []models.Acl{}, err
	}
	acls := []models.Acl{}
	for _, dataI := range data {
		acl := models.Acl{}
		err := json.Unmarshal([]byte(dataI), &acl)
		if err != nil {
			continue
		}
		if acl.NetworkID == netID {
			acls = append(acls, acl)
		}
	}
	return acls, nil
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
	// check default policy if all allowed return true
	defaultPolicy, err := GetDefaultPolicy(models.NetworkID(node.Network), models.DevicePolicy)
	if err == nil {
		if defaultPolicy.Enabled {
			return true
		}
	}
	if node.IsStatic {
		node = node.StaticNode.ConvertToStaticNode()
	}
	if peer.IsStatic {
		peer = peer.StaticNode.ConvertToStaticNode()
	}
	// list device policies
	policies := listDevicePolicies(models.NetworkID(peer.Network))
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		srcMap := convAclTagToValueMap(policy.Src)
		dstMap := convAclTagToValueMap(policy.Dst)
		fmt.Printf("\n======> SRCMAP: %+v\n", srcMap)
		fmt.Printf("\n======> DSTMAP: %+v\n", dstMap)
		fmt.Printf("\n======> node Tags: %+v\n", node.Tags)
		fmt.Printf("\n======> peer Tags: %+v\n", peer.Tags)
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
