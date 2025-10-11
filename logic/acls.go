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

var GetFwRulesForNodeAndPeerOnGw = getFwRulesForNodeAndPeerOnGw

var GetTagMapWithNodesByNetwork = getTagMapWithNodesByNetwork

var GetEgressUserRulesForNode = func(targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	return rules
}
var GetUserAclRulesForNode = func(targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	return rules
}

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
	defer func() {
		if len(rules) == 0 && IsNodeAllowedToCommunicateWithAllRsrcs(node) {
			if node.NetworkRange.IP != nil {
				rules = append(rules, models.FwRule{
					SrcIP: node.NetworkRange,
					Allow: true,
				})
			}
			if node.NetworkRange6.IP != nil {
				rules = append(rules, models.FwRule{
					SrcIP: node.NetworkRange6,
					Allow: true,
				})
			}
			return
		}
	}()

	for _, nodeI := range nodes {
		if !nodeI.IsStatic || nodeI.IsUserNode {
			continue
		}
		if !nodeI.StaticNode.Enabled {
			continue
		}
		if IsNodeAllowedToCommunicateWithAllRsrcs(nodeI) {
			if nodeI.Address.IP != nil {
				rules = append(rules, models.FwRule{
					SrcIP: net.IPNet{
						IP:   nodeI.Address.IP,
						Mask: net.CIDRMask(32, 32),
					},
					Allow: true,
				})
				rules = append(rules, models.FwRule{
					SrcIP: node.NetworkRange,
					DstIP: net.IPNet{
						IP:   nodeI.Address.IP,
						Mask: net.CIDRMask(32, 32),
					},
					Allow: true,
				})
			}
			if nodeI.Address6.IP != nil {
				rules = append(rules, models.FwRule{
					SrcIP: net.IPNet{
						IP:   nodeI.Address6.IP,
						Mask: net.CIDRMask(128, 128),
					},
					Allow: true,
				})
				rules = append(rules, models.FwRule{
					SrcIP: node.NetworkRange6,
					DstIP: net.IPNet{
						IP:   nodeI.Address.IP,
						Mask: net.CIDRMask(128, 128),
					},
					Allow: true,
				})
			}
			continue
		}
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
	if len(node.RelayedNodes) > 0 {
		for _, relayedNodeID := range node.RelayedNodes {
			relayedNode, err := GetNodeByID(relayedNodeID)
			if err != nil {
				continue
			}

			if relayedNode.Address.IP != nil {
				rules = append(rules, models.FwRule{
					AllowedProtocol: models.ALL,
					AllowedPorts:    []string{},
					Allow:           true,
					DstIP:           relayedNode.AddressIPNet4(),
					SrcIP:           node.NetworkRange,
				})
				rules = append(rules, models.FwRule{
					AllowedProtocol: models.ALL,
					AllowedPorts:    []string{},
					Allow:           true,
					DstIP:           node.NetworkRange,
					SrcIP:           relayedNode.AddressIPNet4(),
				})
			}

			if relayedNode.Address6.IP != nil {
				rules = append(rules, models.FwRule{
					AllowedProtocol: models.ALL,
					AllowedPorts:    []string{},
					Allow:           true,
					DstIP:           relayedNode.AddressIPNet6(),
					SrcIP:           node.NetworkRange6,
				})
				rules = append(rules, models.FwRule{
					AllowedProtocol: models.ALL,
					AllowedPorts:    []string{},
					Allow:           true,
					DstIP:           node.NetworkRange6,
					SrcIP:           relayedNode.AddressIPNet6(),
				})
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
				if len(e.DomainAns) > 0 {
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
				} else {
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
		if !extclient.StaticNode.Enabled {
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

var CheckIfAnyPolicyisUniDirectional = func(targetNode models.Node, acls []models.Acl) bool {
	return false
}

func GetAclRulesForNode(targetnodeI *models.Node) (rules map[string]models.AclRule) {
	targetnode := *targetnodeI
	defer func() {
		//if !targetnode.IsIngressGateway {
		rules = GetUserAclRulesForNode(&targetnode, rules)
		//}
	}()
	rules = make(map[string]models.AclRule)
	if IsNodeAllowedToCommunicateWithAllRsrcs(targetnode) {
		aclRule := models.AclRule{
			ID:              fmt.Sprintf("%s-all-allowed-node-rule", targetnode.ID.String()),
			AllowedProtocol: models.ALL,
			Direction:       models.TrafficDirectionBi,
			Allowed:         true,
			IPList:          []net.IPNet{targetnode.NetworkRange},
			IP6List:         []net.IPNet{targetnode.NetworkRange6},
			Dst:             []net.IPNet{targetnode.AddressIPNet4()},
			Dst6:            []net.IPNet{targetnode.AddressIPNet6()},
		}
		e := schema.Egress{Network: targetnode.Network}
		egressRanges4 := []net.IPNet{}
		egressRanges6 := []net.IPNet{}
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
				}
			}
		}
		if len(egressRanges4) > 0 {
			aclRule.Dst = append(aclRule.Dst, egressRanges4...)
		}
		if len(egressRanges6) > 0 {
			aclRule.Dst6 = append(aclRule.Dst6, egressRanges6...)
		}
		rules[aclRule.ID] = aclRule
		return
	}
	var taggedNodes map[models.TagID][]models.Node
	if targetnode.IsIngressGateway {
		taggedNodes = GetTagMapWithNodesByNetwork(models.NetworkID(targetnode.Network), false)
	} else {
		taggedNodes = GetTagMapWithNodesByNetwork(models.NetworkID(targetnode.Network), true)
	}
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
						if servercfg.IsPro && eI.Domain != "" && len(eI.DomainAns) > 0 {
							for _, domainAnsI := range eI.DomainAns {
								ip, cidr, err := net.ParseCIDR(domainAnsI)
								if err == nil {
									if ip.To4() != nil {
										egressRanges4 = append(egressRanges4, *cidr)
									} else {
										egressRanges6 = append(egressRanges6, *cidr)
									}
								}
							}
						} else if eI.Range != "" {
							_, cidr, err := net.ParseCIDR(eI.Range)
							if err == nil {
								if cidr.IP.To4() != nil {
									egressRanges4 = append(egressRanges4, *cidr)
								} else {
									egressRanges6 = append(egressRanges6, *cidr)
								}
							}
						}
						dstTags[targetnode.ID.String()] = struct{}{}
					}
				}
				break
			}
			if dst.ID == models.EgressID {
				e := schema.Egress{ID: dst.Value}
				err := e.Get(db.WithContext(context.TODO()))
				if err == nil && e.Status && len(e.Nodes) > 0 {
					if _, ok := e.Nodes[targetnode.ID.String()]; ok {
						if servercfg.IsPro && e.Domain != "" && len(e.DomainAns) > 0 {
							for _, domainAnsI := range e.DomainAns {
								ip, cidr, err := net.ParseCIDR(domainAnsI)
								if err == nil {
									if ip.To4() != nil {
										egressRanges4 = append(egressRanges4, *cidr)
									} else {
										egressRanges6 = append(egressRanges6, *cidr)
									}
								}
							}
						} else if e.Range != "" {
							_, cidr, err := net.ParseCIDR(e.Range)
							if err == nil {
								if cidr.IP.To4() != nil {
									egressRanges4 = append(egressRanges4, *cidr)
								} else {
									egressRanges6 = append(egressRanges6, *cidr)
								}
							}
						}
						dstTags[targetnode.ID.String()] = struct{}{}
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
			Dst:             []net.IPNet{targetnode.AddressIPNet4()},
			Dst6:            []net.IPNet{targetnode.AddressIPNet6()},
		}
		if len(egressRanges4) > 0 {
			aclRule.Dst = append(aclRule.Dst, egressRanges4...)
		}
		if len(egressRanges6) > 0 {
			aclRule.Dst6 = append(aclRule.Dst6, egressRanges6...)
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
				if existsInDstTag /*&& !existsInSrcTag*/ {
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

		}

		if len(aclRule.IPList) > 0 || len(aclRule.IP6List) > 0 {
			aclRule.IPList = UniqueIPNetList(aclRule.IPList)
			aclRule.IP6List = UniqueIPNetList(aclRule.IP6List)
			rules[acl.ID] = aclRule
		}
	}
	return rules
}

func GetEgressRulesForNode(targetnode models.Node) (rules map[string]models.AclRule) {
	rules = make(map[string]models.AclRule)
	defer func() {
		rules = GetEgressUserRulesForNode(&targetnode, rules)
	}()
	taggedNodes := GetTagMapWithNodesByNetwork(models.NetworkID(targetnode.Network), true)

	acls := ListDevicePolicies(models.NetworkID(targetnode.Network))
	var targetNodeTags = make(map[models.TagID]struct{})
	targetNodeTags[models.TagID(targetnode.ID.String())] = struct{}{}
	targetNodeTags["*"] = struct{}{}
	if targetnode.IsGw && !servercfg.IsPro {
		targetNodeTags[models.TagID(fmt.Sprintf("%s.%s", targetnode.Network, models.GwTagName))] = struct{}{}
	}

	egs, _ := (&schema.Egress{Network: targetnode.Network}).ListByNetwork(db.WithContext(context.TODO()))
	if len(egs) == 0 {
		return
	}
	var egressIDMap = make(map[string]schema.Egress)
	for _, egI := range egs {
		if !egI.Status {
			continue
		}
		if _, ok := egI.Nodes[targetnode.ID.String()]; ok {
			egressIDMap[egI.ID] = egI
		}
	}
	if len(egressIDMap) == 0 {
		return
	}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcTags := ConvAclTagToValueMap(acl.Src)
		dstTags := ConvAclTagToValueMap(acl.Dst)
		_, dstAll := dstTags["*"]
		aclRule := models.AclRule{
			ID:              acl.ID,
			AllowedProtocol: acl.Proto,
			AllowedPorts:    acl.Port,
			Direction:       acl.AllowedDirection,
			Allowed:         true,
		}
		for egressID, egI := range egressIDMap {
			if _, ok := dstTags[egressID]; ok || dstAll {
				if servercfg.IsPro && egI.Domain != "" && len(egI.DomainAns) > 0 {
					for _, domainAnsI := range egI.DomainAns {
						ip, cidr, err := net.ParseCIDR(domainAnsI)
						if err == nil {
							if ip.To4() != nil {
								aclRule.Dst = append(aclRule.Dst, *cidr)
							} else {
								aclRule.Dst6 = append(aclRule.Dst6, *cidr)
							}
						}
					}
				} else {
					ip, cidr, err := net.ParseCIDR(egI.Range)
					if err == nil {
						if ip.To4() != nil {
							aclRule.Dst = append(aclRule.Dst, *cidr)
						} else {
							aclRule.Dst6 = append(aclRule.Dst6, *cidr)
						}
					}
				}

				_, srcAll := srcTags["*"]
				if srcAll {
					if targetnode.NetworkRange.IP != nil {
						aclRule.IPList = append(aclRule.IPList, targetnode.NetworkRange)
					}
					if targetnode.NetworkRange6.IP != nil {
						aclRule.IP6List = append(aclRule.IP6List, targetnode.NetworkRange6)
					}
					continue
				}
				// get all src tags
				for src := range srcTags {
					// Get peers in the tags and add allowed rules
					nodes := taggedNodes[models.TagID(src)]
					for _, node := range nodes {
						if node.ID == targetnode.ID {
							continue
						}
						if !node.IsStatic && node.Address.IP != nil {
							aclRule.IPList = append(aclRule.IPList, node.AddressIPNet4())
						}
						if !node.IsStatic && node.Address6.IP != nil {
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

	return
}

func GetAclRuleForInetGw(targetnode models.Node) (rules map[string]models.AclRule) {
	rules = make(map[string]models.AclRule)
	if targetnode.IsInternetGateway {
		aclRule := models.AclRule{
			ID:              fmt.Sprintf("%s-inet-gw-internal-rule", targetnode.ID.String()),
			AllowedProtocol: models.ALL,
			AllowedPorts:    []string{},
			Direction:       models.TrafficDirectionBi,
			Allowed:         true,
		}
		if targetnode.NetworkRange.IP != nil {
			aclRule.IPList = append(aclRule.IPList, targetnode.NetworkRange)
			_, allIpv4, _ := net.ParseCIDR(IPv4Network)
			aclRule.Dst = append(aclRule.Dst, *allIpv4)
		}
		if targetnode.NetworkRange6.IP != nil {
			aclRule.IP6List = append(aclRule.IP6List, targetnode.NetworkRange6)
			_, allIpv6, _ := net.ParseCIDR(IPv6Network)
			aclRule.Dst6 = append(aclRule.Dst6, *allIpv6)
		}
		rules[aclRule.ID] = aclRule
	}
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
		if !servercfg.IsPro {
			if acl.AllowedDirection == models.TrafficDirectionUni {
				acl.AllowedDirection = models.TrafficDirectionBi
				UpsertAcl(acl)
			}
		}
	}

}

func IsNodeAllowedToCommunicateWithAllRsrcs(node models.Node) bool {
	// check default policy if all allowed return true
	defaultPolicy, err := GetDefaultPolicy(models.NetworkID(node.Network), models.DevicePolicy)
	if err == nil {
		if defaultPolicy.Enabled {
			return true
		}
	}
	var nodeId string
	if node.IsStatic {
		nodeId = node.StaticNode.ClientID
		node = node.StaticNode.ConvertToStaticNode()
	} else {
		nodeId = node.ID.String()
	}
	var nodeTags map[models.TagID]struct{}
	if node.Mutex != nil {
		node.Mutex.Lock()
		nodeTags = maps.Clone(node.Tags)
		node.Mutex.Unlock()
	} else {
		nodeTags = maps.Clone(node.Tags)
	}
	if nodeTags == nil {
		nodeTags = make(map[models.TagID]struct{})
	}
	nodeTags[models.TagID(node.ID.String())] = struct{}{}
	nodeTags["*"] = struct{}{}
	nodeTags[models.TagID(nodeId)] = struct{}{}
	if !servercfg.IsPro && node.IsGw {
		node.Tags[models.TagID(fmt.Sprintf("%s.%s", node.Network, models.GwTagName))] = struct{}{}
	}
	// list device policies
	policies := ListDevicePolicies(models.NetworkID(node.Network))
	srcMap := make(map[string]struct{})
	dstMap := make(map[string]struct{})
	defer func() {
		srcMap = nil
		dstMap = nil
	}()
	if CheckIfAnyPolicyisUniDirectional(node, policies) {
		return false
	}
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		srcMap = ConvAclTagToValueMap(policy.Src)
		dstMap = ConvAclTagToValueMap(policy.Dst)
		_, srcAll := srcMap["*"]
		_, dstAll := dstMap["*"]

		for tagID := range nodeTags {
			if srcAll {
				if _, ok := dstMap[tagID.String()]; ok {
					return true
				}
			}
			if dstAll {
				if _, ok := srcMap[tagID.String()]; ok {
					return true
				}
			}
		}
	}
	return false
}

// IsNodeAllowedToCommunicate - check node is allowed to communicate with the peer // ADD ALLOWED DIRECTION - 0 => node -> peer, 1 => peer-> node,
func IsNodeAllowedToCommunicate(node, peer models.Node, checkDefaultPolicy bool) (bool, []models.Acl) {
	var nodeId, peerId string
	// if peer.IsFailOver && node.FailedOverBy != uuid.Nil && node.FailedOverBy == peer.ID {
	// 	return true, []models.Acl{}
	// }
	// if node.IsFailOver && peer.FailedOverBy != uuid.Nil && peer.FailedOverBy == node.ID {
	// 	return true, []models.Acl{}
	// }
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
	if !servercfg.IsPro && ruleType == models.UserPolicy {
		return models.Acl{Enabled: true}, nil
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

// ListUserPolicies - lists all user policies in a network
func ListUserPolicies(netID models.NetworkID) []models.Acl {
	allAcls := ListAcls()
	userAcls := []models.Acl{}
	for _, acl := range allAcls {
		if acl.NetworkID == netID && acl.RuleType == models.UserPolicy {
			userAcls = append(userAcls, acl)
		}
	}
	return userAcls
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
	for _, src := range req.Src {
		if src.ID == models.UserGroupAclID {
			userGroup, err := GetUserGroup(models.UserGroupID(src.Value))
			if err != nil {
				return err
			}

			_, ok := userGroup.NetworkRoles[models.AllNetworks]
			if ok {
				continue
			}

			_, ok = userGroup.NetworkRoles[req.NetworkID]
			if !ok {
				return fmt.Errorf("user group %s does not have access to network %s", src.Value, req.NetworkID)
			}
		}
	}
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

func getTagMapWithNodesByNetwork(netID models.NetworkID, withStaticNodes bool) (tagNodesMap map[models.TagID][]models.Node) {
	tagNodesMap = make(map[models.TagID][]models.Node)
	nodes, _ := GetNetworkNodes(netID.String())
	netGwTag := models.TagID(fmt.Sprintf("%s.%s", netID.String(), models.GwTagName))
	for _, nodeI := range nodes {
		tagNodesMap[models.TagID(nodeI.ID.String())] = append(tagNodesMap[models.TagID(nodeI.ID.String())], nodeI)
		if nodeI.IsGw {
			tagNodesMap[netGwTag] = append(tagNodesMap[netGwTag], nodeI)
		}
	}
	tagNodesMap["*"] = nodes
	if !withStaticNodes {
		return
	}
	return addTagMapWithStaticNodes(netID, tagNodesMap)
}

func addTagMapWithStaticNodes(netID models.NetworkID,
	tagNodesMap map[models.TagID][]models.Node) map[models.TagID][]models.Node {
	extclients, err := GetNetworkExtClients(netID.String())
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
		tagNodesMap["*"] = append(tagNodesMap["*"], extclient.ConvertToStaticNode())

	}
	return tagNodesMap
}
