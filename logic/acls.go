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

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
)

// TODO: Write Diff Funcs

var IsNodeAllowedToCommunicate = isNodeAllowedToCommunicate

var GetFwRulesForNodeAndPeerOnGw = getFwRulesForNodeAndPeerOnGw

var GetFwRulesForUserNodesOnGw = func(node models.Node, nodes []models.Node) (rules []models.FwRule) { return }

func GetFwRulesOnIngressGateway(node models.Node) (rules []models.FwRule) {
	// fetch user access to static clients via policies
	defer func() {
		sort.Slice(rules, func(i, j int) bool {
			if !rules[i].SrcIP.IP.Equal(rules[j].SrcIP.IP) {
				return string(rules[i].SrcIP.IP.To16()) < string(rules[j].SrcIP.IP.To16())
			}
			return string(rules[i].DstIP.IP.To16()) < string(rules[j].DstIP.IP.To16())
		})
	}()
	defaultDevicePolicy, _ := GetDefaultPolicy(models.NetworkID(node.Network), models.DevicePolicy)
	nodes, _ := GetNetworkNodes(node.Network)
	nodes = append(nodes, GetStaticNodesByNetwork(models.NetworkID(node.Network), true)...)
	rules = GetFwRulesForUserNodesOnGw(node, nodes)
	if defaultDevicePolicy.Enabled {
		return
	}
	for _, nodeI := range nodes {
		if !nodeI.IsStatic || nodeI.IsUserNode {
			continue
		}
		// if nodeI.StaticNode.IngressGatewayID != node.ID.String() {
		// 	continue
		// }
		for _, peer := range nodes {
			if peer.StaticNode.ClientID == nodeI.StaticNode.ClientID || peer.IsUserNode {
				continue
			}
			if nodeI.StaticNode.IngressGatewayID != node.ID.String() &&
				((!peer.IsStatic && peer.ID.String() != node.ID.String()) ||
					(peer.IsStatic && peer.StaticNode.IngressGatewayID != node.ID.String())) {
				continue
			}
			if peer.IsStatic {
				peer = peer.StaticNode.ConvertToStaticNode()
			}
			var allowedPolicies1 []models.Acl
			var ok bool
			if ok, allowedPolicies1 = IsNodeAllowedToCommunicate(nodeI.StaticNode.ConvertToStaticNode(), peer, true); ok {
				rules = append(rules, GetFwRulesForNodeAndPeerOnGw(nodeI.StaticNode.ConvertToStaticNode(), peer, allowedPolicies1)...)
			}
			if ok, allowedPolicies2 := IsNodeAllowedToCommunicate(peer, nodeI.StaticNode.ConvertToStaticNode(), true); ok {
				rules = append(rules,
					GetFwRulesForNodeAndPeerOnGw(peer, nodeI.StaticNode.ConvertToStaticNode(),
						getUniquePolicies(allowedPolicies1, allowedPolicies2))...)
			}
		}
	}
	return
}

func getFwRulesForNodeAndPeerOnGw(node, peer models.Node, allowedPolicies []models.Acl) (rules []models.FwRule) {

	for _, policy := range allowedPolicies {
		// if static peer dst rule not for ingress node -> skip
		if node.Address.IP != nil {
			rules = append(rules, models.FwRule{
				SrcIP: net.IPNet{
					IP:   node.Address.IP,
					Mask: net.CIDRMask(32, 32),
				},
				DstIP: net.IPNet{
					IP:   peer.Address.IP,
					Mask: net.CIDRMask(32, 32),
				},
				Allow: true,
			})
		}

		if node.Address6.IP != nil {
			rules = append(rules, models.FwRule{
				SrcIP: net.IPNet{
					IP:   node.Address6.IP,
					Mask: net.CIDRMask(128, 128),
				},
				DstIP: net.IPNet{
					IP:   peer.Address6.IP,
					Mask: net.CIDRMask(128, 128),
				},
				Allow: true,
			})
		}
		if policy.AllowedDirection == models.TrafficDirectionBi {
			if node.Address.IP != nil {
				rules = append(rules, models.FwRule{
					SrcIP: net.IPNet{
						IP:   peer.Address.IP,
						Mask: net.CIDRMask(32, 32),
					},
					DstIP: net.IPNet{
						IP:   node.Address.IP,
						Mask: net.CIDRMask(32, 32),
					},
					Allow: true,
				})
			}

			if node.Address6.IP != nil {
				rules = append(rules, models.FwRule{
					SrcIP: net.IPNet{
						IP:   peer.Address6.IP,
						Mask: net.CIDRMask(128, 128),
					},
					DstIP: net.IPNet{
						IP:   node.Address6.IP,
						Mask: net.CIDRMask(128, 128),
					},
					Allow: true,
				})
			}
		}
		if len(node.StaticNode.ExtraAllowedIPs) > 0 {
			for _, additionalAllowedIPNet := range node.StaticNode.ExtraAllowedIPs {
				_, ipNet, err := net.ParseCIDR(additionalAllowedIPNet)
				if err != nil {
					continue
				}
				if ipNet.IP.To4() != nil && peer.Address.IP != nil {
					rules = append(rules, models.FwRule{
						SrcIP: net.IPNet{
							IP:   peer.Address.IP,
							Mask: net.CIDRMask(32, 32),
						},
						DstIP: *ipNet,
						Allow: true,
					})
				} else if peer.Address6.IP != nil {
					rules = append(rules, models.FwRule{
						SrcIP: net.IPNet{
							IP:   peer.Address6.IP,
							Mask: net.CIDRMask(128, 128),
						},
						DstIP: *ipNet,
						Allow: true,
					})
				}

			}

		}
		if len(peer.StaticNode.ExtraAllowedIPs) > 0 {
			for _, additionalAllowedIPNet := range peer.StaticNode.ExtraAllowedIPs {
				_, ipNet, err := net.ParseCIDR(additionalAllowedIPNet)
				if err != nil {
					continue
				}
				if ipNet.IP.To4() != nil && node.Address.IP != nil {
					rules = append(rules, models.FwRule{
						SrcIP: net.IPNet{
							IP:   node.Address.IP,
							Mask: net.CIDRMask(32, 32),
						},
						DstIP: *ipNet,
						Allow: true,
					})
				} else if node.Address6.IP != nil {
					rules = append(rules, models.FwRule{
						SrcIP: net.IPNet{
							IP:   node.Address6.IP,
							Mask: net.CIDRMask(128, 128),
						},
						DstIP: *ipNet,
						Allow: true,
					})
				}

			}

		}

		// add egress range rules
		for _, dstI := range policy.Dst {
			if dstI.ID == models.EgressID {

				e := schema.Egress{ID: dstI.Value}
				err := e.Get(db.WithContext(context.TODO()))
				if err != nil {
					continue
				}
				dstI.Value = e.Range

				ip, cidr, err := net.ParseCIDR(dstI.Value)
				if err == nil {
					if ip.To4() != nil {
						if node.Address.IP != nil {
							rules = append(rules, models.FwRule{
								SrcIP: net.IPNet{
									IP:   node.Address.IP,
									Mask: net.CIDRMask(32, 32),
								},
								DstIP: *cidr,
								Allow: true,
							})
						}
					} else {
						if node.Address6.IP != nil {
							rules = append(rules, models.FwRule{
								SrcIP: net.IPNet{
									IP:   node.Address6.IP,
									Mask: net.CIDRMask(128, 128),
								},
								DstIP: *cidr,
								Allow: true,
							})
						}
					}

				}
			}
		}
	}

	return
}

func getUniquePolicies(policies1, policies2 []models.Acl) []models.Acl {
	policies1Map := make(map[string]struct{})
	for _, policy1I := range policies1 {
		policies1Map[policy1I.ID] = struct{}{}
	}
	for i := len(policies2) - 1; i >= 0; i-- {
		if _, ok := policies1Map[policies2[i].ID]; ok {
			policies2 = append(policies2[:i], policies2[i+1:]...)
		}
	}
	return policies2
}

// Sort a slice of net.IP addresses
func sortIPs(ips []net.IP) {
	sort.Slice(ips, func(i, j int) bool {
		ip1, ip2 := ips[i].To16(), ips[j].To16()
		return string(ip1) < string(ip2) // Compare as byte slices
	})
}

func GetStaticNodeIps(node models.Node) (ips []net.IP) {
	defer func() {
		sortIPs(ips)
	}()
	defaultUserPolicy, _ := GetDefaultPolicy(models.NetworkID(node.Network), models.UserPolicy)
	defaultDevicePolicy, _ := GetDefaultPolicy(models.NetworkID(node.Network), models.DevicePolicy)

	extclients := GetStaticNodesByNetwork(models.NetworkID(node.Network), false)
	for _, extclient := range extclients {
		if extclient.IsUserNode && defaultUserPolicy.Enabled {
			continue
		}
		if !extclient.IsUserNode && defaultDevicePolicy.Enabled {
			continue
		}
		if extclient.StaticNode.Address != "" {
			ips = append(ips, extclient.StaticNode.AddressIPNet4().IP)
		}
		if extclient.StaticNode.Address6 != "" {
			ips = append(ips, extclient.StaticNode.AddressIPNet6().IP)
		}
	}
	return
}

var MigrateToGws = func() {

	nodes, err := GetAllNodes()
	if err != nil {
		return
	}
	for _, node := range nodes {
		if node.IsIngressGateway || node.IsRelay || node.IsInternetGateway {
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

func CheckIfNodeHasAccessToAllResources(targetnode *models.Node, acls []models.Acl) bool {
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
	if targetnode.IsGw {
		targetNodeTags[models.TagID(fmt.Sprintf("%s.%s", targetnode.Network, models.GwTagName))] = struct{}{}
	}
	for _, acl := range acls {
		if !acl.Enabled || acl.RuleType != models.DevicePolicy {
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

var CheckIfAnyPolicyisUniDirectional = func(targetNode models.Node, acls []models.Acl) bool {
	return false
}

var CheckIfAnyActiveEgressPolicy = func(targetNode models.Node, acls []models.Acl) bool {
	if !targetNode.EgressDetails.IsEgressGateway {
		return false
	}
	var targetNodeTags = make(map[models.TagID]struct{})
	targetNodeTags[models.TagID(targetNode.ID.String())] = struct{}{}
	targetNodeTags["*"] = struct{}{}
	if targetNode.IsGw {
		targetNodeTags[models.TagID(fmt.Sprintf("%s.%s", targetNode.Network, models.GwTagName))] = struct{}{}
	}
	for _, acl := range acls {
		if !acl.Enabled || acl.RuleType != models.DevicePolicy {
			continue
		}
		srcTags := ConvAclTagToValueMap(acl.Src)
		for _, dst := range acl.Dst {
			if dst.ID == models.EgressID {
				e := schema.Egress{ID: dst.Value}
				err := e.Get(db.WithContext(context.TODO()))
				if err == nil && e.Status {
					for nodeTag := range targetNodeTags {
						if _, ok := srcTags[nodeTag.String()]; ok {
							return true
						}
						if _, ok := srcTags[targetNode.ID.String()]; ok {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

var GetAclRulesForNode = func(targetnodeI *models.Node) (rules map[string]models.AclRule) {
	targetnode := *targetnodeI

	rules = make(map[string]models.AclRule)

	acls := ListDevicePolicies(models.NetworkID(targetnode.Network))
	targetNodeTags := make(map[models.TagID]struct{})
	targetNodeTags[models.TagID(targetnode.ID.String())] = struct{}{}
	targetNodeTags["*"] = struct{}{}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcTags := ConvAclTagToValueMap(acl.Src)
		dstTags := ConvAclTagToValueMap(acl.Dst)
		nodes := []models.Node{}
		for _, dst := range acl.Dst {
			if dst.ID == models.EgressID {
				e := schema.Egress{ID: dst.Value}
				err := e.Get(db.WithContext(context.TODO()))
				if err == nil && e.Status {
					for nodeID := range e.Nodes {
						dstTags[nodeID] = struct{}{}
					}
				}
			}
		}
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

				if existsInSrcTag /* && !existsInDstTag*/ {
					// get all dst tags
					for dst := range dstTags {
						if dst == nodeTag.String() {
							continue
						}
						// Get peers in the tags and add allowed rules
						if dst != targetnode.ID.String() {
							node, err := GetNodeByID(dst)
							if err == nil {
								nodes = append(nodes, node)
							}
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
				if existsInDstTag /*&& !existsInSrcTag*/ {
					// get all src tags
					for src := range srcTags {
						if src == nodeTag.String() {
							continue
						}
						// Get peers in the tags and add allowed rules
						if src != targetnode.ID.String() {
							node, err := GetNodeByID(src)
							if err == nil {
								nodes = append(nodes, node)
							}
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

		}

		if len(aclRule.IPList) > 0 || len(aclRule.IP6List) > 0 {
			aclRule.IPList = UniqueIPNetList(aclRule.IPList)
			aclRule.IP6List = UniqueIPNetList(aclRule.IP6List)
			rules[acl.ID] = aclRule
		}
	}
	return rules
}

var GetEgressRulesForNode = func(targetnode models.Node) (rules map[string]models.AclRule) {
	return
}
var GetAclRuleForInetGw = func(targetnode models.Node) (rules map[string]models.AclRule) {
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
			if srcI.ID == models.NodeTagID && srcI.Value == fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.GwTagName) {
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
			if dstI.ID == models.NodeTagID && dstI.Value == fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.GwTagName) {
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
	// if node.IsGw && peer.IsRelayed && peer.RelayedBy == node.ID.String() {
	// 	return true
	// }
	// if peer.IsGw && node.IsRelayed && node.RelayedBy == peer.ID.String() {
	// 	return true
	// }
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
	if peer.IsGw {
		peerTags[models.TagID(fmt.Sprintf("%s.%s", peer.Network, models.GwTagName))] = struct{}{}
	}
	if node.IsGw {
		nodeTags[models.TagID(fmt.Sprintf("%s.%s", node.Network, models.GwTagName))] = struct{}{}
	}
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
	// if node.IsGw && peer.IsRelayed && peer.RelayedBy == node.ID.String() {
	// 	return true, []models.Acl{}
	// }
	// if peer.IsGw && node.IsRelayed && node.RelayedBy == peer.ID.String() {
	// 	return true, []models.Acl{}
	// }
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
	if peer.IsGw {
		peerTags[models.TagID(fmt.Sprintf("%s.%s", peer.Network, models.GwTagName))] = struct{}{}
	}
	if node.IsGw {
		nodeTags[models.TagID(fmt.Sprintf("%s.%s", node.Network, models.GwTagName))] = struct{}{}
	}
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

// ListEgressAcls - list egress acl policies
func ListEgressAcls(eID string) ([]models.Acl, error) {
	allAcls := ListAcls()
	egressAcls := []models.Acl{}
	for _, acl := range allAcls {
		if !servercfg.IsPro && acl.RuleType == models.UserPolicy {
			continue
		}
		for _, dst := range acl.Dst {
			if dst.ID == models.EgressID && dst.Value == eID {
				egressAcls = append(egressAcls, acl)
			}
		}
	}
	return egressAcls, nil
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
		if !servercfg.IsPro {
			if acl.RuleType == models.UserPolicy {
				continue
			}
			skip := false
			for _, srcI := range acl.Src {
				if srcI.ID == models.NodeTagID && (srcI.Value != "*" && srcI.Value != fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.GwTagName)) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			for _, dstI := range acl.Dst {

				if dstI.ID == models.NodeTagID && (dstI.Value != "*" && dstI.Value != fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.GwTagName)) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
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
