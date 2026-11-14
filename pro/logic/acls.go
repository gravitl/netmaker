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

func GetFwRulesForNodeAndPeerOnGw(node, peer models.Node, allowedPolicies []models.Acl) (rules []models.FwRule) {

	for _, policy := range allowedPolicies {
		selectedIP4, selectedIP6 := getSelectedUserEgressIPNets(policy.Dst)
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
				nodeOwnsEgress := false
				if _, ok := e.Nodes[node.ID.String()]; ok {
					nodeOwnsEgress = true
				}
				// Use virtual range if node doesn't own the egress, otherwise use regular range
				egressRange := e.Range
				if !nodeOwnsEgress && e.VirtualRange != "" {
					egressRange = e.VirtualRange
				}
				if egressRange != "" || logic.IsEgressInternetGateway(e) {
					if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
						for _, cidr := range selectedIP4 {
							if node.Address.IP != nil {
								rules = append(rules, models.FwRule{
									SrcIP: net.IPNet{
										IP:   node.Address.IP,
										Mask: net.CIDRMask(32, 32),
									},
									DstIP:           cidr,
									AllowedProtocol: policy.Proto,
									AllowedPorts:    policy.Port,
									Allow:           true,
								})
							}
						}
						for _, cidr := range selectedIP6 {
							if node.Address6.IP != nil {
								rules = append(rules, models.FwRule{
									SrcIP: net.IPNet{
										IP:   node.Address6.IP,
										Mask: net.CIDRMask(128, 128),
									},
									DstIP:           cidr,
									AllowedProtocol: policy.Proto,
									AllowedPorts:    policy.Port,
									Allow:           true,
								})
							}
						}
						continue
					}
					var dst4, dst6 []net.IPNet
					logic.AppendEgressPolicyRange(e, egressRange, &dst4, &dst6)
					for _, cidr := range dst4 {
						if node.Address.IP != nil {
							rules = append(rules, models.FwRule{
								SrcIP: net.IPNet{
									IP:   node.Address.IP,
									Mask: net.CIDRMask(32, 32),
								},
								DstIP:           cidr,
								AllowedProtocol: policy.Proto,
								AllowedPorts:    policy.Port,
								Allow:           true,
							})
						}
					}
					for _, cidr := range dst6 {
						if node.Address6.IP != nil {
							rules = append(rules, models.FwRule{
								SrcIP: net.IPNet{
									IP:   node.Address6.IP,
									Mask: net.CIDRMask(128, 128),
								},
								DstIP:           cidr,
								AllowedProtocol: policy.Proto,
								AllowedPorts:    policy.Port,
								Allow:           true,
							})
						}
					}
				} else if logic.HasEgressDomainAns(e) {
					if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
						for _, cidr := range selectedIP4 {
							if node.Address.IP != nil {
								rules = append(rules, models.FwRule{
									SrcIP: net.IPNet{
										IP:   node.Address.IP,
										Mask: net.CIDRMask(32, 32),
									},
									DstIP:           cidr,
									AllowedProtocol: policy.Proto,
									AllowedPorts:    policy.Port,
									Allow:           true,
								})
							}
						}
						for _, cidr := range selectedIP6 {
							if node.Address6.IP != nil {
								rules = append(rules, models.FwRule{
									SrcIP: net.IPNet{
										IP:   node.Address6.IP,
										Mask: net.CIDRMask(128, 128),
									},
									DstIP:           cidr,
									AllowedProtocol: policy.Proto,
									AllowedPorts:    policy.Port,
									Allow:           true,
								})
							}
						}
						continue
					}
					for _, domainAnsI := range logic.AllDomainAnsFromEgress(e) {
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

func checkIfAclTagisValid(ctx context.Context, a models.Acl, t models.AclPolicyTag, isSrc bool) (err error) {
	switch t.ID {
	case models.NodeTagID:
		if a.RuleType == models.UserPolicy && isSrc {
			return errors.New("user policy source mismatch")
		}
		// check if tag is valid
		_, err := GetTag(ctx, models.TagID(t.Value))
		if err != nil {
			return errors.New("invalid tag " + t.Value)
		}
	case models.NodeID:
		if a.RuleType == models.UserPolicy && isSrc {
			return errors.New("user policy source mismatch")
		}
		_, nodeErr := logic.GetNodeByID(t.Value)
		if nodeErr != nil {
			_, staticNodeErr := logic.GetExtClient(ctx, t.Value, a.NetworkID.String())
			if staticNodeErr != nil {
				return errors.New("invalid node " + t.Value)
			}
		}
	case models.EgressID, models.EgressRange:
		e := schema.Egress{
			ID: t.Value,
		}
		err := e.Get(ctx)
		if err != nil {
			return errors.New("invalid egress")
		}
	case models.NetmakerIPAclID:
		_, err := logic.NormalizeIPOrCIDR(t.Value)
		if err != nil {
			return err
		}

	case models.UserAclID:
		if a.RuleType == models.DevicePolicy {
			return errors.New("device policy source mismatch")
		}
		if !isSrc {
			return errors.New("user cannot be added to destination")
		}
		userCheck := &schema.User{Username: t.Value}
		err = userCheck.GetWithMembership(ctx)
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
		err := IsGroupValid(ctx, schema.UserGroupID(t.Value))
		if err != nil {
			return errors.New("invalid user group " + t.Value)
		}
		// check if group belongs to this network
		netGrps := GetUserGroupsInNetwork(ctx, a.NetworkID)
		if _, ok := netGrps[schema.UserGroupID(t.Value)]; !ok {
			return errors.New("invalid user group " + t.Value)
		}
	default:
		return errors.New("invalid policy")
	}
	return nil
}

// IsAclPolicyValid - validates if acl policy is valid
func IsAclPolicyValid(ctx context.Context, acl models.Acl) (err error) {
	if acl.IsManagedAccess() {
		if err := logic.ValidateManagedAccessAcl(acl); err != nil {
			return err
		}
	} else if err := logic.ValidateNetworkAccessAcl(acl); err != nil {
		return err
	}
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
			if err = checkIfAclTagisValid(ctx, acl, srcI, true); err != nil {
				return
			}
		}
		for _, dstI := range acl.Dst {

			if dstI.Value == "*" {
				continue
			}

			// check if user group is valid
			if err = checkIfAclTagisValid(ctx, acl, dstI, false); err != nil {
				return
			}
		}
	case models.DevicePolicy:
		for _, srcI := range acl.Src {
			if srcI.Value == "*" {
				continue
			}
			// check if user group is valid
			if err = checkIfAclTagisValid(ctx, acl, srcI, true); err != nil {
				return err
			}
		}
		for _, dstI := range acl.Dst {

			if dstI.Value == "*" {
				continue
			}
			// check if user group is valid
			if err = checkIfAclTagisValid(ctx, acl, dstI, false); err != nil {
				return
			}
		}
	}
	if err := logic.NormalizeAndValidateAclEgressIPs(&acl); err != nil {
		return err
	}
	return nil
}

func getSelectedUserEgressIPNets(dstTags []models.AclPolicyTag) (dst4, dst6 []net.IPNet) {
	for _, dst := range dstTags {
		if dst.ID != models.NetmakerIPAclID {
			continue
		}
		normalized, err := logic.NormalizeIPOrCIDR(dst.Value)
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

// IsPeerAllowed - checks if peer needs to be added to the interface
func IsPeerAllowed(ctx context.Context, node, peer models.Node, checkDefaultPolicy bool) bool {
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
		node = models.ConvertToStaticNode(node.StaticNode)
	} else {
		nodeId = node.ID.String()
	}
	if peer.IsStatic {
		peerId = peer.StaticNode.ClientID
		peer = models.ConvertToStaticNode(peer.StaticNode)
	} else {
		peerId = peer.ID.String()
	}

	var nodeTags, peerTags map[models.TagID]struct{}
	if node.Mutex != nil {
		node.Mutex.Lock()
		nodeTags = maps.Clone(node.Tags)
		node.Mutex.Unlock()
	} else {
		nodeTags = maps.Clone(node.Tags)
	}
	if peer.Mutex != nil {
		peer.Mutex.Lock()
		peerTags = maps.Clone(peer.Tags)
		peer.Mutex.Unlock()
	} else {
		peerTags = maps.Clone(peer.Tags)
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
		defaultPolicy, err := logic.GetDefaultPolicy(ctx, schema.NetworkID(node.Network), models.DevicePolicy)
		if err == nil {
			if defaultPolicy.Enabled {
				return true
			}
		}

	}
	// list device policies
	policies := logic.ListNetworkAccessDevicePolicies(ctx, schema.NetworkID(peer.Network))
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
		if logic.IsEgressRoutingPolicyAllowedForNodes(policy, node, peer) {
			return true
		}

		srcMap = logic.ConvAclTagToValueMap(policy.Src)
		dstMap = logic.ConvAclTagToValueMap(policy.Dst)
		for _, dst := range policy.Dst {
			if dst.ID == models.EgressID {
				e := schema.Egress{ID: dst.Value}
				err := e.Get(ctx)
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

// UpdateDeviceTag - updates device tag on acl policies
func UpdateDeviceTag(ctx context.Context, OldID, newID models.TagID, netID schema.NetworkID) {
	acls := logic.ListNetworkAccessDevicePolicies(ctx, netID)
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
			logic.UpsertAcl(ctx, acl)
		}
	}
}

func CheckIfTagAsActivePolicy(ctx context.Context, tagID models.TagID, netID schema.NetworkID) bool {
	acls := logic.ListNetworkAccessDevicePolicies(ctx, netID)
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
func RemoveDeviceTagFromAclPolicies(ctx context.Context, tagID models.TagID, netID schema.NetworkID) error {
	acls := logic.ListNetworkAccessDevicePolicies(ctx, netID)
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
			logic.UpsertAcl(ctx, acl)
		}
	}
	return nil
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

func GetTagMapWithNodesByNetwork(ctx context.Context, netID schema.NetworkID, withStaticNodes bool) map[models.TagID][]models.Node {
	nodes, _ := logic.GetNetworkNodes(ctx, netID.String())
	return GetTagMapWithNetworkNodes(ctx, netID, nodes, withStaticNodes)
}

// GetTagMapWithNetworkNodes builds the tag map from the network's nodes the caller
// already loaded.
func GetTagMapWithNetworkNodes(ctx context.Context, netID schema.NetworkID, nodes []models.Node, withStaticNodes bool) (tagNodesMap map[models.TagID][]models.Node) {
	tagNodesMap = make(map[models.TagID][]models.Node)
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
	tagNodesMap["*"] = logic.NodesForAllResourcesTag(nodes)
	if !withStaticNodes {
		return
	}
	return AddTagMapWithStaticNodes(ctx, netID, tagNodesMap)
}

func AddTagMapWithStaticNodes(ctx context.Context, netID schema.NetworkID,
	tagNodesMap map[models.TagID][]models.Node) map[models.TagID][]models.Node {
	extclients, err := logic.GetNetworkExtClients(ctx, netID.String())
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
			tagNodesMap[tagID] = append(tagNodesMap[tagID], models.ConvertToStaticNode(extclient))
			tagNodesMap["*"] = append(tagNodesMap["*"], models.ConvertToStaticNode(extclient))
		}
		if extclient.Mutex != nil {
			extclient.Mutex.Unlock()
		}
	}
	return tagNodesMap
}
