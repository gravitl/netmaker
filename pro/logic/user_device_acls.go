package logic

import (
	"context"
	"net"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// Host-backed user devices (schema.Host.OwnerUsername) are real nodes, so their
// firewall source is the node's own overlay address instead of a static-node
// address. These hooks run alongside the ExtClient/RAC user hooks in
// pro/logic/acls.go, which stay untouched.

// userDeviceRuleSuffix keeps user-device rule IDs from colliding with the
// ExtClient user rules, which are keyed by the bare policy ID.
const userDeviceRuleSuffix = "#user-device"

// listUserDevicesForACL lists host-backed user devices; tests may override.
var listUserDevicesForACL = logic.ListUserDevicesByNetwork

// listEgressForUserDeviceACL lists network egresses; tests may override.
var listEgressForUserDeviceACL = func(ctx context.Context, network string) ([]schema.Egress, error) {
	return (&schema.Egress{Network: network}).ListByNetwork(ctx)
}

// getDefaultUserPolicyForUserDeviceACL loads the default user policy; tests may override.
var getDefaultUserPolicyForUserDeviceACL = func(ctx context.Context, netID schema.NetworkID) (models.Acl, error) {
	return logic.GetDefaultPolicy(ctx, netID, models.UserPolicy)
}

// userDeviceSrcIPsForPolicy returns the overlay addresses of the devices whose
// owner is a source of the policy, either directly or through a user group.
func userDeviceSrcIPsForPolicy(policy models.Acl, devices []models.Node,
	userGrpMap map[schema.UserGroupID]map[string]struct{}) (src4, src6 []net.IPNet) {
	owners := make(map[string]struct{})
	for _, srcAcl := range policy.Src {
		switch srcAcl.ID {
		case models.UserAclID:
			owners[srcAcl.Value] = struct{}{}
		case models.UserGroupAclID:
			if usersMap, ok := userGrpMap[schema.UserGroupID(srcAcl.Value)]; ok {
				for userName := range usersMap {
					owners[userName] = struct{}{}
				}
			}
		}
	}
	if len(owners) == 0 {
		return
	}
	for i := range devices {
		if _, ok := owners[logic.NodeOwnerUsername(&devices[i])]; !ok {
			continue
		}
		deviceSrc4, deviceSrc6 := logic.UserDeviceSrcIPs(&devices[i])
		if deviceSrc4.IP != nil {
			src4 = append(src4, deviceSrc4)
		}
		if deviceSrc6.IP != nil {
			src6 = append(src6, deviceSrc6)
		}
	}
	return
}

// allUserDeviceSrcIPs returns every user device address, for the default user
// policy which covers all users.
func allUserDeviceSrcIPs(devices []models.Node) (src4, src6 []net.IPNet) {
	for i := range devices {
		deviceSrc4, deviceSrc6 := logic.UserDeviceSrcIPs(&devices[i])
		if deviceSrc4.IP != nil {
			src4 = append(src4, deviceSrc4)
		}
		if deviceSrc6.IP != nil {
			src6 = append(src6, deviceSrc6)
		}
	}
	return
}

// egressMatchesPolicyDst reports whether an egress is a destination of the policy.
func egressMatchesPolicyDst(e schema.Egress, dstTags map[string]struct{}) bool {
	if _, ok := dstTags[e.ID]; ok {
		return true
	}
	if e.Range != "" {
		if _, ok := dstTags[e.Range]; ok {
			return true
		}
	}
	for _, domainAns := range logic.AllDomainAnsFromEgress(e) {
		if _, ok := dstTags[domainAns]; ok {
			return true
		}
	}
	return false
}

// nodeRoutesEgress reports whether node is a routing node of e. Routing nodes
// are attached either individually (e.Nodes) or by tag (e.Tags).
func nodeRoutesEgress(node *models.Node, e *schema.Egress) bool {
	if node == nil || e == nil {
		return false
	}
	if _, ok := e.Nodes[node.ID.String()]; ok {
		return true
	}
	for tagID := range node.Tags {
		if _, ok := e.Tags[tagID.String()]; ok {
			return true
		}
	}
	return false
}

// appendUserDevicePolicyEgressDsts adds the egress destinations the policy grants
// as seen from node. IPs selected on the policy replace the egress range, so an
// IP-restricted policy never widens back to the whole range.
//
// Internet exits (0.0.0.0/0) are only emitted on nodes that actually route that
// exit. Putting a default-route allow on an unrelated site-egress gateway would
// show up as iptables "anywhere" and bypass per-egress IP restrictions.
func appendUserDevicePolicyEgressDsts(ctx context.Context, node *models.Node,
	policy models.Acl, egs []schema.Egress, dst4, dst6 *[]net.IPNet) {
	selectedIP4, selectedIP6 := getSelectedUserEgressIPNets(policy.Dst)
	dstTags := logic.ConvAclTagToValueMap(policy.Dst)
	_, dstAll := dstTags["*"]
	for _, egI := range egs {
		if !egI.Status || (len(egI.Nodes) == 0 && len(egI.Tags) == 0) {
			continue
		}
		if !dstAll && !egressMatchesPolicyDst(egI, dstTags) {
			continue
		}
		routes := nodeRoutesEgress(node, &egI)
		if logic.IsEgressInternetGateway(egI) {
			// 0.0.0.0/0 must only be installed on the exit's routing node.
			if !routes {
				continue
			}
			logic.AppendEgressPolicyRange(egI, egI.Range, dst4, dst6)
			continue
		}
		if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
			if !routes {
				continue
			}
			*dst4 = append(*dst4, selectedIP4...)
			*dst6 = append(*dst6, selectedIP6...)
			continue
		}
		egressRange := egI.Range
		if !routes && egI.VirtualRange != "" {
			egressRange = egI.VirtualRange
		}
		if egressRange != "" {
			logic.AppendEgressPolicyRange(egI, egressRange, dst4, dst6)
			continue
		}
		if logic.HasEgressDomainAns(egI) {
			for _, domainAns := range logic.AllDomainAnsFromEgress(egI) {
				ip, cidr, err := net.ParseCIDR(domainAns)
				if err != nil {
					continue
				}
				if ip.To4() != nil {
					*dst4 = append(*dst4, *cidr)
				} else {
					*dst6 = append(*dst6, *cidr)
				}
			}
		}
	}
}

// addUserDeviceAclRule merges a user-device rule into rules. A rule with sources
// but no destination is dropped: netclient omits "-d" in that case, which would
// silently allow everything.
func addUserDeviceAclRule(rules map[string]models.AclRule, policy models.Acl,
	src4, src6, dst4, dst6 []net.IPNet) {
	if len(src4) == 0 && len(src6) == 0 {
		return
	}
	if len(dst4) == 0 && len(dst6) == 0 {
		return
	}
	ruleID := policy.ID + userDeviceRuleSuffix
	r, ok := rules[ruleID]
	if !ok {
		r = models.AclRule{
			ID:              ruleID,
			AllowedProtocol: policy.Proto,
			AllowedPorts:    policy.Port,
			Direction:       policy.AllowedDirection,
			Allowed:         true,
		}
	}
	r.IPList = logic.UniqueIPNetList(append(r.IPList, src4...))
	r.IP6List = logic.UniqueIPNetList(append(r.IP6List, src6...))
	r.Dst = logic.UniqueIPNetList(append(r.Dst, dst4...))
	r.Dst6 = logic.UniqueIPNetList(append(r.Dst6, dst6...))
	rules[ruleID] = r
}

// GetUserDeviceEgressRulesForNode emits the forward-chain rules a routing node
// needs for the user devices allowed to reach the egresses it serves.
func GetUserDeviceEgressRulesForNode(ctx context.Context, targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	if logic.IsUserOwnedDevice(targetnode) {
		return rules
	}
	devices := listUserDevicesForACL(ctx, targetnode.Network)
	if len(devices) == 0 {
		return rules
	}
	egs, _ := listEgressForUserDeviceACL(ctx, targetnode.Network)
	if len(egs) == 0 {
		return rules
	}
	netID := schema.NetworkID(targetnode.Network)
	policies := listUserPolicies(ctx, netID)
	if defaultPolicy, err := logic.GetDefaultPolicy(ctx, netID, models.UserPolicy); err == nil && defaultPolicy.Enabled {
		policies = []models.Acl{defaultPolicy}
	}
	userGrpMap := userGroupsForNetwork(ctx, netID)
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		var src4, src6 []net.IPNet
		if policy.Default {
			src4, src6 = allUserDeviceSrcIPs(devices)
		} else {
			src4, src6 = userDeviceSrcIPsForPolicy(policy, devices, userGrpMap)
		}
		if len(src4) == 0 && len(src6) == 0 {
			continue
		}
		var dst4, dst6 []net.IPNet
		appendUserDevicePolicyEgressDsts(ctx, targetnode, policy, egs, &dst4, &dst6)
		addUserDeviceAclRule(rules, policy, src4, src6, dst4, dst6)
	}
	return rules
}

// GetUserDeviceAclRulesForNode emits the input-chain rules a resource needs for
// the user devices whose owner is allowed to reach it.
func GetUserDeviceAclRulesForNode(ctx context.Context, targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	if logic.IsUserOwnedDevice(targetnode) {
		return rules
	}
	devices := listUserDevicesForACL(ctx, targetnode.Network)
	if len(devices) == 0 {
		return rules
	}
	egs, _ := listEgressForUserDeviceACL(ctx, targetnode.Network)
	var dst4, dst6 []net.IPNet
	if targetnode.Address.IP != nil {
		dst4 = append(dst4, targetnode.AddressIPNet4())
	}
	if targetnode.Address6.IP != nil {
		dst6 = append(dst6, targetnode.AddressIPNet6())
	}
	for i := range devices {
		owner := logic.NodeOwnerUsername(&devices[i])
		if owner == "" {
			continue
		}
		allowed, allowedPolicies := IsUserAllowedToCommunicate(ctx, owner, *targetnode)
		if !allowed {
			continue
		}
		src4, src6 := logic.UserDeviceSrcIPs(&devices[i])
		var deviceSrc4, deviceSrc6 []net.IPNet
		if src4.IP != nil {
			deviceSrc4 = []net.IPNet{src4}
		}
		if src6.IP != nil {
			deviceSrc6 = []net.IPNet{src6}
		}
		for _, policy := range allowedPolicies {
			policyDst4 := append([]net.IPNet(nil), dst4...)
			policyDst6 := append([]net.IPNet(nil), dst6...)
			appendUserDevicePolicyEgressDsts(ctx, targetnode, policy, egs, &policyDst4, &policyDst6)
			addUserDeviceAclRule(rules, policy, deviceSrc4, deviceSrc6, policyDst4, policyDst6)
		}
	}
	return rules
}

// GetFwRulesForUserDevicesOnGw emits ingress/relay forward rules so host-backed
// user devices can reach ACL-allowed mesh peers and static/tagged extclients.
// Unlike RAC, user devices have no IngressGatewayID — they peer with the gateway
// directly — so every user device on the network is considered (same shape as
// GetFwRulesForUserNodesOnGw), not only RelayedBy attachments.
func GetFwRulesForUserDevicesOnGw(ctx context.Context, node models.Node, nodes []models.Node) (rules []models.FwRule) {
	devices := listUserDevicesForACL(ctx, node.Network)
	if len(devices) == 0 {
		return
	}
	egs, _ := listEgressForUserDeviceACL(ctx, node.Network)
	defaultUserPolicy, _ := getDefaultUserPolicyForUserDeviceACL(ctx, schema.NetworkID(node.Network))
	for i := range devices {
		device := devices[i]
		owner := logic.NodeOwnerUsername(&device)
		if owner == "" {
			continue
		}
		src4, src6 := logic.UserDeviceSrcIPs(&device)
		if defaultUserPolicy.Enabled {
			if src4.IP != nil {
				rules = append(rules, models.FwRule{
					SrcIP:           src4,
					AllowedProtocol: models.ALL,
					Allow:           true,
				})
			}
			if src6.IP != nil {
				rules = append(rules, models.FwRule{
					SrcIP:           src6,
					AllowedProtocol: models.ALL,
					Allow:           true,
				})
			}
			continue
		}
		egressPolicies := make(map[string]models.Acl)
		for _, peer := range nodes {
			if peer.IsUserNode || logic.IsUserOwnedDevice(&peer) || peer.ID == device.ID {
				continue
			}
			allowed, allowedPolicies := logic.IsUserAllowedToCommunicate(ctx, owner, peer)
			if !allowed {
				continue
			}
			if peer.IsStatic {
				peer = models.ConvertToStaticNode(peer.StaticNode)
			}
			for _, policy := range allowedPolicies {
				egressPolicies[policy.ID] = policy
				if src4.IP != nil && peer.Address.IP != nil {
					rules = append(rules, userDeviceFwRule(src4, peer.AddressIPNet4(), policy))
				}
				if src6.IP != nil && peer.Address6.IP != nil {
					rules = append(rules, userDeviceFwRule(src6, peer.AddressIPNet6(), policy))
				}
				// Extclient additional allowed IPs (LAN/CIDRs behind the client).
				for _, extra := range peer.StaticNode.ExtraAllowedIPs {
					_, ipNet, err := net.ParseCIDR(extra)
					if err != nil || ipNet == nil {
						continue
					}
					if ipNet.IP.To4() != nil {
						if src4.IP != nil {
							rules = append(rules, userDeviceFwRule(src4, *ipNet, policy))
						}
					} else if src6.IP != nil {
						rules = append(rules, userDeviceFwRule(src6, *ipNet, policy))
					}
				}
			}
		}
		for _, policy := range egressPolicies {
			var dst4, dst6 []net.IPNet
			appendUserDevicePolicyEgressDsts(ctx, &node, policy, egs, &dst4, &dst6)
			if src4.IP != nil {
				for _, cidr := range logic.UniqueIPNetList(dst4) {
					rules = append(rules, userDeviceFwRule(src4, cidr, policy))
				}
			}
			if src6.IP != nil {
				for _, cidr := range logic.UniqueIPNetList(dst6) {
					rules = append(rules, userDeviceFwRule(src6, cidr, policy))
				}
			}
		}
	}
	return
}

func userDeviceFwRule(src, dst net.IPNet, policy models.Acl) models.FwRule {
	return models.FwRule{
		SrcIP:           src,
		DstIP:           dst,
		AllowedProtocol: policy.Proto,
		AllowedPorts:    policy.Port,
		Allow:           true,
	}
}
