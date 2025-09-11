package logic

import (
	"context"
	"errors"
	"maps"
	"net"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func getStaticUserNodesByNetwork(network models.NetworkID) (staticNode []models.Node) {
	extClients, err := logic.GetAllExtClients()
	if err != nil {
		return
	}
	for _, extI := range extClients {
		if extI.Network == network.String() {
			if extI.RemoteAccessClientID != "" {
				n := extI.ConvertToStaticNode()
				staticNode = append(staticNode, n)
			}
		}
	}
	return
}

func GetFwRulesForUserNodesOnGw(node models.Node, nodes []models.Node) (rules []models.FwRule) {
	defaultUserPolicy, _ := logic.GetDefaultPolicy(models.NetworkID(node.Network), models.UserPolicy)
	userNodes := getStaticUserNodesByNetwork(models.NetworkID(node.Network))
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
			if peer.IsUserNode {
				continue
			}

			if ok, allowedPolicies := IsUserAllowedToCommunicate(userNodeI.StaticNode.OwnerID, peer); ok {
				if peer.IsStatic {
					peer = peer.StaticNode.ConvertToStaticNode()
				}
				for _, policy := range allowedPolicies {
					if userNodeI.StaticNode.Address != "" {
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
					if userNodeI.StaticNode.Address6 != "" {
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

					// add egress ranges
					for _, dstI := range policy.Dst {
						if dstI.Value == "*" {
							rules = append(rules, models.FwRule{
								SrcIP:           userNodeI.StaticNode.AddressIPNet4(),
								DstIP:           net.IPNet{},
								AllowedProtocol: policy.Proto,
								AllowedPorts:    policy.Port,
								Allow:           true,
							})
							break
						}
						if dstI.ID == models.EgressID {

							e := schema.Egress{ID: dstI.Value}
							err := e.Get(db.WithContext(context.TODO()))
							if err != nil {
								continue
							}
							if e.Range != "" {
								dstI.Value = e.Range

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
							} else if len(e.DomainAns) > 0 {
								for _, domainAns := range e.DomainAns {
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

func GetFwRulesForNodeAndPeerOnGw(node, peer models.Node, allowedPolicies []models.Acl) (rules []models.FwRule) {

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
				AllowedProtocol: policy.Proto,
				AllowedPorts:    policy.Port,
				Allow:           true,
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
				AllowedProtocol: policy.Proto,
				AllowedPorts:    policy.Port,
				Allow:           true,
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
					AllowedProtocol: policy.Proto,
					AllowedPorts:    policy.Port,
					Allow:           true,
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
					AllowedProtocol: policy.Proto,
					AllowedPorts:    policy.Port,
					Allow:           true,
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
				if e.Range != "" {
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
									DstIP:           *cidr,
									AllowedProtocol: policy.Proto,
									AllowedPorts:    policy.Port,
									Allow:           true,
								})
							}
						} else {
							if node.Address6.IP != nil {
								rules = append(rules, models.FwRule{
									SrcIP: net.IPNet{
										IP:   node.Address6.IP,
										Mask: net.CIDRMask(128, 128),
									},
									DstIP:           *cidr,
									AllowedProtocol: policy.Proto,
									AllowedPorts:    policy.Port,
									Allow:           true,
								})
							}
						}

					}
				} else if len(e.DomainAns) > 0 {
					for _, domainAnsI := range e.DomainAns {
						dstI.Value = domainAnsI

						ip, cidr, err := net.ParseCIDR(dstI.Value)
						if err == nil {
							if ip.To4() != nil {
								if node.Address.IP != nil {
									rules = append(rules, models.FwRule{
										SrcIP: net.IPNet{
											IP:   node.Address.IP,
											Mask: net.CIDRMask(32, 32),
										},
										DstIP:           *cidr,
										AllowedProtocol: policy.Proto,
										AllowedPorts:    policy.Port,
										Allow:           true,
									})
								}
							} else {
								if node.Address6.IP != nil {
									rules = append(rules, models.FwRule{
										SrcIP: net.IPNet{
											IP:   node.Address6.IP,
											Mask: net.CIDRMask(128, 128),
										},
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

	return
}

func checkIfAclTagisValid(a models.Acl, t models.AclPolicyTag, isSrc bool) (err error) {
	switch t.ID {
	case models.NodeTagID:
		if a.RuleType == models.UserPolicy && isSrc {
			return errors.New("user policy source mismatch")
		}
		// check if tag is valid
		_, err := GetTag(models.TagID(t.Value))
		if err != nil {
			return errors.New("invalid tag " + t.Value)
		}
	case models.NodeID:
		if a.RuleType == models.UserPolicy && isSrc {
			return errors.New("user policy source mismatch")
		}
		_, nodeErr := logic.GetNodeByID(t.Value)
		if nodeErr != nil {
			_, staticNodeErr := logic.GetExtClient(t.Value, a.NetworkID.String())
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

	case models.UserAclID:
		if a.RuleType == models.DevicePolicy {
			return errors.New("device policy source mismatch")
		}
		if !isSrc {
			return errors.New("user cannot be added to destination")
		}
		_, err := logic.GetUser(t.Value)
		if err != nil {
			return errors.New("invalid user " + t.Value)
		}
	case models.UserGroupAclID:
		if a.RuleType == models.DevicePolicy {
			return errors.New("device policy source mismatch")
		}
		if !isSrc {
			return errors.New("user cannot be added to destination")
		}
		err := IsGroupValid(models.UserGroupID(t.Value))
		if err != nil {
			return errors.New("invalid user group " + t.Value)
		}
		// check if group belongs to this network
		netGrps := GetUserGroupsInNetwork(a.NetworkID)
		if _, ok := netGrps[models.UserGroupID(t.Value)]; !ok {
			return errors.New("invalid user group " + t.Value)
		}
	default:
		return errors.New("invalid policy")
	}
	return nil
}

// IsAclPolicyValid - validates if acl policy is valid
func IsAclPolicyValid(acl models.Acl) (err error) {
	//check if src and dst are valid
	if acl.AllowedDirection != models.TrafficDirectionBi &&
		acl.AllowedDirection != models.TrafficDirectionUni {
		return errors.New("invalid traffic direction")
	}
	switch acl.RuleType {
	case models.UserPolicy:
		// src list should only contain users
		for _, srcI := range acl.Src {

			if srcI.Value == "*" {
				continue
			}
			// check if user group is valid
			if err = checkIfAclTagisValid(acl, srcI, true); err != nil {
				return
			}
		}
		for _, dstI := range acl.Dst {

			if dstI.Value == "*" {
				continue
			}

			// check if user group is valid
			if err = checkIfAclTagisValid(acl, dstI, false); err != nil {
				return
			}
		}
	case models.DevicePolicy:
		for _, srcI := range acl.Src {
			if srcI.Value == "*" {
				continue
			}
			// check if user group is valid
			if err = checkIfAclTagisValid(acl, srcI, true); err != nil {
				return err
			}
		}
		for _, dstI := range acl.Dst {

			if dstI.Value == "*" {
				continue
			}
			// check if user group is valid
			if err = checkIfAclTagisValid(acl, dstI, false); err != nil {
				return
			}
		}
	}
	return nil
}

// ListUserPolicies - lists all acl policies enforced on an user
func ListUserPolicies(u models.User) []models.Acl {
	allAcls := logic.ListAcls()
	userAcls := []models.Acl{}
	for _, acl := range allAcls {

		if acl.RuleType == models.UserPolicy {
			srcMap := logic.ConvAclTagToValueMap(acl.Src)
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
	allAcls := logic.ListAcls()
	userAcls := []models.Acl{}
	if _, ok := user.UserGroups[globalNetworksAdminGroupID]; ok {
		user.UserGroups[GetDefaultNetworkAdminGroupID(netID)] = struct{}{}
	}
	if _, ok := user.UserGroups[globalNetworksUserGroupID]; ok {
		user.UserGroups[GetDefaultNetworkUserGroupID(netID)] = struct{}{}
	}
	if user.PlatformRoleID == models.AdminRole || user.PlatformRoleID == models.SuperAdminRole {
		user.UserGroups[GetDefaultNetworkAdminGroupID(netID)] = struct{}{}
	}
	for _, acl := range allAcls {
		if acl.NetworkID == netID && acl.RuleType == models.UserPolicy {
			srcMap := logic.ConvAclTagToValueMap(acl.Src)
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

// listUserPolicies - lists all user policies in a network
func listUserPolicies(netID models.NetworkID) []models.Acl {
	allAcls := logic.ListAcls()
	deviceAcls := []models.Acl{}
	for _, acl := range allAcls {
		if acl.NetworkID == netID && acl.RuleType == models.UserPolicy {
			deviceAcls = append(deviceAcls, acl)
		}
	}
	return deviceAcls
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
	if peerTags == nil {
		peerTags = make(map[models.TagID]struct{})
	}
	peerTags[models.TagID(peerId)] = struct{}{}
	peerTags[models.TagID("*")] = struct{}{}
	acl, _ := logic.GetDefaultPolicy(models.NetworkID(peer.Network), models.UserPolicy)
	if acl.Enabled {
		return true, []models.Acl{acl}
	}
	user, err := logic.GetUser(userName)
	if err != nil {
		return false, []models.Acl{}
	}
	allowedPolicies := []models.Acl{}
	policies := listPoliciesOfUser(*user, models.NetworkID(peer.Network))
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		dstMap := logic.ConvAclTagToValueMap(policy.Dst)
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
	// if peer.IsFailOver && node.FailedOverBy != uuid.Nil && node.FailedOverBy == peer.ID {
	// 	return true
	// }
	// if node.IsFailOver && peer.FailedOverBy != uuid.Nil && peer.FailedOverBy == node.ID {
	// 	return true
	// }
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
		defaultPolicy, err := logic.GetDefaultPolicy(models.NetworkID(node.Network), models.DevicePolicy)
		if err == nil {
			if defaultPolicy.Enabled {
				return true
			}
		}

	}
	// list device policies
	policies := logic.ListDevicePolicies(models.NetworkID(peer.Network))
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

		srcMap = logic.ConvAclTagToValueMap(policy.Src)
		dstMap = logic.ConvAclTagToValueMap(policy.Dst)
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
		if logic.CheckTagGroupPolicy(srcMap, dstMap, node, peer, nodeTags, peerTags) {
			return true
		}

	}
	return false
}

func RemoveUserFromAclPolicy(userName string) {
	acls := logic.ListAcls()
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
				logic.DeleteAcl(acl)
				continue
			}
			if update {
				logic.UpsertAcl(acl)
			}
		}
	}
}

// UpdateDeviceTag - updates device tag on acl policies
func UpdateDeviceTag(OldID, newID models.TagID, netID models.NetworkID) {
	acls := logic.ListDevicePolicies(netID)
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
			logic.UpsertAcl(acl)
		}
	}
}

func CheckIfTagAsActivePolicy(tagID models.TagID, netID models.NetworkID) bool {
	acls := logic.ListDevicePolicies(netID)
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
	acls := logic.ListDevicePolicies(netID)
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
			logic.UpsertAcl(acl)
		}
	}
	return nil
}

func GetEgressUserRulesForNode(targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	userNodes := getStaticUserNodesByNetwork(models.NetworkID(targetnode.Network))
	userGrpMap := GetUserGrpMap()
	allowedUsers := make(map[string][]models.Acl)
	acls := listUserPolicies(models.NetworkID(targetnode.Network))
	var targetNodeTags = make(map[models.TagID]struct{})
	targetNodeTags["*"] = struct{}{}
	egs, _ := (&schema.Egress{Network: targetnode.Network}).ListByNetwork(db.WithContext(context.TODO()))
	if len(egs) == 0 {
		return rules
	}
	defaultPolicy, _ := logic.GetDefaultPolicy(models.NetworkID(targetnode.Network), models.UserPolicy)

	for _, egI := range egs {
		if !egI.Status {
			continue
		}
		if _, ok := egI.Nodes[targetnode.ID.String()]; ok {
			if egI.Range != "" {
				targetNodeTags[models.TagID(egI.Range)] = struct{}{}
			} else if len(egI.DomainAns) > 0 {
				for _, domainAnsI := range egI.DomainAns {
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
			dstTags := logic.ConvAclTagToValueMap(acl.Dst)
			for _, dst := range acl.Dst {
				if dst.ID == models.EgressID {
					e := schema.Egress{ID: dst.Value}
					err := e.Get(db.WithContext(context.TODO()))
					if err == nil && e.Status {
						for nodeID := range e.Nodes {
							dstTags[nodeID] = struct{}{}
						}
						if e.Range != "" {
							dstTags[e.Range] = struct{}{}
						} else if len(e.DomainAns) > 0 {
							for _, domainAnsI := range e.DomainAns {
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
						if usersMap, ok := userGrpMap[models.UserGroupID(srcAcl.Value)]; ok {
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
						if e.Range != "" {
							ip, cidr, err := net.ParseCIDR(e.Range)
							if err == nil {
								if ip.To4() != nil {
									r.Dst = append(r.Dst, *cidr)
								} else {
									r.Dst6 = append(r.Dst6, *cidr)
								}

							}
						} else if len(e.DomainAns) > 0 {
							for _, domainAnsI := range e.DomainAns {
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

					aclRule.IPList = logic.UniqueIPNetList(aclRule.IPList)
					aclRule.IP6List = logic.UniqueIPNetList(aclRule.IP6List)

					aclRule.Dst = logic.UniqueIPNetList(aclRule.Dst)
					aclRule.Dst6 = logic.UniqueIPNetList(aclRule.Dst6)

					rules[acl.ID] = aclRule
				} else {
					r.IPList = logic.UniqueIPNetList(r.IPList)
					r.IP6List = logic.UniqueIPNetList(r.IP6List)

					r.Dst = logic.UniqueIPNetList(r.Dst)
					r.Dst6 = logic.UniqueIPNetList(r.Dst6)
					rules[acl.ID] = r
				}
			}

		}
	}

	return rules
}

func GetUserAclRulesForNode(targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	userNodes := getStaticUserNodesByNetwork(models.NetworkID(targetnode.Network))
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
	defaultPolicy, _ := logic.GetDefaultPolicy(models.NetworkID(targetnode.Network), models.UserPolicy)
	targetNodeTags[models.TagID(targetnode.ID.String())] = struct{}{}
	if !defaultPolicy.Enabled {
		for _, acl := range acls {
			if !acl.Enabled {
				continue
			}
			dstTags := logic.ConvAclTagToValueMap(acl.Dst)
			_, all := dstTags["*"]
			addUsers := false
			if !all {
				for _, dst := range acl.Dst {
					if dst.ID == models.EgressID {
						e := schema.Egress{ID: dst.Value}
						err := e.Get(db.WithContext(context.TODO()))
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
						if usersMap, ok := userGrpMap[models.UserGroupID(srcAcl.Value)]; ok {
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
			acls, ok := allowedUsers[userNode.StaticNode.OwnerID]
			if !ok {
				continue
			}
			for _, acl := range acls {

				if !acl.Enabled {
					continue
				}
				egressRanges4 := []net.IPNet{}
				egressRanges6 := []net.IPNet{}

				for _, dst := range acl.Dst {
					if dst.Value == "*" {
						e := schema.Egress{Network: targetnode.Network}
						eli, _ := e.ListByNetwork(db.WithContext(context.Background()))
						for _, eI := range eli {
							if !eI.Status || len(eI.Nodes) == 0 {
								continue
							}
							if _, ok := eI.Nodes[targetnode.ID.String()]; ok {
								if eI.Range != "" {
									_, cidr, err := net.ParseCIDR(eI.Range)
									if err == nil {
										if cidr.IP.To4() != nil {
											egressRanges4 = append(egressRanges4, *cidr)
										} else {
											egressRanges6 = append(egressRanges6, *cidr)
										}
									}
								} else if len(eI.DomainAns) > 0 {
									for _, domainAnsI := range eI.DomainAns {
										_, cidr, err := net.ParseCIDR(domainAnsI)
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
						break
					}
					if dst.ID == models.EgressID {
						e := schema.Egress{ID: dst.Value}
						err := e.Get(db.WithContext(context.TODO()))
						if err == nil && e.Status && len(e.Nodes) > 0 {
							if _, ok := e.Nodes[targetnode.ID.String()]; ok {
								if e.Range != "" {
									_, cidr, err := net.ParseCIDR(e.Range)
									if err == nil {
										if cidr.IP.To4() != nil {
											egressRanges4 = append(egressRanges4, *cidr)
										} else {
											egressRanges6 = append(egressRanges6, *cidr)
										}
									}
								} else if len(e.DomainAns) > 0 {
									for _, domainAnsI := range e.DomainAns {
										_, cidr, err := net.ParseCIDR(domainAnsI)
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

					aclRule.IPList = logic.UniqueIPNetList(aclRule.IPList)
					aclRule.IP6List = logic.UniqueIPNetList(aclRule.IP6List)

					aclRule.Dst = logic.UniqueIPNetList(aclRule.Dst)
					aclRule.Dst6 = logic.UniqueIPNetList(aclRule.Dst6)

					rules[acl.ID] = aclRule
				} else {
					r.IPList = logic.UniqueIPNetList(r.IPList)
					r.IP6List = logic.UniqueIPNetList(r.IP6List)

					r.Dst = logic.UniqueIPNetList(r.Dst)
					r.Dst6 = logic.UniqueIPNetList(r.Dst6)
					rules[acl.ID] = r
				}
			}
		}
	}
	return rules
}

func CheckIfAnyPolicyisUniDirectional(targetNode models.Node, acls []models.Acl) bool {
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
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		if acl.AllowedDirection == models.TrafficDirectionBi && acl.Proto == models.ALL && acl.ServiceType == models.Any {
			continue
		}
		if acl.Proto != models.ALL || acl.ServiceType != models.Any {
			return true
		}
		srcTags := logic.ConvAclTagToValueMap(acl.Src)
		dstTags := logic.ConvAclTagToValueMap(acl.Dst)
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

func GetTagMapWithNodesByNetwork(netID models.NetworkID, withStaticNodes bool) (tagNodesMap map[models.TagID][]models.Node) {
	tagNodesMap = make(map[models.TagID][]models.Node)
	nodes, _ := logic.GetNetworkNodes(netID.String())
	for _, nodeI := range nodes {
		tagNodesMap[models.TagID(nodeI.ID.String())] = []models.Node{
			nodeI,
		}
		if nodeI.Tags == nil {
			continue
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Lock()
		}
		for nodeTagID := range nodeI.Tags {
			if nodeTagID == models.TagID(nodeI.ID.String()) {
				continue
			}
			tagNodesMap[nodeTagID] = append(tagNodesMap[nodeTagID], nodeI)
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Unlock()
		}
	}
	tagNodesMap["*"] = nodes
	if !withStaticNodes {
		return
	}
	return AddTagMapWithStaticNodes(netID, tagNodesMap)
}

func AddTagMapWithStaticNodes(netID models.NetworkID,
	tagNodesMap map[models.TagID][]models.Node) map[models.TagID][]models.Node {
	extclients, err := logic.GetNetworkExtClients(netID.String())
	if err != nil {
		return tagNodesMap
	}
	for _, extclient := range extclients {
		if extclient.RemoteAccessClientID != "" {
			continue
		}
		tagNodesMap[models.TagID(extclient.ClientID)] = []models.Node{
			{
				IsStatic:   true,
				StaticNode: extclient,
			},
		}
		if extclient.Tags == nil {
			continue
		}

		if extclient.Mutex != nil {
			extclient.Mutex.Lock()
		}
		for tagID := range extclient.Tags {
			if tagID == models.TagID(extclient.ClientID) {
				continue
			}
			tagNodesMap[tagID] = append(tagNodesMap[tagID], extclient.ConvertToStaticNode())
			tagNodesMap["*"] = append(tagNodesMap["*"], extclient.ConvertToStaticNode())
		}
		if extclient.Mutex != nil {
			extclient.Mutex.Unlock()
		}
	}
	return tagNodesMap
}
