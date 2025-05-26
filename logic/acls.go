package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
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

// TODO: Write Diff Funcs

var IsNodeAllowedToCommunicate = isNodeAllowedToCommunicate
var MigrateToGws = func() {

	nodes, err := GetAllNodes()
	if err != nil {
		return
	}
	for _, node := range nodes {
		if node.IsIngressGateway || node.IsRelay {
			node.IsGw = true
			node.IsIngressGateway = true
			node.IsRelay = true
			if node.Tags == nil {
				node.Tags = make(map[models.TagID]struct{})
			}
			UpsertNode(&node)
		}
	}

}

func CheckIfNodeHasAccessToAllResources(targetnode *models.Node) bool {
	acls := ListDevicePolicies(models.NetworkID(targetnode.Network))
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
		srcTags := ConvAclTagToValueMap(acl.Src)
		dstTags := ConvAclTagToValueMap(acl.Dst)
		_, srcAll := srcTags["*"]
		_, dstAll := dstTags["*"]
		for nodeTag := range targetNodeTags {

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
			if acl.AllowedDirection == models.TrafficDirectionBi {
				if existsInSrcTag && dstAll || existsInDstTag && srcAll {
					return true
				}
			} else {
				if existsInDstTag && srcAll {
					return true
				}
			}
		}
	}
	return false
}

var CheckIfAnyPolicyisUniDirectional = func(targetNode models.Node) bool {
	return false
}

var CheckIfAnyActiveEgressPolicy = func(targetNode models.Node) bool {
	if !targetNode.EgressDetails.IsEgressGateway {
		return false
	}
	var targetNodeTags = make(map[models.TagID]struct{})
	targetNodeTags[models.TagID(targetNode.ID.String())] = struct{}{}
	targetNodeTags["*"] = struct{}{}
	acls, _ := ListAclsByNetwork(models.NetworkID(targetNode.Network))
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcTags := ConvAclTagToValueMap(acl.Src)
		dstTags := ConvAclTagToValueMap(acl.Dst)
		for _, dst := range acl.Dst {
			if dst.ID == models.EgressID {
				e := schema.Egress{ID: dst.Value}
				err := e.Get(db.WithContext(context.TODO()))
				if err == nil && e.Status {
					for nodeID := range e.Nodes {
						dstTags[nodeID] = struct{}{}
					}
					dstTags[e.Range] = struct{}{}
				}
			}
		}
		for nodeTag := range targetNodeTags {
			if acl.RuleType == models.DevicePolicy && acl.AllowedDirection == models.TrafficDirectionBi {
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

var GetAclRulesForNode = func(targetnodeI *models.Node) (rules map[string]models.AclRule) {
	return
}

var GetEgressRulesForNode = func(targetnode models.Node) (rules map[string]models.AclRule) {
	return
}

func checkIfAclTagisValid(a models.Acl, t models.AclPolicyTag, isSrc bool) (err error) {
	switch t.ID {
	case models.NodeID:
		if a.RuleType == models.UserPolicy && isSrc {
			return errors.New("user policy source mismatch")
		}
		_, nodeErr := GetNodeByID(t.Value)
		if nodeErr != nil {
			_, staticNodeErr := GetExtClient(t.Value, a.NetworkID.String())
			if staticNodeErr != nil {
				return errors.New("invalid node " + t.Value)
			}
		}
	case models.EgressID, models.EgressRange:
		e := schema.Egress{
			ID: t.Value,
		}
		err := e.Get(db.WithContext(context.TODO()))
		if err != nil {
			return errors.New("invalid egress")
		}
		if e.IsInetGw {
			req := models.InetNodeReq{}
			for _, srcI := range a.Src {
				if srcI.ID == models.NodeID {
					req.InetNodeClientIDs = append(req.InetNodeClientIDs, srcI.Value)
				}
			}
			if len(e.Nodes) > 0 {
				for k := range e.Nodes {
					inetNode, err := GetNodeByID(k)
					if err != nil {
						return errors.New("invalid node " + t.Value)
					}
					if err = ValidateInetGwReq(inetNode, req, false); err != nil {
						return err
					}
				}
			}

		}
	default:
		return errors.New("invalid policy")
	}
	return nil
}

var IsAclPolicyValid = func(acl models.Acl) (err error) {

	//check if src and dst are valid
	if acl.AllowedDirection == models.TrafficDirectionUni {
		return errors.New("uni traffic flow not allowed on CE")
	}
	switch acl.RuleType {

	case models.DevicePolicy:
		for _, srcI := range acl.Src {
			if srcI.Value == "*" {
				continue
			}
			if err = checkIfAclTagisValid(acl, srcI, true); err != nil {
				return err
			}
		}
		for _, dstI := range acl.Dst {

			if dstI.Value == "*" {
				continue
			}
			if err = checkIfAclTagisValid(acl, dstI, false); err != nil {
				return
			}
		}
	default:
		return errors.New("unknown acl policy type " + string(acl.RuleType))
	}
	return nil
}

var IsPeerAllowed = func(node, peer models.Node, checkDefaultPolicy bool) bool {
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

	peerTags := make(map[models.TagID]struct{})
	nodeTags := make(map[models.TagID]struct{})
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
	policies := ListDevicePolicies(models.NetworkID(peer.Network))
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

		srcMap = ConvAclTagToValueMap(policy.Src)
		dstMap = ConvAclTagToValueMap(policy.Dst)
		for _, dst := range policy.Dst {
			if dst.ID == models.EgressID {
				e := schema.Egress{ID: dst.Value}
				err := e.Get(db.WithContext(context.TODO()))
				if err == nil && e.Status {
					for nodeID := range e.Nodes {
						dstMap[nodeID] = struct{}{}
					}
				}
			}
		}
		if CheckTagGroupPolicy(srcMap, dstMap, node, peer, nodeTags, peerTags) {
			return true
		}

	}
	return false
}

func CheckTagGroupPolicy(srcMap, dstMap map[string]struct{}, node, peer models.Node,
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

var GetInetClientsFromAclPolicies = func(eID string) (inetClientIDs []string) {
	e := schema.Egress{ID: eID}
	err := e.Get(db.WithContext(context.TODO()))
	if err != nil || !e.Status {
		return
	}
	acls, _ := ListAclsByNetwork(models.NetworkID(e.Network))
	for _, acl := range acls {
		for _, dstI := range acl.Dst {
			if dstI.ID == models.EgressID {
				if dstI.Value != eID {
					continue
				}
				for _, srcI := range acl.Src {
					if srcI.Value == "*" {
						continue
					}
					if srcI.ID == models.NodeID {
						inetClientIDs = append(inetClientIDs, srcI.Value)
					}
				}
			}
		}
	}
	return

}
var IsNodeUsingInternetGw = func(node *models.Node) {
	host, err := GetHost(node.HostID.String())
	if err != nil {
		return
	}
	if host.IsDefault || node.IsFailOver {
		return
	}
	acls, _ := ListAclsByNetwork(models.NetworkID(node.Network))
	var isUsing bool
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcVal := ConvAclTagToValueMap(acl.Src)
		for _, dstI := range acl.Dst {
			if dstI.ID == models.EgressID {
				e := schema.Egress{ID: dstI.Value}
				err := e.Get(db.WithContext(context.TODO()))
				if err != nil || !e.Status {
					continue
				}

				if e.IsInetGw {
					if _, ok := srcVal[node.ID.String()]; ok {
						for nodeID := range e.Nodes {
							if nodeID == node.ID.String() {
								continue
							}
							node.EgressDetails.InternetGwID = nodeID
							isUsing = true
							return
						}
					}
				}
			}
		}
	}
	if !isUsing {
		node.EgressDetails.InternetGwID = ""
	}
}

var (
	CreateDefaultTags = func(netID models.NetworkID) {}

	DeleteAllNetworkTags = func(networkID models.NetworkID) {}

	IsUserAllowedToCommunicate = func(userName string, peer models.Node) (bool, []models.Acl) {
		return false, []models.Acl{}
	}

	RemoveUserFromAclPolicy = func(userName string) {}
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

// IsNodeAllowedToCommunicate - check node is allowed to communicate with the peer // ADD ALLOWED DIRECTION - 0 => node -> peer, 1 => peer-> node,
func isNodeAllowedToCommunicate(node, peer models.Node, checkDefaultPolicy bool) (bool, []models.Acl) {
	var nodeId, peerId string
	if peer.IsGw && node.RelayedBy == peer.ID.String() {
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

	nodeTags := make(map[models.TagID]struct{})
	peerTags := make(map[models.TagID]struct{})

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
		allowedPolicies = UniquePolicies(allowedPolicies)
	}()
	// list device policies
	policies := ListDevicePolicies(models.NetworkID(peer.Network))
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
		srcMap = ConvAclTagToValueMap(policy.Src)
		dstMap = ConvAclTagToValueMap(policy.Dst)
		for _, dst := range policy.Dst {
			if dst.ID == models.EgressID {
				e := schema.Egress{ID: dst.Value}
				err := e.Get(db.WithContext(context.TODO()))
				if err == nil && e.Status {
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
			dstMap = ConvAclTagToValueMap(policy.Dst)
			srcMap = ConvAclTagToValueMap(policy.Src)
			if _, ok := srcMap["*"]; ok {
				if _, ok := dstMap["*"]; ok {
					return policy, nil
				}
			}
		}

	}

	return acl, nil
}

// ListAcls - lists all acl policies
func ListAclsByNetwork(netID models.NetworkID) ([]models.Acl, error) {

	allAcls := ListAcls()
	netAcls := []models.Acl{}
	for _, acl := range allAcls {
		if !servercfg.IsPro && acl.RuleType == models.UserPolicy {
			continue
		}
		if acl.NetworkID == netID {
			netAcls = append(netAcls, acl)
		}
	}
	return netAcls, nil
}

// ListDevicePolicies - lists all device policies in a network
func ListDevicePolicies(netID models.NetworkID) []models.Acl {
	allAcls := ListAcls()
	deviceAcls := []models.Acl{}
	for _, acl := range allAcls {
		if acl.NetworkID == netID && acl.RuleType == models.DevicePolicy {
			deviceAcls = append(deviceAcls, acl)
		}
	}
	return deviceAcls
}

func ConvAclTagToValueMap(acltags []models.AclPolicyTag) map[string]struct{} {
	aclValueMap := make(map[string]struct{})
	for _, aclTagI := range acltags {
		aclValueMap[aclTagI.Value] = struct{}{}
	}
	return aclValueMap
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

func UniquePolicies(items []models.Acl) []models.Acl {
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

// DeleteNetworkPolicies - deletes all default network acl policies
func DeleteNetworkPolicies(netId models.NetworkID) {
	acls, _ := ListAclsByNetwork(netId)
	for _, acl := range acls {
		if acl.NetworkID == netId {
			DeleteAcl(acl)
		}
	}
}

// SortTagEntrys - Sorts slice of Tag entries by their id
func SortAclEntrys(acls []models.Acl) {
	sort.Slice(acls, func(i, j int) bool {
		return acls[i].Name < acls[j].Name
	})
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
