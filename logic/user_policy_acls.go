package logic

import (
	"context"
	"fmt"
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
}

// Default group IDs; must match the slugs created in pro/logic/user_mgmt.go.
var (
	globalNetworksAdminGroupID = schema.UserGroupID(fmt.Sprintf("global-%s-grp", schema.NetworkAdmin))
	globalNetworksUserGroupID  = schema.UserGroupID(fmt.Sprintf("global-%s-grp", schema.NetworkUser))
)

func defaultNetworkGroupID(netID schema.NetworkID, role schema.UserRoleID) schema.UserGroupID {
	return schema.UserGroupID(fmt.Sprintf("%s-%s-grp", netID, role))
}

// userGroupsForNetwork includes members of the global admin and user groups in
// the network's default groups, matching listNetworkAccessPoliciesOfUser.
// GetUserGrpMap is empty on CE and populated by Pro.
func userGroupsForNetwork(ctx context.Context, netID schema.NetworkID) map[schema.UserGroupID]map[string]struct{} {
	grp := GetUserGrpMap(ctx)
	copyGroupMembers(grp, globalNetworksAdminGroupID, defaultNetworkGroupID(netID, schema.NetworkAdmin))
	copyGroupMembers(grp, globalNetworksUserGroupID, defaultNetworkGroupID(netID, schema.NetworkUser))
	return grp
}

func copyGroupMembers(grp map[schema.UserGroupID]map[string]struct{}, from, to schema.UserGroupID) {
	src, ok := grp[from]
	if !ok || from == to {
		return
	}
	dst := grp[to]
	if dst == nil {
		dst = make(map[string]struct{})
	}
	for userName := range src {
		dst[userName] = struct{}{}
	}
	grp[to] = dst
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

			if ok, allowedPolicies := IsUserAllowedToCommunicate(ctx, userNodeI.StaticNode.OwnerID, peer); ok {
				if peer.IsStatic {
					peer = models.ConvertToStaticNode(peer.StaticNode)
				}
				for _, policy := range allowedPolicies {
					selectedIP4, selectedIP6 := getSelectedEgressIPNets(policy.Dst)
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

					// Static extclients may advertise ExtraAllowedIPs (egress ranges).
					// AllowedIPs on the user client already include them via
					// GetEgressRangesOnNetwork; without matching forward rules here the
					// ingress gateway drops user -> ExtraAllowedIP traffic even when the
					// user policy grants access to that extclient (device policies already
					// emit these via GetFwRulesForNodeAndPeerOnGw).
					if len(peer.StaticNode.ExtraAllowedIPs) > 0 {
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
					}

					// add egress ranges
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
						if dstI.ID == models.EgressID {

							e := schema.Egress{ID: dstI.Value}
							err := e.Get(ctx)
							if err != nil || !e.Status {
								continue
							}
							nodeOwnsEgress := false
							if _, ok := e.Nodes[node.ID.String()]; ok {
								nodeOwnsEgress = true
							}
							// Use virtual range if peer node doesn't own the egress, otherwise use regular range
							egressRange := e.Range
							if !nodeOwnsEgress && e.VirtualRange != "" {
								egressRange = e.VirtualRange
							}
							if egressRange != "" || IsEgressInternetGateway(e) {
								dstI.Value = egressRange

								if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
									for _, selectedCIDR := range selectedIP4 {
										if userNodeI.StaticNode.Address != "" {
											rules = append(rules, models.FwRule{
												SrcIP:           userNodeI.StaticNode.AddressIPNet4(),
												DstIP:           selectedCIDR,
												AllowedProtocol: policy.Proto,
												AllowedPorts:    policy.Port,
												Allow:           true,
											})
										}
									}
									for _, selectedCIDR := range selectedIP6 {
										if userNodeI.StaticNode.Address6 != "" {
											rules = append(rules, models.FwRule{
												SrcIP:           userNodeI.StaticNode.AddressIPNet6(),
												DstIP:           selectedCIDR,
												AllowedProtocol: policy.Proto,
												AllowedPorts:    policy.Port,
												Allow:           true,
											})
										}
									}
									continue
								}
								var dst4, dst6 []net.IPNet
								AppendEgressPolicyRange(e, egressRange, &dst4, &dst6)
								for _, cidr := range dst4 {
									if userNodeI.StaticNode.Address != "" {
										rules = append(rules, models.FwRule{
											SrcIP:           userNodeI.StaticNode.AddressIPNet4(),
											DstIP:           cidr,
											AllowedProtocol: policy.Proto,
											AllowedPorts:    policy.Port,
											Allow:           true,
										})
									}
								}
								for _, cidr := range dst6 {
									if userNodeI.StaticNode.Address6 != "" {
										rules = append(rules, models.FwRule{
											SrcIP:           userNodeI.StaticNode.AddressIPNet6(),
											DstIP:           cidr,
											AllowedProtocol: policy.Proto,
											AllowedPorts:    policy.Port,
											Allow:           true,
										})
									}
								}
							} else if HasEgressDomainAns(e) {
								for _, domainAns := range AllDomainAnsFromEgress(e) {
									dstI.Value = domainAns

									ip, cidr, err := net.ParseCIDR(dstI.Value)
									if err == nil {
										if ip.To4() != nil && userNodeI.StaticNode.Address != "" {
											rules = append(rules, models.FwRule{
												SrcIP:           userNodeI.StaticNode.AddressIPNet4(),
												DstIP:           *cidr,
												AllowedProtocol: policy.Proto,
												AllowedPorts:    policy.Port,
												Allow:           true,
											})
										} else if ip.To16() != nil && userNodeI.StaticNode.Address6 != "" {
											rules = append(rules, models.FwRule{
												SrcIP:           userNodeI.StaticNode.AddressIPNet6(),
												DstIP:           *cidr,
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

				}

			}
		}
	}
	return
}

// listNetworkAccessPoliciesOfUser - lists all network access user acl policies applied to user in an network
func listNetworkAccessPoliciesOfUser(ctx context.Context, user *schema.User, netID schema.NetworkID) []models.Acl {
	allAcls := ListAcls(ctx)
	var userAcls []models.Acl
	if _, ok := user.UserGroups.Data()[globalNetworksAdminGroupID]; ok {
		user.UserGroups.Data()[defaultNetworkGroupID(netID, schema.NetworkAdmin)] = struct{}{}
	}
	if _, ok := user.UserGroups.Data()[globalNetworksUserGroupID]; ok {
		user.UserGroups.Data()[defaultNetworkGroupID(netID, schema.NetworkUser)] = struct{}{}
	}
	for _, acl := range allAcls {
		if acl.NetworkID == netID && acl.IsNetworkAccess() && acl.RuleType == models.UserPolicy {
			srcMap := ConvAclTagToValueMap(acl.Src)
			if _, ok := srcMap["*"]; ok {
				userAcls = append(userAcls, acl)
				continue
			}
			if _, ok := srcMap[user.Username]; ok {
				userAcls = append(userAcls, acl)
				continue
			}
			for userG := range user.UserGroups.Data() {
				if _, ok := srcMap[userG.String()]; ok {
					userAcls = append(userAcls, acl)
					continue
				}
			}

		}
	}
	return userAcls
}

// aclsForUserOwner returns policies keyed to owner plus any user:* policies.
func aclsForUserOwner(allowedUsers map[string][]models.Acl, owner string) []models.Acl {
	if owner == "" {
		return append([]models.Acl(nil), allowedUsers["*"]...)
	}
	out := append([]models.Acl(nil), allowedUsers[owner]...)
	out = append(out, allowedUsers["*"]...)
	return out
}

// isUserAllowedToCommunicate - check if user is allowed to communicate with peer
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
	user := &schema.User{Username: userName}
	err := user.GetWithMembership(ctx)
	if err != nil {
		return false, []models.Acl{}
	}
	allowedPolicies := []models.Acl{}
	policies := listNetworkAccessPoliciesOfUser(ctx, user, schema.NetworkID(peer.Network))
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		dstMap := ConvAclTagToValueMap(policy.Dst)
		for _, dst := range policy.Dst {
			if dst.ID == models.EgressID {
				e := schema.Egress{ID: dst.Value}
				err := e.Get(ctx)
				if err == nil && e.Status {
					for nodeID := range e.Nodes {
						dstMap[nodeID] = struct{}{}
					}
					// Routing nodes can be attached to an egress by tag
					// instead of individually, in which case e.Nodes is
					// empty. peerTags below carries the peer's tags.
					for tagID := range e.Tags {
						dstMap[tagID] = struct{}{}
					}
				}
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
	// Host-backed user devices reach static/tagged extclients through the ingress
	// gateway. Allow the ingress itself when any attached (non-RAC) extclient is a
	// policy destination so peering and AllowedIPs via that gateway work.
	if peer.IsIngressGateway {
		if ok, policies := userAllowedToAnyExtClientOnIngress(ctx, userName, peer); ok {
			return true, policies
		}
	}
	return false, []models.Acl{}
}

// listExtClientsForUserACL lists extclients for user↔ingress ACL bridging; tests may override.
var listExtClientsForUserACL = func(ctx context.Context, network string) ([]models.ExtClient, error) {
	return GetNetworkExtClients(ctx, network)
}

// userAllowedToAnyExtClientOnIngress is true when userName may reach at least one
// enabled non-RAC extclient attached to ingress under a user policy.
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
		// Use the logic package hook so tests can stub nested static-peer checks
		// without re-entering this ingress bridge.
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
				DeleteAcl(ctx, acl)
				continue
			}
			if update {
				UpsertAcl(ctx, acl)
			}
		}
	}
}

func getEgressUserRulesForNode(ctx context.Context, targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	userNodes := GetStaticUserNodesByNetwork(ctx, schema.NetworkID(targetnode.Network))
	userGrpMap := userGroupsForNetwork(ctx, schema.NetworkID(targetnode.Network))
	allowedUsers := make(map[string][]models.Acl)
	acls := ListNetworkAccessUserPolicies(ctx, schema.NetworkID(targetnode.Network))
	var targetNodeTags = make(map[models.TagID]struct{})
	targetNodeTags["*"] = struct{}{}
	egs, _ := (&schema.Egress{Network: targetnode.Network}).ListByNetwork(ctx)
	if len(egs) == 0 {
		return rules
	}
	defaultPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(targetnode.Network), models.UserPolicy)

	for _, egI := range egs {
		if !egI.Status {
			continue
		}
		if _, ok := egI.Nodes[targetnode.ID.String()]; ok {
			if egI.Range != "" {
				targetNodeTags[models.TagID(egI.Range)] = struct{}{}
			} else if HasEgressDomainAns(egI) {
				for _, domainAnsI := range AllDomainAnsFromEgress(egI) {
					targetNodeTags[models.TagID(domainAnsI)] = struct{}{}
				}
			}

			targetNodeTags[models.TagID(egI.ID)] = struct{}{}
		}
	}
	if !defaultPolicy.Enabled {
		for _, acl := range acls {
			if !acl.Enabled {
				continue
			}
			dstTags := ConvAclTagToValueMap(acl.Dst)
			for _, dst := range acl.Dst {
				if dst.ID == models.EgressID {
					e := schema.Egress{ID: dst.Value}
					err := e.Get(ctx)
					if err == nil && e.Status {
						for nodeID := range e.Nodes {
							dstTags[nodeID] = struct{}{}
						}
						if e.Range != "" {
							dstTags[e.Range] = struct{}{}
						} else if HasEgressDomainAns(e) {
							for _, domainAnsI := range AllDomainAnsFromEgress(e) {
								dstTags[domainAnsI] = struct{}{}
							}
						}

					}
				}
			}
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
						if usersMap, ok := userGrpMap[schema.UserGroupID(srcAcl.Value)]; ok {
							for userName := range usersMap {
								allowedUsers[userName] = append(allowedUsers[userName], acl)
							}
						}
					}
				}
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

			// Get peers in the tags and add allowed rules
			if userNode.StaticNode.Address != "" {
				r.IPList = append(r.IPList, userNode.StaticNode.AddressIPNet4())
			}
			if userNode.StaticNode.Address6 != "" {
				r.IP6List = append(r.IP6List, userNode.StaticNode.AddressIPNet6())
			}
		}
		rules[defaultPolicy.ID] = r
	} else {
		for _, userNode := range userNodes {
			if !userNode.StaticNode.Enabled {
				continue
			}

			acls := aclsForUserOwner(allowedUsers, userNode.StaticNode.OwnerID)
			if len(acls) == 0 {
				continue
			}
			for _, acl := range acls {

				if !acl.Enabled {
					continue
				}
				selectedIP4, selectedIP6 := getSelectedEgressIPNets(acl.Dst)
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
						err := e.Get(ctx)
						if err != nil {
							continue
						}
						nodeOwnsEgress := false
						if _, ok := e.Nodes[targetnode.ID.String()]; ok {
							nodeOwnsEgress = true
						}
						// Use virtual range if target node doesn't own the egress, otherwise use regular range
						egressRange := e.Range
						if !nodeOwnsEgress && e.VirtualRange != "" {
							egressRange = e.VirtualRange
						}
						if egressRange != "" || IsEgressInternetGateway(e) {
							if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
								r.Dst = append(r.Dst, selectedIP4...)
								r.Dst6 = append(r.Dst6, selectedIP6...)
								continue
							}
							AppendEgressPolicyRange(e, egressRange, &r.Dst, &r.Dst6)
						} else if HasEgressDomainAns(e) {
							for _, domainAnsI := range AllDomainAnsFromEgress(e) {
								ip, cidr, err := net.ParseCIDR(domainAnsI)
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

				}
				if userNode.StaticNode.Address6 != "" {
					r.IP6List = append(r.IP6List, userNode.StaticNode.AddressIPNet6())
				}
				if aclRule, ok := rules[acl.ID]; ok {

					aclRule.IPList = append(aclRule.IPList, r.IPList...)
					aclRule.IP6List = append(aclRule.IP6List, r.IP6List...)

					aclRule.Dst = append(aclRule.Dst, r.Dst...)
					aclRule.Dst6 = append(aclRule.Dst6, r.Dst6...)

					aclRule.IPList = UniqueIPNetList(aclRule.IPList)
					aclRule.IP6List = UniqueIPNetList(aclRule.IP6List)

					aclRule.Dst = UniqueIPNetList(aclRule.Dst)
					aclRule.Dst6 = UniqueIPNetList(aclRule.Dst6)

					rules[acl.ID] = aclRule
				} else {
					r.IPList = UniqueIPNetList(r.IPList)
					r.IP6List = UniqueIPNetList(r.IP6List)

					r.Dst = UniqueIPNetList(r.Dst)
					r.Dst6 = UniqueIPNetList(r.Dst6)
					rules[acl.ID] = r
				}
			}

		}

		appendUserExtClientRemoteEgressFwdRules(targetnode, acls, egs, userNodes, userGrpMap, rules)
	}

	return rules
}

// appendUserExtClientRemoteEgressFwdRules mirrors the device-path fix from logic/acls.go
// for user-policy static extclients: when a user is permitted to reach an egress
// hosted on a DIFFERENT node, targetnode's forward chain needs an explicit allow rule
// or the packet is dropped here even though the ingress chain permitted it. Emitted
// rules are keyed with the "#ext-fwd" suffix to avoid collision with the local-egress
// rules keyed by acl.ID, and a "-reverse" companion is added for Bi policies.
func appendUserExtClientRemoteEgressFwdRules(
	targetnode *models.Node,
	acls []models.Acl,
	egs []schema.Egress,
	userNodes []models.Node,
	userGrpMap map[schema.UserGroupID]map[string]struct{},
	rules map[string]models.AclRule,
) {
	remoteEgresses := make(map[string]schema.Egress)
	for _, egI := range egs {
		if !egI.Status {
			continue
		}
		if _, ok := egI.Nodes[targetnode.ID.String()]; ok {
			continue
		}
		remoteEgresses[egI.ID] = egI
	}
	if len(remoteEgresses) == 0 {
		return
	}

	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		dstTags := ConvAclTagToValueMap(acl.Dst)
		_, dstAll := dstTags["*"]
		selectedIP4, selectedIP6 := getSelectedEgressIPNets(acl.Dst)

		var dst4, dst6 []net.IPNet
		for egID, egI := range remoteEgresses {
			if _, ok := dstTags[egID]; !ok && !dstAll {
				continue
			}
			if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
				dst4 = append(dst4, selectedIP4...)
				dst6 = append(dst6, selectedIP6...)
				continue
			}
			egressRange := egI.Range
			if egI.VirtualRange != "" {
				egressRange = egI.VirtualRange
			}
			if egressRange != "" || IsEgressInternetGateway(egI) {
				AppendEgressPolicyRange(egI, egressRange, &dst4, &dst6)
				continue
			}
			if HasEgressDomainAns(egI) {
				for _, domainAnsI := range AllDomainAnsFromEgress(egI) {
					ip, cidr, parseErr := net.ParseCIDR(domainAnsI)
					if parseErr != nil {
						continue
					}
					if ip.To4() != nil {
						dst4 = append(dst4, *cidr)
					} else {
						dst6 = append(dst6, *cidr)
					}
				}
			}
		}
		if len(dst4) == 0 && len(dst6) == 0 {
			continue
		}

		allowedOwners := make(map[string]struct{})
		for _, srcAcl := range acl.Src {
			if srcAcl.ID == models.UserAclID {
				allowedOwners[srcAcl.Value] = struct{}{}
			} else if srcAcl.ID == models.UserGroupAclID {
				if usersMap, ok := userGrpMap[schema.UserGroupID(srcAcl.Value)]; ok {
					for userName := range usersMap {
						allowedOwners[userName] = struct{}{}
					}
				}
			}
		}
		if len(allowedOwners) == 0 {
			continue
		}

		var srcIP4, srcIP6 []net.IPNet
		for _, userNode := range userNodes {
			if !userNode.StaticNode.Enabled {
				continue
			}
			if userNode.StaticNode.IngressGatewayID != targetnode.ID.String() {
				continue
			}
			_, ownerOK := allowedOwners[userNode.StaticNode.OwnerID]
			_, allUsers := allowedOwners["*"]
			if !ownerOK && !allUsers {
				continue
			}
			if userNode.StaticNode.Address != "" {
				srcIP4 = append(srcIP4, userNode.StaticNode.AddressIPNet4())
			}
			if userNode.StaticNode.Address6 != "" {
				srcIP6 = append(srcIP6, userNode.StaticNode.AddressIPNet6())
			}
		}
		if len(srcIP4) == 0 && len(srcIP6) == 0 {
			continue
		}

		ruleID := acl.ID + "#ext-fwd"
		aclRule := models.AclRule{
			ID:              ruleID,
			AllowedProtocol: acl.Proto,
			AllowedPorts:    acl.Port,
			Direction:       acl.AllowedDirection,
			Allowed:         true,
			IPList:          UniqueIPNetList(srcIP4),
			IP6List:         UniqueIPNetList(srcIP6),
			Dst:             UniqueIPNetList(dst4),
			Dst6:            UniqueIPNetList(dst6),
		}
		rules[ruleID] = aclRule

		if acl.AllowedDirection == models.TrafficDirectionBi &&
			(len(aclRule.Dst) > 0 || len(aclRule.Dst6) > 0) {
			revID := ruleID + "-reverse"
			rules[revID] = models.AclRule{
				ID:              revID,
				AllowedProtocol: acl.Proto,
				AllowedPorts:    acl.Port,
				Direction:       acl.AllowedDirection,
				Allowed:         true,
				IPList:          append([]net.IPNet(nil), aclRule.Dst...),
				IP6List:         append([]net.IPNet(nil), aclRule.Dst6...),
				Dst:             append([]net.IPNet(nil), aclRule.IPList...),
				Dst6:            append([]net.IPNet(nil), aclRule.IP6List...),
			}
		}
	}
}

func getUserAclRulesForNode(ctx context.Context, targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	userNodes := GetStaticUserNodesByNetwork(ctx, schema.NetworkID(targetnode.Network))
	userGrpMap := userGroupsForNetwork(ctx, schema.NetworkID(targetnode.Network))
	allowedUsers := make(map[string][]models.Acl)
	acls := ListNetworkAccessUserPolicies(ctx, schema.NetworkID(targetnode.Network))
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
	defaultPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(targetnode.Network), models.UserPolicy)
	targetNodeTags[models.TagID(targetnode.ID.String())] = struct{}{}
	if !defaultPolicy.Enabled {
		for _, acl := range acls {
			if !acl.Enabled {
				continue
			}
			dstTags := ConvAclTagToValueMap(acl.Dst)
			_, all := dstTags["*"]
			addUsers := false
			if !all {
				for _, dst := range acl.Dst {
					if dst.ID == models.EgressID {
						e := schema.Egress{ID: dst.Value}
						err := e.Get(ctx)
						if err == nil && e.Status && len(e.Nodes) > 0 {
							if _, ok := e.Nodes[targetnode.ID.String()]; ok {
								dstTags[targetnode.ID.String()] = struct{}{}
							}
						}
					}
				}
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
						if usersMap, ok := userGrpMap[schema.UserGroupID(srcAcl.Value)]; ok {
							for userName := range usersMap {
								allowedUsers[userName] = append(allowedUsers[userName], acl)
							}
						}
					}
				}
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

			// Get peers in the tags and add allowed rules
			if userNode.StaticNode.Address != "" {
				r.IPList = append(r.IPList, userNode.StaticNode.AddressIPNet4())
			}
			if userNode.StaticNode.Address6 != "" {
				r.IP6List = append(r.IP6List, userNode.StaticNode.AddressIPNet6())
			}
		}
		rules[defaultPolicy.ID] = r
	} else {
		for _, userNode := range userNodes {
			if !userNode.StaticNode.Enabled {
				continue
			}
			acls := aclsForUserOwner(allowedUsers, userNode.StaticNode.OwnerID)
			if len(acls) == 0 {
				continue
			}
			for _, acl := range acls {

				if !acl.Enabled {
					continue
				}
				egressRanges4 := []net.IPNet{}
				egressRanges6 := []net.IPNet{}
				selectedIP4, selectedIP6 := getSelectedEgressIPNets(acl.Dst)

				for _, dst := range acl.Dst {
					if dst.Value == "*" {
						e := schema.Egress{Network: targetnode.Network}
						eli, _ := e.ListByNetwork(ctx)
						for _, eI := range eli {
							if !eI.Status || len(eI.Nodes) == 0 {
								continue
							}
							nodeOwnsEgress := false
							if _, ok := eI.Nodes[targetnode.ID.String()]; ok {
								nodeOwnsEgress = true
							}
							if nodeOwnsEgress {
								if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
									egressRanges4 = append(egressRanges4, selectedIP4...)
									egressRanges6 = append(egressRanges6, selectedIP6...)
								} else if HasEgressDomainAns(eI) && eI.Range == "" && !IsEgressInternetGateway(eI) {
									for _, domainAnsI := range AllDomainAnsFromEgress(eI) {
										_, cidr, err := net.ParseCIDR(domainAnsI)
										if err == nil {
											if cidr.IP.To4() != nil {
												egressRanges4 = append(egressRanges4, *cidr)
											} else {
												egressRanges6 = append(egressRanges6, *cidr)
											}
										}
									}
								} else {
									AppendEgressPolicyRange(eI, eI.Range, &egressRanges4, &egressRanges6)
								}
							} else if eI.VirtualRange != "" {
								// Use virtual range if target node doesn't own the egress
								_, cidr, err := net.ParseCIDR(eI.VirtualRange)
								if err == nil {
									if cidr.IP.To4() != nil {
										egressRanges4 = append(egressRanges4, *cidr)
									} else {
										egressRanges6 = append(egressRanges6, *cidr)
									}
								}
							}
						}
						break
					}
					if dst.ID == models.EgressID {
						e := schema.Egress{ID: dst.Value}
						err := e.Get(ctx)
						if err == nil && e.Status && len(e.Nodes) > 0 {
							nodeOwnsEgress := false
							if _, ok := e.Nodes[targetnode.ID.String()]; ok {
								nodeOwnsEgress = true
							}
							if nodeOwnsEgress {
								if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
									egressRanges4 = append(egressRanges4, selectedIP4...)
									egressRanges6 = append(egressRanges6, selectedIP6...)
								} else if HasEgressDomainAns(e) && e.Range == "" && !IsEgressInternetGateway(e) {
									for _, domainAnsI := range AllDomainAnsFromEgress(e) {
										_, cidr, err := net.ParseCIDR(domainAnsI)
										if err == nil {
											if cidr.IP.To4() != nil {
												egressRanges4 = append(egressRanges4, *cidr)
											} else {
												egressRanges6 = append(egressRanges6, *cidr)
											}
										}
									}
								} else {
									AppendEgressPolicyRange(e, e.Range, &egressRanges4, &egressRanges6)
								}
							} else if e.VirtualRange != "" {
								// Use virtual range if target node doesn't own the egress
								_, cidr, err := net.ParseCIDR(e.VirtualRange)
								if err == nil {
									if cidr.IP.To4() != nil {
										egressRanges4 = append(egressRanges4, *cidr)
									} else {
										egressRanges6 = append(egressRanges6, *cidr)
									}
								}
							}

						}
					}

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
				if len(egressRanges4) > 0 {
					r.Dst = append(r.Dst, egressRanges4...)
				}
				if len(egressRanges6) > 0 {
					r.Dst6 = append(r.Dst6, egressRanges6...)
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

					aclRule.Dst = append(aclRule.Dst, r.Dst...)
					aclRule.Dst6 = append(aclRule.Dst6, r.Dst6...)

					aclRule.IPList = UniqueIPNetList(aclRule.IPList)
					aclRule.IP6List = UniqueIPNetList(aclRule.IP6List)

					aclRule.Dst = UniqueIPNetList(aclRule.Dst)
					aclRule.Dst6 = UniqueIPNetList(aclRule.Dst6)

					rules[acl.ID] = aclRule
				} else {
					r.IPList = UniqueIPNetList(r.IPList)
					r.IP6List = UniqueIPNetList(r.IP6List)

					r.Dst = UniqueIPNetList(r.Dst)
					r.Dst6 = UniqueIPNetList(r.Dst6)
					rules[acl.ID] = r
				}
			}
		}
	}
	return rules
}
