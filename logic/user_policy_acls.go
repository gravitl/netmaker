package logic

import (
	"context"
	"maps"
	"net"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func init() {
	IsUserAllowedToCommunicate = isUserAllowedToCommunicate
	RemoveUserFromAclPolicy = removeUserFromAclPolicy
	GetFwRulesForUserNodesOnGw = getFwRulesForUserNodesOnGw
	GetUserAclRulesForNode = getUserAclRulesForNode
	GetEgressUserRulesForNode = getEgressUserRulesForNode
	GetUserDeviceAclRulesForNode = getUserDeviceAclRulesForNode
	GetUserDeviceEgressRulesForNode = getUserDeviceEgressRulesForNode
	GetFwRulesForUserDevicesOnGw = getFwRulesForUserDevicesOnGw
}

// listNetworkAccessPoliciesForUser returns network-access user policies whose
// source includes userName or user:*. Group sources are ignored (CE has no groups).
func listNetworkAccessPoliciesForUser(ctx context.Context, userName string, netID schema.NetworkID) []models.Acl {
	allAcls := ListAcls(ctx)
	var userAcls []models.Acl
	for _, acl := range allAcls {
		if acl.NetworkID != netID || !acl.IsNetworkAccess() || acl.RuleType != models.UserPolicy {
			continue
		}
		srcMap := ConvAclTagToValueMap(acl.Src)
		if _, ok := srcMap["*"]; ok {
			userAcls = append(userAcls, acl)
			continue
		}
		if _, ok := srcMap[userName]; ok {
			userAcls = append(userAcls, acl)
		}
	}
	return userAcls
}

func isUserAllowedToCommunicate(ctx context.Context, userName string, peer models.Node) (bool, []models.Acl) {
	var peerId string
	if peer.IsStatic {
		peerId = peer.StaticNode.ClientID
		peer = models.ConvertToStaticNode(peer.StaticNode)
	} else {
		peerId = peer.ID.String()
	}

	var peerTags map[models.TagID]struct{}
	if peer.Mutex != nil {
		peer.Mutex.Lock()
		peerTags = maps.Clone(peer.Tags)
		peer.Mutex.Unlock()
	} else {
		peerTags = maps.Clone(peer.Tags)
	}
	if peerTags == nil {
		peerTags = make(map[models.TagID]struct{})
	}
	peerTags[models.TagID(peerId)] = struct{}{}
	peerTags[models.TagID("*")] = struct{}{}

	acl, _ := GetDefaultPolicy(ctx, schema.NetworkID(peer.Network), models.UserPolicy)
	if acl.Enabled {
		return true, []models.Acl{acl}
	}

	allowedPolicies := []models.Acl{}
	policies := listNetworkAccessPoliciesForUser(ctx, userName, schema.NetworkID(peer.Network))
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		dstMap := ConvAclTagToValueMap(policy.Dst)
		for _, dst := range policy.Dst {
			if dst.ID != models.EgressID {
				continue
			}
			e := schema.Egress{ID: dst.Value}
			if err := e.Get(ctx); err != nil || !e.Status {
				continue
			}
			for nodeID := range e.Nodes {
				dstMap[nodeID] = struct{}{}
			}
			for tagID := range e.Tags {
				dstMap[tagID] = struct{}{}
			}
		}
		if _, ok := dstMap["*"]; ok {
			allowedPolicies = append(allowedPolicies, policy)
			continue
		}
		if _, ok := dstMap[peer.ID.String()]; ok {
			allowedPolicies = append(allowedPolicies, policy)
			continue
		}
		if _, ok := dstMap[peerId]; ok {
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
	if peer.IsIngressGateway {
		if ok, policies := userAllowedToAnyExtClientOnIngress(ctx, userName, peer); ok {
			return true, policies
		}
	}
	return false, nil
}

// listExtClientsForUserACL lists extclients for user↔ingress ACL bridging; tests may override.
var listExtClientsForUserACL = func(ctx context.Context, network string) ([]models.ExtClient, error) {
	return GetNetworkExtClients(ctx, network)
}

func userAllowedToAnyExtClientOnIngress(ctx context.Context, userName string, ingress models.Node) (bool, []models.Acl) {
	if !ingress.IsIngressGateway {
		return false, nil
	}
	extclients, err := listExtClientsForUserACL(ctx, ingress.Network)
	if err != nil {
		return false, nil
	}
	ingressID := ingress.ID.String()
	seen := make(map[string]struct{})
	var allowed []models.Acl
	for i := range extclients {
		ec := extclients[i]
		if !ec.Enabled || ec.IngressGatewayID != ingressID || ec.RemoteAccessClientID != "" {
			continue
		}
		ok, policies := IsUserAllowedToCommunicate(ctx, userName, models.ConvertToStaticNode(ec))
		if !ok {
			continue
		}
		for _, p := range policies {
			if _, exists := seen[p.ID]; exists {
				continue
			}
			seen[p.ID] = struct{}{}
			allowed = append(allowed, p)
		}
	}
	return len(allowed) > 0, allowed
}

func removeUserFromAclPolicy(ctx context.Context, userName string) {
	acls := ListAcls(ctx)
	for _, acl := range acls {
		if acl.RuleType != models.UserPolicy {
			continue
		}
		deletePolicy := false
		update := false
		for i := len(acl.Src) - 1; i >= 0; i-- {
			if acl.Src[i].ID == models.UserAclID && acl.Src[i].Value == userName {
				if len(acl.Src) == 1 {
					deletePolicy = true
					break
				}
				acl.Src = append(acl.Src[:i], acl.Src[i+1:]...)
				update = true
			}
		}
		if deletePolicy {
			DeleteAcl(ctx, acl)
			continue
		}
		if update {
			UpsertAcl(ctx, acl)
		}
	}
}

func getSelectedUserEgressIPNets(dstTags []models.AclPolicyTag) (dst4, dst6 []net.IPNet) {
	for _, dst := range dstTags {
		if dst.ID != models.NetmakerIPAclID {
			continue
		}
		normalized, err := NormalizeIPOrCIDR(dst.Value)
		if err != nil {
			continue
		}
		ip, cidr, err := net.ParseCIDR(normalized)
		if err != nil {
			continue
		}
		if ip.To4() != nil {
			dst4 = append(dst4, *cidr)
		} else {
			dst6 = append(dst6, *cidr)
		}
	}
	return
}

func aclsForUserOwner(allowedUsers map[string][]models.Acl, owner string) []models.Acl {
	if owner == "" {
		return append([]models.Acl(nil), allowedUsers["*"]...)
	}
	out := append([]models.Acl(nil), allowedUsers[owner]...)
	out = append(out, allowedUsers["*"]...)
	return out
}

func collectUserAclSources(acl models.Acl, allowedUsers map[string][]models.Acl) {
	for _, srcAcl := range acl.Src {
		if srcAcl.ID == models.UserAclID {
			allowedUsers[srcAcl.Value] = append(allowedUsers[srcAcl.Value], acl)
		}
	}
}

func getFwRulesForUserNodesOnGw(ctx context.Context, node models.Node, nodes []models.Node) (rules []models.FwRule) {
	defaultUserPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(node.Network), models.UserPolicy)
	userNodes := GetStaticUserNodesByNetwork(ctx, schema.NetworkID(node.Network))
	for _, userNodeI := range userNodes {
		if !userNodeI.StaticNode.Enabled {
			continue
		}
		if defaultUserPolicy.Enabled {
			if userNodeI.StaticNode.Address != "" {
				rules = append(rules, models.FwRule{
					SrcIP:           userNodeI.StaticNode.AddressIPNet4(),
					DstIP:           net.IPNet{},
					AllowedProtocol: models.ALL,
					AllowedPorts:    []string{},
					Allow:           true,
				})
			}
			if userNodeI.StaticNode.Address6 != "" {
				rules = append(rules, models.FwRule{
					SrcIP:           userNodeI.StaticNode.AddressIPNet6(),
					DstIP:           net.IPNet{},
					AllowedProtocol: models.ALL,
					AllowedPorts:    []string{},
					Allow:           true,
				})
			}
			continue
		}
		for _, peer := range nodes {
			if peer.IsUserNode || IsUserOwnedDevice(&peer) {
				continue
			}
			ok, allowedPolicies := IsUserAllowedToCommunicate(ctx, userNodeI.StaticNode.OwnerID, peer)
			if !ok {
				continue
			}
			if peer.IsStatic {
				peer = models.ConvertToStaticNode(peer.StaticNode)
			}
			for _, policy := range allowedPolicies {
				if userNodeI.StaticNode.Address != "" && peer.Address.IP != nil {
					rules = append(rules, models.FwRule{
						SrcIP: userNodeI.StaticNode.AddressIPNet4(),
						DstIP: net.IPNet{
							IP:   peer.Address.IP,
							Mask: net.CIDRMask(32, 32),
						},
						AllowedProtocol: policy.Proto,
						AllowedPorts:    policy.Port,
						Allow:           true,
					})
				}
				if userNodeI.StaticNode.Address6 != "" && peer.Address6.IP != nil {
					rules = append(rules, models.FwRule{
						SrcIP: userNodeI.StaticNode.AddressIPNet6(),
						DstIP: net.IPNet{
							IP:   peer.Address6.IP,
							Mask: net.CIDRMask(128, 128),
						},
						AllowedProtocol: policy.Proto,
						AllowedPorts:    policy.Port,
						Allow:           true,
					})
				}
				for _, additionalAllowedIPNet := range peer.StaticNode.ExtraAllowedIPs {
					_, ipNet, err := net.ParseCIDR(additionalAllowedIPNet)
					if err != nil {
						continue
					}
					if ipNet.IP.To4() != nil {
						if userNodeI.StaticNode.Address != "" {
							rules = append(rules, models.FwRule{
								SrcIP:           userNodeI.StaticNode.AddressIPNet4(),
								DstIP:           *ipNet,
								AllowedProtocol: policy.Proto,
								AllowedPorts:    policy.Port,
								Allow:           true,
							})
						}
					} else if userNodeI.StaticNode.Address6 != "" {
						rules = append(rules, models.FwRule{
							SrcIP:           userNodeI.StaticNode.AddressIPNet6(),
							DstIP:           *ipNet,
							AllowedProtocol: policy.Proto,
							AllowedPorts:    policy.Port,
							Allow:           true,
						})
					}
				}
				for _, dstI := range policy.Dst {
					if dstI.Value == "*" {
						if userNodeI.StaticNode.Address != "" {
							rules = append(rules, models.FwRule{
								SrcIP:           userNodeI.StaticNode.AddressIPNet4(),
								DstIP:           net.IPNet{},
								AllowedProtocol: policy.Proto,
								AllowedPorts:    policy.Port,
								Allow:           true,
							})
						}
						if userNodeI.StaticNode.Address6 != "" {
							rules = append(rules, models.FwRule{
								SrcIP:           userNodeI.StaticNode.AddressIPNet6(),
								DstIP:           net.IPNet{},
								AllowedProtocol: policy.Proto,
								AllowedPorts:    policy.Port,
								Allow:           true,
							})
						}
						break
					}
					if dstI.ID != models.EgressID {
						continue
					}
					e := schema.Egress{ID: dstI.Value}
					if err := e.Get(ctx); err != nil || !e.Status {
						continue
					}
					selectedIP4, selectedIP6 := getSelectedUserEgressIPNets(policy.Dst)
					if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
						for _, dst := range selectedIP4 {
							if userNodeI.StaticNode.Address != "" {
								rules = append(rules, models.FwRule{
									SrcIP:           userNodeI.StaticNode.AddressIPNet4(),
									DstIP:           dst,
									AllowedProtocol: policy.Proto,
									AllowedPorts:    policy.Port,
									Allow:           true,
								})
							}
						}
						for _, dst := range selectedIP6 {
							if userNodeI.StaticNode.Address6 != "" {
								rules = append(rules, models.FwRule{
									SrcIP:           userNodeI.StaticNode.AddressIPNet6(),
									DstIP:           dst,
									AllowedProtocol: policy.Proto,
									AllowedPorts:    policy.Port,
									Allow:           true,
								})
							}
						}
						continue
					}
					egressRange := e.Range
					if _, ok := e.Nodes[node.ID.String()]; !ok && e.VirtualRange != "" {
						egressRange = e.VirtualRange
					}
					var dst4, dst6 []net.IPNet
					AppendEgressPolicyRange(e, egressRange, &dst4, &dst6)
					for _, dst := range dst4 {
						if userNodeI.StaticNode.Address != "" {
							rules = append(rules, models.FwRule{
								SrcIP:           userNodeI.StaticNode.AddressIPNet4(),
								DstIP:           dst,
								AllowedProtocol: policy.Proto,
								AllowedPorts:    policy.Port,
								Allow:           true,
							})
						}
					}
					for _, dst := range dst6 {
						if userNodeI.StaticNode.Address6 != "" {
							rules = append(rules, models.FwRule{
								SrcIP:           userNodeI.StaticNode.AddressIPNet6(),
								DstIP:           dst,
								AllowedProtocol: policy.Proto,
								AllowedPorts:    policy.Port,
								Allow:           true,
							})
						}
					}
				}
			}
		}
	}
	return rules
}

func getUserAclRulesForNode(ctx context.Context, targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	userNodes := GetStaticUserNodesByNetwork(ctx, schema.NetworkID(targetnode.Network))
	allowedUsers := make(map[string][]models.Acl)
	acls := ListNetworkAccessUserPolicies(ctx, schema.NetworkID(targetnode.Network))
	var targetNodeTags map[models.TagID]struct{}
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
	defaultPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(targetnode.Network), models.UserPolicy)
	targetNodeTags[models.TagID(targetnode.ID.String())] = struct{}{}
	if !defaultPolicy.Enabled {
		for _, acl := range acls {
			if !acl.Enabled {
				continue
			}
			dstTags := ConvAclTagToValueMap(acl.Dst)
			_, all := dstTags["*"]
			addUsers := all
			if !all {
				for _, dst := range acl.Dst {
					if dst.ID != models.EgressID {
						continue
					}
					e := schema.Egress{ID: dst.Value}
					if err := e.Get(ctx); err == nil && e.Status {
						if _, ok := e.Nodes[targetnode.ID.String()]; ok {
							dstTags[targetnode.ID.String()] = struct{}{}
						}
					}
				}
				for nodeTag := range targetNodeTags {
					if _, ok := dstTags[nodeTag.String()]; ok {
						addUsers = true
						break
					}
				}
			}
			if addUsers {
				collectUserAclSources(acl, allowedUsers)
			}
		}
	}
	if defaultPolicy.Enabled {
		r := models.AclRule{
			ID:              defaultPolicy.ID,
			AllowedProtocol: defaultPolicy.Proto,
			AllowedPorts:    defaultPolicy.Port,
			Direction:       defaultPolicy.AllowedDirection,
			Allowed:         true,
		}
		for _, userNode := range userNodes {
			if !userNode.StaticNode.Enabled {
				continue
			}
			if userNode.StaticNode.Address != "" {
				r.IPList = append(r.IPList, userNode.StaticNode.AddressIPNet4())
			}
			if userNode.StaticNode.Address6 != "" {
				r.IP6List = append(r.IP6List, userNode.StaticNode.AddressIPNet6())
			}
		}
		rules[defaultPolicy.ID] = r
		return rules
	}
	for _, userNode := range userNodes {
		if !userNode.StaticNode.Enabled {
			continue
		}
		ownerAcls := aclsForUserOwner(allowedUsers, userNode.StaticNode.OwnerID)
		for _, acl := range ownerAcls {
			if !acl.Enabled {
				continue
			}
			r := models.AclRule{
				ID:              acl.ID,
				AllowedProtocol: acl.Proto,
				AllowedPorts:    acl.Port,
				Direction:       acl.AllowedDirection,
				Dst:             []net.IPNet{targetnode.AddressIPNet4()},
				Dst6:            []net.IPNet{targetnode.AddressIPNet6()},
				Allowed:         true,
			}
			if userNode.StaticNode.Address != "" {
				r.IPList = append(r.IPList, userNode.StaticNode.AddressIPNet4())
			}
			if userNode.StaticNode.Address6 != "" {
				r.IP6List = append(r.IP6List, userNode.StaticNode.AddressIPNet6())
			}
			if existing, ok := rules[acl.ID]; ok {
				existing.IPList = UniqueIPNetList(append(existing.IPList, r.IPList...))
				existing.IP6List = UniqueIPNetList(append(existing.IP6List, r.IP6List...))
				existing.Dst = UniqueIPNetList(append(existing.Dst, r.Dst...))
				existing.Dst6 = UniqueIPNetList(append(existing.Dst6, r.Dst6...))
				rules[acl.ID] = existing
			} else {
				r.IPList = UniqueIPNetList(r.IPList)
				r.IP6List = UniqueIPNetList(r.IP6List)
				rules[acl.ID] = r
			}
		}
	}
	return rules
}

func getEgressUserRulesForNode(ctx context.Context, targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	userNodes := GetStaticUserNodesByNetwork(ctx, schema.NetworkID(targetnode.Network))
	allowedUsers := make(map[string][]models.Acl)
	acls := ListNetworkAccessUserPolicies(ctx, schema.NetworkID(targetnode.Network))
	egs, _ := (&schema.Egress{Network: targetnode.Network}).ListByNetwork(ctx)
	if len(egs) == 0 {
		return rules
	}
	defaultPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(targetnode.Network), models.UserPolicy)
	targetNodeTags := map[models.TagID]struct{}{"*": {}}
	for _, egI := range egs {
		if !egI.Status {
			continue
		}
		if _, ok := egI.Nodes[targetnode.ID.String()]; !ok {
			continue
		}
		if egI.Range != "" {
			targetNodeTags[models.TagID(egI.Range)] = struct{}{}
		}
		targetNodeTags[models.TagID(egI.ID)] = struct{}{}
	}
	if !defaultPolicy.Enabled {
		for _, acl := range acls {
			if !acl.Enabled {
				continue
			}
			dstTags := ConvAclTagToValueMap(acl.Dst)
			_, all := dstTags["*"]
			addUsers := all
			if !all {
				for nodeTag := range targetNodeTags {
					if _, ok := dstTags[nodeTag.String()]; ok {
						addUsers = true
						break
					}
				}
			}
			if addUsers {
				collectUserAclSources(acl, allowedUsers)
			}
		}
	}
	if defaultPolicy.Enabled {
		r := models.AclRule{
			ID:              defaultPolicy.ID,
			AllowedProtocol: defaultPolicy.Proto,
			AllowedPorts:    defaultPolicy.Port,
			Direction:       defaultPolicy.AllowedDirection,
			Allowed:         true,
		}
		for _, userNode := range userNodes {
			if !userNode.StaticNode.Enabled {
				continue
			}
			if userNode.StaticNode.Address != "" {
				r.IPList = append(r.IPList, userNode.StaticNode.AddressIPNet4())
			}
			if userNode.StaticNode.Address6 != "" {
				r.IP6List = append(r.IP6List, userNode.StaticNode.AddressIPNet6())
			}
		}
		rules[defaultPolicy.ID] = r
		return rules
	}
	for _, userNode := range userNodes {
		if !userNode.StaticNode.Enabled {
			continue
		}
		ownerAcls := aclsForUserOwner(allowedUsers, userNode.StaticNode.OwnerID)
		for _, acl := range ownerAcls {
			if !acl.Enabled {
				continue
			}
			selectedIP4, selectedIP6 := getSelectedUserEgressIPNets(acl.Dst)
			r := models.AclRule{
				ID:              acl.ID,
				AllowedProtocol: acl.Proto,
				AllowedPorts:    acl.Port,
				Direction:       acl.AllowedDirection,
				Allowed:         true,
			}
			if userNode.StaticNode.Address != "" {
				r.IPList = append(r.IPList, userNode.StaticNode.AddressIPNet4())
			}
			if userNode.StaticNode.Address6 != "" {
				r.IP6List = append(r.IP6List, userNode.StaticNode.AddressIPNet6())
			}
			for _, dstI := range acl.Dst {
				if dstI.ID != models.EgressID {
					continue
				}
				e := schema.Egress{ID: dstI.Value}
				if err := e.Get(ctx); err != nil || !e.Status {
					continue
				}
				nodeOwnsEgress := false
				if _, ok := e.Nodes[targetnode.ID.String()]; ok {
					nodeOwnsEgress = true
				}
				egressRange := e.Range
				if !nodeOwnsEgress && e.VirtualRange != "" {
					egressRange = e.VirtualRange
				}
				if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
					r.Dst = append(r.Dst, selectedIP4...)
					r.Dst6 = append(r.Dst6, selectedIP6...)
					continue
				}
				AppendEgressPolicyRange(e, egressRange, &r.Dst, &r.Dst6)
			}
			if existing, ok := rules[acl.ID]; ok {
				existing.IPList = UniqueIPNetList(append(existing.IPList, r.IPList...))
				existing.IP6List = UniqueIPNetList(append(existing.IP6List, r.IP6List...))
				existing.Dst = UniqueIPNetList(append(existing.Dst, r.Dst...))
				existing.Dst6 = UniqueIPNetList(append(existing.Dst6, r.Dst6...))
				rules[acl.ID] = existing
			} else {
				r.IPList = UniqueIPNetList(r.IPList)
				r.IP6List = UniqueIPNetList(r.IP6List)
				r.Dst = UniqueIPNetList(r.Dst)
				r.Dst6 = UniqueIPNetList(r.Dst6)
				rules[acl.ID] = r
			}
		}
	}
	return rules
}

const userDeviceRuleSuffix = "#user-device"

func getUserDeviceAclRulesForNode(_ context.Context, _ *PeerGraph, targetnode *models.Node,
	networkNodes []models.Node, rules map[string]models.AclRule) map[string]models.AclRule {
	return addUserDeviceRules(targetnode.Network, targetnode, networkNodes, rules, false)
}

func getUserDeviceEgressRulesForNode(_ context.Context, targetnode *models.Node,
	networkNodes []models.Node, rules map[string]models.AclRule) map[string]models.AclRule {
	return addUserDeviceRules(targetnode.Network, targetnode, networkNodes, rules, true)
}

func filterUserOwnedDevices(nodes []models.Node) []models.Node {
	var out []models.Node
	for i := range nodes {
		if IsUserOwnedDevice(&nodes[i]) {
			out = append(out, nodes[i])
		}
	}
	return out
}

func addUserDeviceRules(network string, targetnode *models.Node, networkNodes []models.Node,
	rules map[string]models.AclRule, egressOnly bool) map[string]models.AclRule {
	ctx := context.TODO()
	devices := filterUserOwnedDevices(networkNodes)
	if len(devices) == 0 {
		return rules
	}
	defaultPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(network), models.UserPolicy)
	if defaultPolicy.Enabled {
		src4, src6 := allUserDeviceSrcIPs(devices)
		if len(src4) == 0 && len(src6) == 0 {
			return rules
		}
		id := defaultPolicy.ID + userDeviceRuleSuffix
		r := models.AclRule{
			ID:              id,
			AllowedProtocol: defaultPolicy.Proto,
			AllowedPorts:    defaultPolicy.Port,
			Direction:       defaultPolicy.AllowedDirection,
			Allowed:         true,
			IPList:          src4,
			IP6List:         src6,
		}
		if !egressOnly {
			r.Dst = []net.IPNet{targetnode.AddressIPNet4()}
			r.Dst6 = []net.IPNet{targetnode.AddressIPNet6()}
		}
		rules[id] = r
		return rules
	}
	acls := ListNetworkAccessUserPolicies(ctx, schema.NetworkID(network))
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		src4, src6 := userDeviceSrcIPsForPolicy(acl, devices)
		if len(src4) == 0 && len(src6) == 0 {
			continue
		}
		id := acl.ID + userDeviceRuleSuffix
		r := models.AclRule{
			ID:              id,
			AllowedProtocol: acl.Proto,
			AllowedPorts:    acl.Port,
			Direction:       acl.AllowedDirection,
			Allowed:         true,
			IPList:          src4,
			IP6List:         src6,
		}
		if !egressOnly {
			r.Dst = []net.IPNet{targetnode.AddressIPNet4()}
			r.Dst6 = []net.IPNet{targetnode.AddressIPNet6()}
		} else {
			selectedIP4, selectedIP6 := getSelectedUserEgressIPNets(acl.Dst)
			for _, dstI := range acl.Dst {
				if dstI.ID != models.EgressID && dstI.Value != "*" {
					continue
				}
				if dstI.Value == "*" {
					egs, _ := (&schema.Egress{Network: network}).ListByNetwork(ctx)
					for _, e := range egs {
						if !e.Status {
							continue
						}
						if _, ok := e.Nodes[targetnode.ID.String()]; !ok {
							continue
						}
						AppendEgressPolicyRange(e, e.Range, &r.Dst, &r.Dst6)
					}
					break
				}
				e := schema.Egress{ID: dstI.Value}
				if err := e.Get(ctx); err != nil || !e.Status {
					continue
				}
				if _, ok := e.Nodes[targetnode.ID.String()]; !ok {
					continue
				}
				if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
					r.Dst = append(r.Dst, selectedIP4...)
					r.Dst6 = append(r.Dst6, selectedIP6...)
					continue
				}
				AppendEgressPolicyRange(e, e.Range, &r.Dst, &r.Dst6)
			}
			if len(r.Dst) == 0 && len(r.Dst6) == 0 {
				continue
			}
		}
		rules[id] = r
	}
	return rules
}

func userDeviceSrcIPsForPolicy(policy models.Acl, devices []models.Node) (src4, src6 []net.IPNet) {
	owners := make(map[string]struct{})
	for _, srcAcl := range policy.Src {
		if srcAcl.ID != models.UserAclID {
			continue
		}
		if srcAcl.Value == "*" {
			return allUserDeviceSrcIPs(devices)
		}
		owners[srcAcl.Value] = struct{}{}
	}
	if len(owners) == 0 {
		return
	}
	for i := range devices {
		if _, ok := owners[NodeOwnerUsername(&devices[i])]; !ok {
			continue
		}
		deviceSrc4, deviceSrc6 := UserDeviceSrcIPs(&devices[i])
		if deviceSrc4.IP != nil {
			src4 = append(src4, deviceSrc4)
		}
		if deviceSrc6.IP != nil {
			src6 = append(src6, deviceSrc6)
		}
	}
	return
}

func allUserDeviceSrcIPs(devices []models.Node) (src4, src6 []net.IPNet) {
	for i := range devices {
		deviceSrc4, deviceSrc6 := UserDeviceSrcIPs(&devices[i])
		if deviceSrc4.IP != nil {
			src4 = append(src4, deviceSrc4)
		}
		if deviceSrc6.IP != nil {
			src6 = append(src6, deviceSrc6)
		}
	}
	return
}

func getFwRulesForUserDevicesOnGw(ctx context.Context, node models.Node, nodes []models.Node) (rules []models.FwRule) {
	defaultUserPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(node.Network), models.UserPolicy)
	devices := filterUserOwnedDevices(nodes)
	for i := range devices {
		dev := &devices[i]
		src4, src6 := UserDeviceSrcIPs(dev)
		owner := NodeOwnerUsername(dev)
		if defaultUserPolicy.Enabled {
			if src4.IP != nil {
				rules = append(rules, models.FwRule{
					SrcIP: src4, DstIP: net.IPNet{}, AllowedProtocol: models.ALL, Allow: true,
				})
			}
			if src6.IP != nil {
				rules = append(rules, models.FwRule{
					SrcIP: src6, DstIP: net.IPNet{}, AllowedProtocol: models.ALL, Allow: true,
				})
			}
			continue
		}
		for _, peer := range nodes {
			if peer.ID == dev.ID || IsUserOwnedDevice(&peer) || peer.IsUserNode {
				continue
			}
			ok, allowedPolicies := IsUserAllowedToCommunicate(ctx, owner, peer)
			if !ok {
				continue
			}
			for _, policy := range allowedPolicies {
				if src4.IP != nil && peer.Address.IP != nil {
					rules = append(rules, models.FwRule{
						SrcIP: src4,
						DstIP: net.IPNet{IP: peer.Address.IP, Mask: net.CIDRMask(32, 32)},
						AllowedProtocol: policy.Proto,
						AllowedPorts:    policy.Port,
						Allow:           true,
					})
				}
				if src6.IP != nil && peer.Address6.IP != nil {
					rules = append(rules, models.FwRule{
						SrcIP: src6,
						DstIP: net.IPNet{IP: peer.Address6.IP, Mask: net.CIDRMask(128, 128)},
						AllowedProtocol: policy.Proto,
						AllowedPorts:    policy.Port,
						Allow:           true,
					})
				}
			}
		}
	}
	return rules
}
