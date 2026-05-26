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
var getEgressByID = func(egressID string) (schema.Egress, error) {
	e := schema.Egress{ID: egressID}
	err := e.Get(db.WithContext(context.TODO()))
	return e, err
}
var getEgressByNetwork = func(network string) ([]schema.Egress, error) {
	e := schema.Egress{Network: network}
	return e.ListByNetwork(db.WithContext(context.Background()))
}
var getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
	return ListDevicePolicies(netID)
}

// listNetworkExtClients fetches all extclients in a network; tests may override.
var listNetworkExtClients = func(network string) ([]models.ExtClient, error) {
	return GetNetworkExtClients(network)
}

// getNodeByID fetches a node by its UUID; tests may override.
var getNodeByID = func(uuid string) (models.Node, error) {
	return GetNodeByID(uuid)
}

// getNodeByIDForEgressFw resolves routing-node mesh addresses for NAT-aware egress firewall rules (tests may override).
var getNodeByIDForEgressFw = GetNodeByID

var GetEgressUserRulesForNode = func(targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	return rules
}
var GetUserAclRulesForNode = func(targetnode *models.Node,
	rules map[string]models.AclRule) map[string]models.AclRule {
	return rules
}

var GetFwRulesForUserNodesOnGw = func(node models.Node, nodes []models.Node) (rules []models.FwRule) { return }

func getEgressToEgressPoliciesForNode(targetnode models.Node) []models.Acl {
	policies := getDevicePoliciesByNetwork(schema.NetworkID(targetnode.Network))
	filtered := make([]models.Acl, 0)
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		if !isEgressToEgressPolicyForTarget(policy, targetnode) {
			continue
		}
		filtered = append(filtered, policy)
	}
	return filtered
}

func isEgressToEgressPolicyForTarget(policy models.Acl, targetnode models.Node) bool {
	srcEgresses := getEgressesFromPolicyTags(policy.Src, targetnode.Network)
	if len(srcEgresses) == 0 {
		return false
	}
	dstEgresses := getEgressesFromPolicyTags(policy.Dst, targetnode.Network)
	if len(dstEgresses) == 0 {
		return false
	}
	targetRoutesSrcEgress := targetNodeRoutesAnyEgress(targetnode, srcEgresses)
	targetRoutesDstEgress := targetNodeRoutesAnyEgress(targetnode, dstEgresses)
	// Both Uni and Bi policies require a forward rule on EVERY node the packet
	// traverses, i.e. both the src-egress router AND the dst-egress router.
	// (Bi additionally needs a reverse leg, emitted further down the pipeline.)
	// Only skip when this node hosts neither egress and therefore would never
	// see the packet.
	return targetRoutesSrcEgress || targetRoutesDstEgress
}

func getEgressesFromPolicyTags(tags []models.AclPolicyTag, network string) []schema.Egress {
	egresses := make([]schema.Egress, 0)
	seen := make(map[string]struct{})
	for _, tag := range tags {
		switch {
		case tag.Value == "*":
			list, err := getEgressByNetwork(network)
			if err != nil {
				continue
			}
			for _, e := range list {
				if _, ok := seen[e.ID]; ok {
					continue
				}
				seen[e.ID] = struct{}{}
				egresses = append(egresses, e)
			}
		case tag.ID == models.EgressID || tag.ID == models.EgressRange:
			e, err := getEgressByID(tag.Value)
			if err != nil {
				continue
			}
			if _, ok := seen[e.ID]; ok {
				continue
			}
			seen[e.ID] = struct{}{}
			egresses = append(egresses, e)
		}
	}
	return egresses
}

func targetNodeRoutesAnyEgress(targetnode models.Node, egresses []schema.Egress) bool {
	targetID := targetnode.ID.String()
	for _, e := range egresses {
		if !e.Status || len(e.Nodes) == 0 {
			continue
		}
		if _, ok := e.Nodes[targetID]; ok {
			return true
		}
	}
	return false
}

// IsEgressRoutingPolicyAllowedForNodes reports whether `policy` permits a
// peering relationship (and corresponding mesh peer ACL rule) between `node`
// and `peer` on either side of an egress<->egress flow. WireGuard peering is
// inherently bidirectional: even a Uni "src-egress -> dst-egress" policy
// requires the src-router and dst-router hosts to complete a wg handshake so
// the tunnel can carry the one-way L4 traffic. The L4 direction (Uni vs Bi)
// is then enforced downstream by the FORWARD/INPUT rule generators, not at
// peer-allow time. We therefore accept the policy whenever EITHER side of the
// pair routes the matching egress, otherwise the dst-side router would never
// add the src-side router as a peer (callers query symmetrically as
// (X, Y) and (Y, X)) and the handshake would silently never occur.
func IsEgressRoutingPolicyAllowedForNodes(policy models.Acl, node, peer models.Node) bool {
	srcEgresses := getEgressesFromPolicyTags(policy.Src, node.Network)
	if len(srcEgresses) == 0 {
		return false
	}
	dstEgresses := getEgressesFromPolicyTags(policy.Dst, node.Network)
	if len(dstEgresses) == 0 {
		return false
	}

	nodeRoutesSrc := targetNodeRoutesAnyEgress(node, srcEgresses)
	nodeRoutesDst := targetNodeRoutesAnyEgress(node, dstEgresses)
	peerRoutesSrc := targetNodeRoutesAnyEgress(peer, srcEgresses)
	peerRoutesDst := targetNodeRoutesAnyEgress(peer, dstEgresses)

	forwardAllowed := nodeRoutesSrc && peerRoutesDst
	reverseAllowed := nodeRoutesDst && peerRoutesSrc
	return forwardAllowed || reverseAllowed
}

const egressSiteACLReverseSuffix = "-reverse"

// crossSiteEgressIPNetPairs yields (src,dst) pairs with distinct CIDR strings so downstream
// firewall generation does not expand reflexive allows (e.g. 10.110.0.0/20 -> 10.110.0.0/20)
// when multiple egress LANs are merged into one policy.
func crossSiteEgressIPNetPairs(srcs, dsts []net.IPNet) []struct{ Src, Dst net.IPNet } {
	if len(srcs) == 0 || len(dsts) == 0 {
		return nil
	}
	var out []struct{ Src, Dst net.IPNet }
	for _, s := range srcs {
		for _, d := range dsts {
			if s.String() == d.String() {
				continue
			}
			out = append(out, struct{ Src, Dst net.IPNet }{s, d})
		}
	}
	return out
}

// egressSiteToSiteRuleKey returns the rules-map key for a site-to-site rule.
// The "#xs<idx>" suffix is always added (even for a single pair) so the key
// never collides with the main-loop's `acl.ID` (or `acl.ID-reverse`) rules,
// which are emitted from GetEgressRulesForNode for the same acl when its src
// also references mesh devices (NodeID/NodeTagID). Without the suffix the
// site-to-site rule would overwrite the device-mesh-IP rule under that key.
// The `total` arg is retained for call-site symmetry; idx is always used.
func egressSiteToSiteRuleKey(aclID string, reverse bool, idx int, _ int) string {
	suf := ""
	if reverse {
		suf = egressSiteACLReverseSuffix
	}
	return fmt.Sprintf("%s%s#xs%d", aclID, suf, idx)
}

func appendEgressSiteToSiteRules(
	rules map[string]models.AclRule,
	acl models.Acl,
	direction models.AllowedTrafficDirection,
	v4pairs, v6pairs []struct{ Src, Dst net.IPNet },
	reverse bool,
) {
	total := len(v4pairs) + len(v6pairs)
	if total == 0 {
		return
	}
	if len(v4pairs) == 1 && len(v6pairs) == 0 {
		id := egressSiteToSiteRuleKey(acl.ID, reverse, 0, 1)
		rules[id] = models.AclRule{
			ID:              id,
			AllowedProtocol: acl.Proto,
			AllowedPorts:    acl.Port,
			Direction:       direction,
			Allowed:         true,
			IPList:          []net.IPNet{v4pairs[0].Src},
			Dst:             []net.IPNet{v4pairs[0].Dst},
		}
		return
	}
	if len(v6pairs) == 1 && len(v4pairs) == 0 {
		id := egressSiteToSiteRuleKey(acl.ID, reverse, 0, 1)
		rules[id] = models.AclRule{
			ID:              id,
			AllowedProtocol: acl.Proto,
			AllowedPorts:    acl.Port,
			Direction:       direction,
			Allowed:         true,
			IP6List:         []net.IPNet{v6pairs[0].Src},
			Dst6:            []net.IPNet{v6pairs[0].Dst},
		}
		return
	}
	if len(v4pairs) == 1 && len(v6pairs) == 1 {
		id := egressSiteToSiteRuleKey(acl.ID, reverse, 0, 1)
		rules[id] = models.AclRule{
			ID:              id,
			AllowedProtocol: acl.Proto,
			AllowedPorts:    acl.Port,
			Direction:       direction,
			Allowed:         true,
			IPList:          []net.IPNet{v4pairs[0].Src},
			Dst:             []net.IPNet{v4pairs[0].Dst},
			IP6List:         []net.IPNet{v6pairs[0].Src},
			Dst6:            []net.IPNet{v6pairs[0].Dst},
		}
		return
	}
	idx := 0
	for _, p := range v4pairs {
		id := egressSiteToSiteRuleKey(acl.ID, reverse, idx, total)
		rules[id] = models.AclRule{
			ID:              id,
			AllowedProtocol: acl.Proto,
			AllowedPorts:    acl.Port,
			Direction:       direction,
			Allowed:         true,
			IPList:          []net.IPNet{p.Src},
			Dst:             []net.IPNet{p.Dst},
		}
		idx++
	}
	for _, p := range v6pairs {
		id := egressSiteToSiteRuleKey(acl.ID, reverse, idx, total)
		rules[id] = models.AclRule{
			ID:              id,
			AllowedProtocol: acl.Proto,
			AllowedPorts:    acl.Port,
			Direction:       direction,
			Allowed:         true,
			IP6List:         []net.IPNet{p.Src},
			Dst6:            []net.IPNet{p.Dst},
		}
		idx++
	}
}

func getEgressAclRulesForTargetNode(targetnode models.Node) map[string]models.AclRule {
	rules := make(map[string]models.AclRule)
	policies := getEgressToEgressPoliciesForNode(targetnode)
	targetID := targetnode.ID.String()
	for _, acl := range policies {
		srcEgresses := getEgressesFromPolicyTags(acl.Src, targetnode.Network)
		dstEgresses := getEgressesFromPolicyTags(acl.Dst, targetnode.Network)
		if len(srcEgresses) == 0 || len(dstEgresses) == 0 {
			continue
		}
		srcRouted := targetNodeRoutesAnyEgress(targetnode, srcEgresses)
		dstRouted := targetNodeRoutesAnyEgress(targetnode, dstEgresses)
		// A node only needs a rule for this policy when it actually sits on the
		// packet path. For both Uni (src -> dst) and Bi traffic, that means
		// either it routes the src-egress (LAN behind it is the source) or it
		// routes the dst-egress (LAN behind it is the destination). The Bi-only
		// reverse leg is appended after the forward leg below.
		if !srcRouted && !dstRouted {
			continue
		}
		origSrcIP4, origSrcIP6 := getSelectedEgressIPNets(acl.Src)
		if len(origSrcIP4) == 0 && len(origSrcIP6) == 0 {
			origSrcIP4, origSrcIP6 = getEgressCIDRsForSiteToSite(targetID, srcEgresses)
		} else {
			origSrcIP4, origSrcIP6 = selectedEgressIPNetsForSiteToSiteEgresses(targetID, srcEgresses, origSrcIP4, origSrcIP6)
		}
		dstIP4, dstIP6 := getSelectedEgressIPNets(acl.Dst)
		if len(dstIP4) == 0 && len(dstIP6) == 0 {
			dstIP4, dstIP6 = getEgressCIDRsForSiteToSite(targetID, dstEgresses)
		} else {
			dstIP4, dstIP6 = selectedEgressIPNetsForSiteToSiteEgresses(targetID, dstEgresses, dstIP4, dstIP6)
		}
		srcNat := egressListHasNAT(srcEgresses)
		dstNat := egressListHasNAT(dstEgresses)

		fwdSrcIP4 := append([]net.IPNet(nil), origSrcIP4...)
		fwdSrcIP6 := append([]net.IPNet(nil), origSrcIP6...)
		if dstRouted && !srcRouted && srcNat {
			m4, m6 := meshNetsForEgressRouters(srcEgresses, targetID)
			if len(m4) > 0 || len(m6) > 0 {
				fwdSrcIP4, fwdSrcIP6 = m4, m6
			}
		}

		fwdSrcIP4 = UniqueIPNetList(fwdSrcIP4)
		fwdSrcIP6 = UniqueIPNetList(fwdSrcIP6)
		dstIP4 = UniqueIPNetList(dstIP4)
		dstIP6 = UniqueIPNetList(dstIP6)

		if len(fwdSrcIP4) == 0 && len(fwdSrcIP6) == 0 {
			continue
		}
		if len(dstIP4) == 0 && len(dstIP6) == 0 {
			continue
		}

		v4pairs := crossSiteEgressIPNetPairs(fwdSrcIP4, dstIP4)
		v6pairs := crossSiteEgressIPNetPairs(fwdSrcIP6, dstIP6)
		if len(v4pairs) == 0 && len(v6pairs) == 0 {
			continue
		}

		appendEgressSiteToSiteRules(rules, acl, acl.AllowedDirection, v4pairs, v6pairs, false)

		if acl.AllowedDirection == models.TrafficDirectionBi {
			revSrcIP4 := append([]net.IPNet(nil), dstIP4...)
			revSrcIP6 := append([]net.IPNet(nil), dstIP6...)
			if srcRouted && !dstRouted && dstNat {
				m4, m6 := meshNetsForEgressRouters(dstEgresses, targetID)
				if len(m4) > 0 || len(m6) > 0 {
					revSrcIP4, revSrcIP6 = m4, m6
				}
			}
			revDst4 := append([]net.IPNet(nil), origSrcIP4...)
			revDst6 := append([]net.IPNet(nil), origSrcIP6...)
			revSrcIP4 = UniqueIPNetList(revSrcIP4)
			revSrcIP6 = UniqueIPNetList(revSrcIP6)
			revDst4 = UniqueIPNetList(revDst4)
			revDst6 = UniqueIPNetList(revDst6)

			revV4 := crossSiteEgressIPNetPairs(revSrcIP4, revDst4)
			revV6 := crossSiteEgressIPNetPairs(revSrcIP6, revDst6)
			if len(revV4) == 0 && len(revV6) == 0 {
				continue
			}
			appendEgressSiteToSiteRules(rules, acl, models.TrafficDirectionUni, revV4, revV6, true)
		}
	}
	return rules
}

func getEgressCIDRs(egresses []schema.Egress) (ip4 []net.IPNet, ip6 []net.IPNet) {
	for _, e := range egresses {
		if !e.Status {
			continue
		}
		if e.Range == "" {
			continue
		}
		_, cidr, err := net.ParseCIDR(e.Range)
		if err != nil {
			continue
		}
		if cidr.IP.To4() != nil {
			ip4 = append(ip4, *cidr)
		} else {
			ip6 = append(ip6, *cidr)
		}
	}
	return
}

// getEgressCIDRsForSiteToSite returns full egress range CIDRs from the routing
// node's perspective (virtual range when the node does not own the egress).
func getEgressCIDRsForSiteToSite(nodeID string, egresses []schema.Egress) (ip4, ip6 []net.IPNet) {
	for _, e := range egresses {
		if !e.Status {
			continue
		}
		egressRange := e.Range
		if _, owns := e.Nodes[nodeID]; !owns && e.VirtualRange != "" {
			egressRange = e.VirtualRange
		}
		if egressRange == "" {
			continue
		}
		_, cidr, err := net.ParseCIDR(egressRange)
		if err != nil {
			continue
		}
		if cidr.IP.To4() != nil {
			ip4 = append(ip4, *cidr)
		} else {
			ip6 = append(ip6, *cidr)
		}
	}
	return
}

func egressContainingIPNet(egresses []schema.Egress, cidr net.IPNet) (schema.Egress, bool) {
	for _, e := range egresses {
		if e.Range == "" {
			continue
		}
		_, realNet, err := net.ParseCIDR(e.Range)
		if err != nil {
			continue
		}
		if cidrContainsCIDR(realNet, &cidr) {
			return e, true
		}
	}
	return schema.Egress{}, false
}

// selectedEgressIPNetsForSiteToSiteEgresses maps restricted IPs per egress using
// the site-to-site routing node's perspective (real on owner, virtual elsewhere).
func selectedEgressIPNetsForSiteToSiteEgresses(
	nodeID string,
	egresses []schema.Egress,
	selected4, selected6 []net.IPNet,
) (v4, v6 []net.IPNet) {
	for _, cidr := range selected4 {
		e, ok := egressContainingIPNet(egresses, cidr)
		if !ok {
			continue
		}
		m4, _ := SelectedEgressDstNetsForNode(nodeID, e, []net.IPNet{cidr}, nil)
		v4 = append(v4, m4...)
	}
	for _, cidr := range selected6 {
		e, ok := egressContainingIPNet(egresses, cidr)
		if !ok {
			continue
		}
		_, m6 := SelectedEgressDstNetsForNode(nodeID, e, nil, []net.IPNet{cidr})
		v6 = append(v6, m6...)
	}
	return v4, v6
}

func egressListHasNAT(egresses []schema.Egress) bool {
	for _, e := range egresses {
		if e.Nat {
			return true
		}
	}
	return false
}

func meshNetsForEgressRouters(egresses []schema.Egress, excludeNodeID string) (ip4 []net.IPNet, ip6 []net.IPNet) {
	seen := make(map[string]struct{})
	for _, eg := range egresses {
		for nodeID := range eg.Nodes {
			if nodeID == excludeNodeID {
				continue
			}
			n, err := getNodeByIDForEgressFw(nodeID)
			if err != nil {
				continue
			}
			if n.Address.IP != nil {
				nw := n.AddressIPNet4()
				key := nw.String()
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					ip4 = append(ip4, nw)
				}
			}
			if n.Address6.IP != nil {
				nw := n.AddressIPNet6()
				key := nw.String()
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					ip6 = append(ip6, nw)
				}
			}
		}
	}
	return
}

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
	defaultDevicePolicy, _ := GetDefaultPolicy(schema.NetworkID(node.Network), models.DevicePolicy)
	nodes, _ := GetNetworkNodes(node.Network)
	nodes = append(nodes, GetStaticNodesByNetwork(schema.NetworkID(node.Network), true)...)
	rules = GetFwRulesForUserNodesOnGw(node, nodes)
	if defaultDevicePolicy.Enabled {
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

	// For each extclient attached to this gateway, emit explicit allow rules to every
	// egress range it has policy access to (including egresses hosted on other nodes).
	// The peer-iteration above only adds the rule when it happens to iterate the
	// egress-owning peer; that misses orphan/stale egresses and is also brittle when
	// the egress->node ownership map is partial. This pass keys directly off the
	// extclient + egress, which is what netclient needs on the forward chain so
	// EC -> remote_egress traffic isn't dropped at this gateway.
	rules = append(rules, getExtClientEgressFwRulesOnIngressGw(node)...)
	// Same idea, but for relayed mesh devices: the blanket NetworkRange <-> relayed
	// rule below covers in-mesh traffic; it does not cover traffic from a relayed
	// device to an external egress range (LAN CIDR / VirtualRange / domain CIDRs),
	// so those packets would otherwise be dropped on the relay's forward chain.
	rules = append(rules, getDeviceEgressFwRulesOnIngressGw(node)...)

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
		selectedIP4, selectedIP6 := getSelectedEgressIPNets(policy.Dst)
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
				if HasEgressDomainAns(e) {
					if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
						sel4, sel6 := SelectedEgressDstNetsForNode(node.ID.String(), e, selectedIP4, selectedIP6)
						for _, cidr := range sel4 {
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
						for _, cidr := range sel6 {
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
					for _, domainAnsI := range AllDomainAnsFromEgress(e) {
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
					if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
						sel4, sel6 := SelectedEgressDstNetsForNode(node.ID.String(), e, selectedIP4, selectedIP6)
						for _, cidr := range sel4 {
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
						for _, cidr := range sel6 {
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
					// Use virtual range if node doesn't own the egress, otherwise use regular range
					egressRange := e.Range
					if !nodeOwnsEgress && e.VirtualRange != "" {
						egressRange = e.VirtualRange
					}
					if egressRange != "" {
						dstI.Value = egressRange

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
	defaultUserPolicy, _ := GetDefaultPolicy(schema.NetworkID(node.Network), models.UserPolicy)
	defaultDevicePolicy, _ := GetDefaultPolicy(schema.NetworkID(node.Network), models.DevicePolicy)

	extclients := GetStaticNodesByNetwork(schema.NetworkID(node.Network), false)
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

var CleanupGwsMigration = func() {}

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
		taggedNodes = GetTagMapWithNodesByNetwork(schema.NetworkID(targetnode.Network), false)
	} else {
		taggedNodes = GetTagMapWithNodesByNetwork(schema.NetworkID(targetnode.Network), true)
	}
	acls := getDevicePoliciesByNetwork(schema.NetworkID(targetnode.Network))
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
		// Expand any EgressID entries on the src side into the egress's owner
		// node IDs. ConvAclTagToValueMap collapses every tag to its raw Value,
		// so an "EgressID:blr-eg" entry only ends up in srcTags as the literal
		// "blr-eg" key, which never matches any node in taggedNodes/GetNodeByID
		// below. As a result, a site-to-site policy whose src is "an egress"
		// (rather than a node-tag/node-id) would emit a rule on the dst-side
		// egress node with no entries from the src-egress mesh routers in the
		// IPList - so the dst node's INPUT chain would silently drop incoming
		// wg traffic from the src-egress router and the handshake/connection
		// would never establish. Inserting the egress's owner node IDs into
		// srcTags lets the existing taggedNodes[NodeID] / GetNodeByID(NodeID)
		// resolution add their mesh AddressIPNet4/6 to the rule's IPList.
		for _, src := range acl.Src {
			if src.ID != models.EgressID {
				continue
			}
			e, err := getEgressByID(src.Value)
			if err == nil && e.Status {
				for nodeID := range e.Nodes {
					srcTags[nodeID] = struct{}{}
				}
			}
		}
		egressRanges4 := []net.IPNet{}
		egressRanges6 := []net.IPNet{}
		selectedIP4, selectedIP6 := getSelectedEgressIPNets(acl.Dst)
		for _, dst := range acl.Dst {
			if dst.Value == "*" {
				e := schema.Egress{Network: targetnode.Network}
				eli, _ := e.ListByNetwork(db.WithContext(context.Background()))
				for _, eI := range eli {
					if !eI.Status || len(eI.Nodes) == 0 {
						continue
					}
					nodeOwnsEgress := false
					if _, ok := eI.Nodes[targetnode.ID.String()]; ok {
						nodeOwnsEgress = true
						dstTags[targetnode.ID.String()] = struct{}{}
					}
					if nodeOwnsEgress {
						if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
							sel4, sel6 := SelectedEgressDstNetsForNode(targetnode.ID.String(), eI, selectedIP4, selectedIP6)
							egressRanges4 = append(egressRanges4, sel4...)
							egressRanges6 = append(egressRanges6, sel6...)
						} else if servercfg.IsPro && IsDomainBasedEgress(eI) && HasEgressDomainAns(eI) {
							for _, domainAnsI := range AllDomainAnsFromEgress(eI) {
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
					} else if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
						sel4, sel6 := SelectedEgressDstNetsForNode(targetnode.ID.String(), eI, selectedIP4, selectedIP6)
						egressRanges4 = append(egressRanges4, sel4...)
						egressRanges6 = append(egressRanges6, sel6...)
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
				e, err := getEgressByID(dst.Value)
				if err == nil && e.Status && len(e.Nodes) > 0 {
					nodeOwnsEgress := false
					if _, ok := e.Nodes[targetnode.ID.String()]; ok {
						nodeOwnsEgress = true
						dstTags[targetnode.ID.String()] = struct{}{}
					}
					if nodeOwnsEgress {
						if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
							sel4, sel6 := SelectedEgressDstNetsForNode(targetnode.ID.String(), e, selectedIP4, selectedIP6)
							egressRanges4 = append(egressRanges4, sel4...)
							egressRanges6 = append(egressRanges6, sel6...)
						} else if servercfg.IsPro && IsDomainBasedEgress(e) && HasEgressDomainAns(e) {
							for _, domainAnsI := range AllDomainAnsFromEgress(e) {
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
					} else if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
						sel4, sel6 := SelectedEgressDstNetsForNode(targetnode.ID.String(), e, selectedIP4, selectedIP6)
						egressRanges4 = append(egressRanges4, sel4...)
						egressRanges6 = append(egressRanges6, sel6...)
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

// GetEgressDefaultAllowAllFwRule returns one bidirectional allow from the node's VPN (mesh) CIDR(s)
// to every egress LAN range this gateway advertises, for default all-resources device+user policies.
// Netclients use this to install a single mesh → LAN ACCEPT (e.g. 100.64.0.0/16 → 10.104.0.0/20).
func GetEgressDefaultAllowAllFwRule(node models.Node) (models.AclRule, bool) {
	if !node.EgressDetails.IsEgressGateway || len(node.EgressDetails.EgressGatewayRequest.Ranges) == 0 {
		return models.AclRule{}, false
	}
	if node.NetworkRange.IP == nil && node.NetworkRange6.IP == nil {
		return models.AclRule{}, false
	}
	var dst4, dst6 []net.IPNet
	for _, r := range node.EgressDetails.EgressGatewayRequest.Ranges {
		_, cidr, err := net.ParseCIDR(r)
		if err != nil {
			continue
		}
		if cidr.IP.To4() != nil {
			dst4 = append(dst4, *cidr)
		} else {
			dst6 = append(dst6, *cidr)
		}
	}
	if len(dst4) == 0 && len(dst6) == 0 {
		return models.AclRule{}, false
	}
	rule := models.AclRule{
		ID:              fmt.Sprintf("%s-egress-all-rsrc-mesh", node.ID.String()),
		AllowedProtocol: models.ALL,
		Direction:       models.TrafficDirectionBi,
		Allowed:         true,
	}
	if node.NetworkRange.IP != nil {
		rule.IPList = []net.IPNet{node.NetworkRange}
	}
	if node.NetworkRange6.IP != nil {
		rule.IP6List = []net.IPNet{node.NetworkRange6}
	}
	if len(dst4) > 0 {
		rule.Dst = UniqueIPNetList(dst4)
	}
	if len(dst6) > 0 {
		rule.Dst6 = UniqueIPNetList(dst6)
	}
	return rule, true
}

func GetEgressRulesForNode(targetnode models.Node) (rules map[string]models.AclRule) {
	rules = make(map[string]models.AclRule)
	defer func() {
		rules = GetEgressUserRulesForNode(&targetnode, rules)
	}()
	taggedNodes := GetTagMapWithNodesByNetwork(schema.NetworkID(targetnode.Network), true)

	acls := getDevicePoliciesByNetwork(schema.NetworkID(targetnode.Network))
	var targetNodeTags = make(map[models.TagID]struct{})
	targetNodeTags[models.TagID(targetnode.ID.String())] = struct{}{}
	targetNodeTags["*"] = struct{}{}
	if targetnode.IsGw && !servercfg.IsPro {
		targetNodeTags[models.TagID(fmt.Sprintf("%s.%s", targetnode.Network, models.GwTagName))] = struct{}{}
	}

	egs, _ := getEgressByNetwork(targetnode.Network)
	if len(egs) == 0 {
		return
	}
	var egressIDMap = make(map[string]schema.Egress)
	var remoteEgresses = make(map[string]schema.Egress)
	for _, egI := range egs {
		if !egI.Status {
			continue
		}
		if _, ok := egI.Nodes[targetnode.ID.String()]; ok {
			egressIDMap[egI.ID] = egI
		} else {
			remoteEgresses[egI.ID] = egI
		}
	}
	if len(egressIDMap) == 0 && len(remoteEgresses) == 0 {
		return
	}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcTags := ConvAclTagToValueMap(acl.Src)
		dstTags := ConvAclTagToValueMap(acl.Dst)
		selectedIP4, selectedIP6 := getSelectedEgressIPNets(acl.Dst)
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
				if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
					aclRule.Dst = append(aclRule.Dst, selectedIP4...)
					aclRule.Dst6 = append(aclRule.Dst6, selectedIP6...)
				} else if servercfg.IsPro && IsDomainBasedEgress(egI) && HasEgressDomainAns(egI) {
					for _, domainAnsI := range AllDomainAnsFromEgress(egI) {
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
			aclRule.Dst = UniqueIPNetList(aclRule.Dst)
			aclRule.Dst6 = UniqueIPNetList(aclRule.Dst6)
			rules[acl.ID] = aclRule

			// Bi-directional device->egress policies need an explicit reverse rule
			// (egress IPs/range -> src devices) emitted as a separate AclRule, because the
			// downstream firewall generator pairs IPList x Dst rather than expanding the
			// Bi direction into both legs.
			if acl.AllowedDirection == models.TrafficDirectionBi &&
				(len(aclRule.Dst) > 0 || len(aclRule.Dst6) > 0) {
				revID := acl.ID + egressSiteACLReverseSuffix
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

	for aclID, aclRule := range appendExtClientRemoteEgressFwdRules(targetnode, acls, remoteEgresses) {
		rules[aclID] = aclRule
	}

	for aclID, aclRule := range appendDeviceRemoteEgressFwdRules(targetnode, acls, remoteEgresses) {
		rules[aclID] = aclRule
	}

	for aclID, aclRule := range getEgressAclRulesForTargetNode(targetnode) {
		rules[aclID] = aclRule
	}

	return
}

// appendExtClientRemoteEgressFwdRules emits forward-chain rules for extclients attached
// to targetnode (as their ingress gateway) so traffic to egress ranges hosted on OTHER
// nodes is not dropped at targetnode. Without these, even when the per-policy ingress
// rule allows the packet at the ingress chain, the egress/forward chain on targetnode
// has no matching rule because the remote egress is not in egressIDMap. The emitted
// rules are keyed with the "#ext-fwd" suffix to avoid colliding with the local-egress
// rules keyed by acl.ID, and a "-reverse" companion is added for Bi policies.
func appendExtClientRemoteEgressFwdRules(
	targetnode models.Node,
	acls []models.Acl,
	remoteEgresses map[string]schema.Egress,
) map[string]models.AclRule {
	out := make(map[string]models.AclRule)
	if len(remoteEgresses) == 0 {
		return out
	}
	extclients, err := listNetworkExtClients(targetnode.Network)
	if err != nil {
		return out
	}
	attached := extclients[:0:0]
	for _, ec := range extclients {
		if !ec.Enabled {
			continue
		}
		if ec.IngressGatewayID != targetnode.ID.String() {
			continue
		}
		// user-policy extclients are handled by GetEgressUserRulesForNode
		if ec.RemoteAccessClientID != "" {
			continue
		}
		attached = append(attached, ec)
	}
	if len(attached) == 0 {
		return out
	}

	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcTags := ConvAclTagToValueMap(acl.Src)
		dstTags := ConvAclTagToValueMap(acl.Dst)
		_, srcAll := srcTags["*"]
		_, dstAll := dstTags["*"]
		selectedIP4, selectedIP6 := getSelectedEgressIPNets(acl.Dst)

		var dst4, dst6 []net.IPNet
		for egID, egI := range remoteEgresses {
			if _, ok := dstTags[egID]; !ok && !dstAll {
				continue
			}
			if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
				sel4, sel6 := SelectedEgressDstNetsForNode(targetnode.ID.String(), egI, selectedIP4, selectedIP6)
				dst4 = append(dst4, sel4...)
				dst6 = append(dst6, sel6...)
				continue
			}
			if servercfg.IsPro && IsDomainBasedEgress(egI) && HasEgressDomainAns(egI) {
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
				continue
			}
			// at the forward chain on the ingress gw, packets carry the address the
			// extclient was told to use (virtual_range when set, else range).
			egressRange := egI.Range
			if egI.VirtualRange != "" {
				egressRange = egI.VirtualRange
			}
			if egressRange == "" {
				continue
			}
			ip, cidr, parseErr := net.ParseCIDR(egressRange)
			if parseErr != nil {
				continue
			}
			if ip.To4() != nil {
				dst4 = append(dst4, *cidr)
			} else {
				dst6 = append(dst6, *cidr)
			}
		}
		if len(dst4) == 0 && len(dst6) == 0 {
			continue
		}

		var srcIP4, srcIP6 []net.IPNet
		for _, ec := range attached {
			if !extclientMatchesAclSrc(ec, srcTags, srcAll) {
				continue
			}
			if ec.Address != "" {
				srcIP4 = append(srcIP4, ec.AddressIPNet4())
			}
			if ec.Address6 != "" {
				srcIP6 = append(srcIP6, ec.AddressIPNet6())
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
		out[ruleID] = aclRule

		if acl.AllowedDirection == models.TrafficDirectionBi &&
			(len(aclRule.Dst) > 0 || len(aclRule.Dst6) > 0) {
			revID := ruleID + egressSiteACLReverseSuffix
			out[revID] = models.AclRule{
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

	return out
}

// appendDeviceRemoteEgressFwdRules is the relayed-mesh-device twin of
// appendExtClientRemoteEgressFwdRules: for nodes relayed by targetnode, emit
// forward-chain AclRules so traffic from a relayed device to egress ranges
// hosted on OTHER nodes is not dropped at targetnode. Rules are keyed
// "<acl.ID>#dev-fwd" to avoid colliding with the extclient or local-egress
// keys, and a "-reverse" companion is added for Bi policies.
func appendDeviceRemoteEgressFwdRules(
	targetnode models.Node,
	acls []models.Acl,
	remoteEgresses map[string]schema.Egress,
) map[string]models.AclRule {
	out := make(map[string]models.AclRule)
	if len(remoteEgresses) == 0 || len(targetnode.RelayedNodes) == 0 {
		return out
	}
	var relayed []models.Node
	for _, id := range targetnode.RelayedNodes {
		r, err := getNodeByID(id)
		if err != nil {
			continue
		}
		relayed = append(relayed, r)
	}
	if len(relayed) == 0 {
		return out
	}

	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcTags := ConvAclTagToValueMap(acl.Src)
		_, srcAll := srcTags["*"]
		dst4, dst6 := computeEgressDstsForAcl(targetnode.ID.String(), acl, remoteEgresses)
		if len(dst4) == 0 && len(dst6) == 0 {
			continue
		}

		var srcIP4, srcIP6 []net.IPNet
		for _, r := range relayed {
			if !nodeMatchesAclSrc(r, srcTags, srcAll) {
				continue
			}
			if r.Address.IP != nil {
				srcIP4 = append(srcIP4, r.AddressIPNet4())
			}
			if r.Address6.IP != nil {
				srcIP6 = append(srcIP6, r.AddressIPNet6())
			}
		}
		if len(srcIP4) == 0 && len(srcIP6) == 0 {
			continue
		}

		ruleID := acl.ID + "#dev-fwd"
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
		out[ruleID] = aclRule

		if acl.AllowedDirection == models.TrafficDirectionBi &&
			(len(aclRule.Dst) > 0 || len(aclRule.Dst6) > 0) {
			revID := ruleID + egressSiteACLReverseSuffix
			out[revID] = models.AclRule{
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

	return out
}

// getExtClientEgressFwRulesOnIngressGw emits []models.FwRule entries for every
// (attached extclient, egress range) pair allowed by an enabled device policy.
// This is the FwRule (IngressInfo.Rules) twin of appendExtClientRemoteEgressFwdRules
// and exists so the ingress gateway's forward chain has the rule even when this
// gateway is not itself an egress gateway (in which case GetEgressRulesForNode
// is never called for it).
func getExtClientEgressFwRulesOnIngressGw(node models.Node) (rules []models.FwRule) {
	extclients, err := listNetworkExtClients(node.Network)
	if err != nil {
		return
	}
	var attached []models.ExtClient
	for _, ec := range extclients {
		if !ec.Enabled {
			continue
		}
		if ec.IngressGatewayID != node.ID.String() {
			continue
		}
		// user-policy extclients are handled by GetFwRulesForUserNodesOnGw
		if ec.RemoteAccessClientID != "" {
			continue
		}
		attached = append(attached, ec)
	}
	if len(attached) == 0 {
		return
	}

	egs, err := getEgressByNetwork(node.Network)
	if err != nil || len(egs) == 0 {
		return
	}
	egByID := make(map[string]schema.Egress, len(egs))
	for _, eg := range egs {
		if !eg.Status {
			continue
		}
		egByID[eg.ID] = eg
	}
	if len(egByID) == 0 {
		return
	}

	acls := getDevicePoliciesByNetwork(schema.NetworkID(node.Network))
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcTags := ConvAclTagToValueMap(acl.Src)
		_, srcAll := srcTags["*"]
		dst4, dst6 := computeEgressDstsForAcl(node.ID.String(), acl, egByID)
		if len(dst4) == 0 && len(dst6) == 0 {
			continue
		}
		for _, ec := range attached {
			if !extclientMatchesAclSrc(ec, srcTags, srcAll) {
				continue
			}
			var src4, src6 net.IPNet
			if ec.Address != "" {
				src4 = ec.AddressIPNet4()
			}
			if ec.Address6 != "" {
				src6 = ec.AddressIPNet6()
			}
			rules = append(rules, emitEgressFwRulesForSrc(acl, src4, src6, dst4, dst6)...)
		}
	}
	return
}

// getDeviceEgressFwRulesOnIngressGw is the relayed-mesh-device twin of
// getExtClientEgressFwRulesOnIngressGw: for every node relayed by this gateway,
// emit allow rules to every egress range the relayed device has policy access to.
// Without this the relay's forward chain only allows traffic between the mesh
// network range and the relayed device, so relayed_device -> external_egress_range
// traffic gets dropped here.
func getDeviceEgressFwRulesOnIngressGw(node models.Node) (rules []models.FwRule) {
	if len(node.RelayedNodes) == 0 {
		return
	}
	var relayed []models.Node
	for _, id := range node.RelayedNodes {
		r, err := getNodeByID(id)
		if err != nil {
			continue
		}
		relayed = append(relayed, r)
	}
	if len(relayed) == 0 {
		return
	}

	egs, err := getEgressByNetwork(node.Network)
	if err != nil || len(egs) == 0 {
		return
	}
	egByID := make(map[string]schema.Egress, len(egs))
	for _, eg := range egs {
		if !eg.Status {
			continue
		}
		egByID[eg.ID] = eg
	}
	if len(egByID) == 0 {
		return
	}

	acls := getDevicePoliciesByNetwork(schema.NetworkID(node.Network))
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcTags := ConvAclTagToValueMap(acl.Src)
		_, srcAll := srcTags["*"]
		dst4, dst6 := computeEgressDstsForAcl(node.ID.String(), acl, egByID)
		if len(dst4) == 0 && len(dst6) == 0 {
			continue
		}
		for _, r := range relayed {
			if !nodeMatchesAclSrc(r, srcTags, srcAll) {
				continue
			}
			var src4, src6 net.IPNet
			if r.Address.IP != nil {
				src4 = r.AddressIPNet4()
			}
			if r.Address6.IP != nil {
				src6 = r.AddressIPNet6()
			}
			rules = append(rules, emitEgressFwRulesForSrc(acl, src4, src6, dst4, dst6)...)
		}
	}
	return
}

// computeEgressDstsForAcl returns the IPv4 and IPv6 destination CIDRs an acl
// grants access to across egByID, viewed from `nodeID`'s perspective: the
// destination address a source on this node sees depends on whether the node
// owns the egress (use Range) or not (prefer VirtualRange when set). Selected
// egress IPs (NetmakerIPAclID entries in acl.Dst) take precedence over the
// egress range, and domain answers take precedence over Range when configured.
func computeEgressDstsForAcl(
	nodeID string,
	acl models.Acl,
	egByID map[string]schema.Egress,
) (dst4, dst6 []net.IPNet) {
	dstTags := ConvAclTagToValueMap(acl.Dst)
	_, dstAll := dstTags["*"]
	selectedIP4, selectedIP6 := getSelectedEgressIPNets(acl.Dst)
	for egID, egI := range egByID {
		if _, ok := dstTags[egID]; !ok && !dstAll {
			continue
		}
		if len(selectedIP4) > 0 || len(selectedIP6) > 0 {
			sel4, sel6 := SelectedEgressDstNetsForNode(nodeID, egI, selectedIP4, selectedIP6)
			dst4 = append(dst4, sel4...)
			dst6 = append(dst6, sel6...)
			continue
		}
		if servercfg.IsPro && IsDomainBasedEgress(egI) && HasEgressDomainAns(egI) {
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
			continue
		}
		nodeOwnsEgress := false
		if _, ok := egI.Nodes[nodeID]; ok {
			nodeOwnsEgress = true
		}
		egressRange := egI.Range
		if !nodeOwnsEgress && egI.VirtualRange != "" {
			egressRange = egI.VirtualRange
		}
		if egressRange == "" {
			continue
		}
		ip, cidr, parseErr := net.ParseCIDR(egressRange)
		if parseErr != nil {
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

// emitEgressFwRulesForSrc emits FwRule entries for a single src address pair
// against the dst CIDRs. For Bi-directional acls a reverse leg is also emitted
// so return traffic from the egress range back to the source is allowed.
// Zero-valued IPNets (IP == nil) are treated as "this address family not present".
func emitEgressFwRulesForSrc(acl models.Acl, src4, src6 net.IPNet, dst4, dst6 []net.IPNet) (rules []models.FwRule) {
	if src4.IP != nil {
		for _, cidr := range dst4 {
			rules = append(rules, models.FwRule{
				SrcIP:           src4,
				DstIP:           cidr,
				AllowedProtocol: acl.Proto,
				AllowedPorts:    acl.Port,
				Allow:           true,
			})
		}
	}
	if src6.IP != nil {
		for _, cidr := range dst6 {
			rules = append(rules, models.FwRule{
				SrcIP:           src6,
				DstIP:           cidr,
				AllowedProtocol: acl.Proto,
				AllowedPorts:    acl.Port,
				Allow:           true,
			})
		}
	}
	if acl.AllowedDirection != models.TrafficDirectionBi {
		return
	}
	if src4.IP != nil {
		for _, cidr := range dst4 {
			rules = append(rules, models.FwRule{
				SrcIP:           cidr,
				DstIP:           src4,
				AllowedProtocol: acl.Proto,
				AllowedPorts:    acl.Port,
				Allow:           true,
			})
		}
	}
	if src6.IP != nil {
		for _, cidr := range dst6 {
			rules = append(rules, models.FwRule{
				SrcIP:           cidr,
				DstIP:           src6,
				AllowedProtocol: acl.Proto,
				AllowedPorts:    acl.Port,
				Allow:           true,
			})
		}
	}
	return
}

// extclientMatchesAclSrc reports whether an extclient is permitted as a source by an acl,
// matching on its ClientID or any of its tags (mirroring how AddTagMapWithStaticNodes
// keys the tag map).
func extclientMatchesAclSrc(ec models.ExtClient, srcTags map[string]struct{}, srcAll bool) bool {
	if srcAll {
		return true
	}
	if _, ok := srcTags[ec.ClientID]; ok {
		return true
	}
	if ec.Mutex != nil {
		ec.Mutex.Lock()
		defer ec.Mutex.Unlock()
	}
	for tag := range ec.Tags {
		if _, ok := srcTags[tag.String()]; ok {
			return true
		}
	}
	return false
}

// nodeMatchesAclSrc reports whether a mesh node is permitted as a source by an acl,
// matching on its node UUID or any of its tags.
func nodeMatchesAclSrc(n models.Node, srcTags map[string]struct{}, srcAll bool) bool {
	if srcAll {
		return true
	}
	if _, ok := srcTags[n.ID.String()]; ok {
		return true
	}
	if n.Mutex != nil {
		n.Mutex.Lock()
		defer n.Mutex.Unlock()
	}
	for tag := range n.Tags {
		if _, ok := srcTags[tag.String()]; ok {
			return true
		}
	}
	return false
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

func NormalizeIPOrCIDR(value string) (string, error) {
	if value == "" {
		return "", errors.New("empty ip/cidr value")
	}
	if normalizedCIDR, err := NormalizeCIDR(value); err == nil {
		return normalizedCIDR, nil
	}
	ip := net.ParseIP(value)
	if ip == nil {
		return "", errors.New("invalid ip/cidr value: " + value)
	}
	if ip.To4() != nil {
		return (&net.IPNet{IP: ip.To4(), Mask: net.CIDRMask(32, 32)}).String(), nil
	}
	return (&net.IPNet{IP: ip.To16(), Mask: net.CIDRMask(128, 128)}).String(), nil
}

func cidrContainsCIDR(parent, child *net.IPNet) bool {
	if parent == nil || child == nil {
		return false
	}
	parentBits, _ := parent.Mask.Size()
	childBits, _ := child.Mask.Size()
	if childBits < parentBits {
		return false
	}
	if !parent.Contains(child.IP) {
		return false
	}
	last := make(net.IP, len(child.IP))
	copy(last, child.IP)
	for i := 0; i < len(child.Mask); i++ {
		last[i] |= ^child.Mask[i]
	}
	return parent.Contains(last)
}

// MapEgressIPNetToVirtualNAT maps a real LAN CIDR from an egress range into the
// corresponding address in the egress virtual NAT range (host offset preserved).
func MapEgressIPNetToVirtualNAT(cidr net.IPNet, e schema.Egress) (net.IPNet, bool) {
	if e.VirtualRange == "" || e.Range == "" {
		return net.IPNet{}, false
	}
	_, realNet, err := net.ParseCIDR(e.Range)
	if err != nil {
		return net.IPNet{}, false
	}
	_, virtNet, err := net.ParseCIDR(e.VirtualRange)
	if err != nil {
		return net.IPNet{}, false
	}
	if realNet.IP.To4() == nil || virtNet.IP.To4() == nil || cidr.IP.To4() == nil {
		return net.IPNet{}, false
	}
	if !realNet.Contains(cidr.IP) {
		return net.IPNet{}, false
	}
	virtBase := virtNet.IP.Mask(virtNet.Mask)
	mapped := make(net.IP, 4)
	copy(mapped, virtBase.To4())
	for i := 0; i < 4; i++ {
		mapped[i] |= cidr.IP[i] & ^realNet.Mask[i]
	}
	return net.IPNet{IP: mapped, Mask: cidr.Mask}, true
}

// MapSelectedEgressIPNetsToVirtualNAT maps restricted egress IPs that fall within
// e.Range into e.VirtualRange; entries outside the real range are skipped.
func MapSelectedEgressIPNetsToVirtualNAT(selected4, selected6 []net.IPNet, e schema.Egress) (v4, v6 []net.IPNet) {
	for _, cidr := range selected4 {
		if mapped, ok := MapEgressIPNetToVirtualNAT(cidr, e); ok {
			v4 = append(v4, mapped)
		}
	}
	for _, cidr := range selected6 {
		if mapped, ok := MapEgressIPNetToVirtualNAT(cidr, e); ok {
			v6 = append(v6, mapped)
		}
	}
	return v4, v6
}

// SelectedEgressDstNetsForNode returns restricted-IP dst CIDRs from the node's
// perspective: real LAN on the egress owner, virtual NAT addresses elsewhere.
func SelectedEgressDstNetsForNode(nodeID string, e schema.Egress, selected4, selected6 []net.IPNet) (dst4, dst6 []net.IPNet) {
	if _, ok := e.Nodes[nodeID]; ok || e.VirtualRange == "" {
		return selected4, selected6
	}
	return MapSelectedEgressIPNetsToVirtualNAT(selected4, selected6, e)
}

func appendEgressAllowedCIDRs(cidrs *[]*net.IPNet, e schema.Egress) error {
	if e.Range != "" {
		_, cidr, err := net.ParseCIDR(e.Range)
		if err != nil {
			return errors.New("invalid egress range")
		}
		*cidrs = append(*cidrs, cidr)
	}
	if e.VirtualRange != "" {
		_, cidr, err := net.ParseCIDR(e.VirtualRange)
		if err != nil {
			return errors.New("invalid egress virtual range")
		}
		*cidrs = append(*cidrs, cidr)
	}
	return nil
}

func NormalizeAndValidateAclEgressIPs(acl *models.Acl) error {
	if acl == nil {
		return nil
	}
	egressCIDRs := []*net.IPNet{}
	for _, dst := range acl.Dst {
		if dst.ID != models.EgressID || dst.Value == "*" {
			continue
		}
		e, err := getEgressByID(dst.Value)
		if err != nil {
			return errors.New("invalid egress")
		}
		if err := appendEgressAllowedCIDRs(&egressCIDRs, e); err != nil {
			return err
		}
	}
	for i := range acl.Dst {
		if acl.Dst[i].ID != models.NetmakerIPAclID {
			continue
		}
		if len(egressCIDRs) == 0 {
			return errors.New("egress ip destination requires at least one egress destination")
		}
		normalized, err := NormalizeIPOrCIDR(acl.Dst[i].Value)
		if err != nil {
			return err
		}
		_, cidr, err := net.ParseCIDR(normalized)
		if err != nil {
			return err
		}
		allowed := false
		for _, egressCIDR := range egressCIDRs {
			if cidrContainsCIDR(egressCIDR, cidr) {
				allowed = true
				break
			}
		}
		if !allowed {
			return errors.New("selected ip/cidr " + normalized + " is outside the egress range")
		}
		acl.Dst[i].Value = normalized
	}
	srcEgressCIDRs := []*net.IPNet{}
	for _, src := range acl.Src {
		if src.ID != models.EgressID || src.Value == "*" {
			continue
		}
		e, err := getEgressByID(src.Value)
		if err != nil {
			return errors.New("invalid egress")
		}
		if err := appendEgressAllowedCIDRs(&srcEgressCIDRs, e); err != nil {
			return err
		}
	}
	for i := range acl.Src {
		if acl.Src[i].ID != models.NetmakerIPAclID {
			continue
		}
		if len(srcEgressCIDRs) == 0 {
			return errors.New("egress ip source requires at least one egress source")
		}
		normalized, err := NormalizeIPOrCIDR(acl.Src[i].Value)
		if err != nil {
			return err
		}
		_, cidr, err := net.ParseCIDR(normalized)
		if err != nil {
			return err
		}
		allowed := false
		for _, egressCIDR := range srcEgressCIDRs {
			if cidrContainsCIDR(egressCIDR, cidr) {
				allowed = true
				break
			}
		}
		if !allowed {
			return errors.New("selected ip/cidr " + normalized + " is outside the src egress range")
		}
		acl.Src[i].Value = normalized
	}
	return nil
}

func getSelectedEgressIPNets(dstTags []models.AclPolicyTag) (dst4, dst6 []net.IPNet) {
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
	case models.NetmakerIPAclID:
		_, err := NormalizeIPOrCIDR(t.Value)
		if err != nil {
			return err
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
	if err := NormalizeAndValidateAclEgressIPs(&acl); err != nil {
		return err
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
		defaultPolicy, err := GetDefaultPolicy(schema.NetworkID(node.Network), models.DevicePolicy)
		if err == nil {
			if defaultPolicy.Enabled {
				return true
			}
		}

	}
	// list device policies
	policies := ListDevicePolicies(schema.NetworkID(peer.Network))
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
		if IsEgressRoutingPolicyAllowedForNodes(policy, node, peer) {
			return true
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
	CreateDefaultTags = func(netID schema.NetworkID) {}

	DeleteAllNetworkTags = func(networkID schema.NetworkID) {}

	IsUserAllowedToCommunicate = func(userName string, peer models.Node) (bool, []models.Acl) {
		return false, []models.Acl{}
	}

	RemoveUserFromAclPolicy = func(userName string) {}

	EnsureDefaultUserGroupNetworkPolicies = func(old, new *schema.UserGroup) error {
		return nil
	}

	GetGroupNetworksMap = func(group *schema.UserGroup) (map[schema.NetworkID]schema.Network, error) {
		return nil, nil
	}
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
	defaultPolicy, err := GetDefaultPolicy(schema.NetworkID(node.Network), models.DevicePolicy)
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
	policies := ListDevicePolicies(schema.NetworkID(node.Network))
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
		defaultPolicy, err := GetDefaultPolicy(schema.NetworkID(node.Network), models.DevicePolicy)
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
	policies := ListDevicePolicies(schema.NetworkID(peer.Network))
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
		if IsEgressRoutingPolicyAllowedForNodes(policy, node, peer) {
			allowedPolicies = append(allowedPolicies, policy)
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
func GetDefaultPolicy(netID schema.NetworkID, ruleType models.AclPolicyType) (models.Acl, error) {
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

// ListAcls - lists all acl policies
func ListAclsByNetwork(netID schema.NetworkID) ([]models.Acl, error) {

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
func ListDevicePolicies(netID schema.NetworkID) []models.Acl {
	allAcls := ListAcls()
	deviceAcls := []models.Acl{}
	for _, acl := range allAcls {
		if acl.NetworkID == netID && acl.RuleType == models.DevicePolicy {
			deviceAcls = append(deviceAcls, acl)
		}
	}
	return deviceAcls
}

// ListUserPolicies - lists all user policies in a network
func ListUserPolicies(netID schema.NetworkID) []models.Acl {
	allAcls := ListAcls()
	userAcls := []models.Acl{}
	for _, acl := range allAcls {
		if acl.NetworkID == netID && acl.RuleType == models.UserPolicy {
			userAcls = append(userAcls, acl)
		}
	}
	return userAcls
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
func DeleteNetworkPolicies(netId schema.NetworkID) {
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

// PopulateAclPolicyTagNames resolves human-readable names for ACL policy tags
func PopulateAclPolicyTagNames(acls []models.Acl) {
	for i := range acls {
		populateTagNames(acls[i].Src)
		populateTagNames(acls[i].Dst)
	}
}

func populateTagNames(tags []models.AclPolicyTag) {
	for i := range tags {
		tag := &tags[i]
		if tag.Value == "" || tag.Value == "*" {
			tag.Name = tag.Value
			continue
		}
		switch tag.ID {
		case models.UserAclID:
			tag.Name = tag.Value
		case models.UserGroupAclID:
			grp, err := GetUserGroup(schema.UserGroupID(tag.Value))
			if err == nil {
				tag.Name = grp.Name
			} else {
				tag.Name = tag.Value
			}
		case models.NodeTagID:
			tag.Name = tag.Value
		case models.NodeID:
			node, err := GetNodeByID(tag.Value)
			if err == nil {
				host := &schema.Host{ID: node.HostID}
				if err := host.Get(db.WithContext(context.TODO())); err == nil {
					tag.Name = host.Name
				} else {
					tag.Name = tag.Value
				}
			} else {
				tag.Name = tag.Value
			}
		case models.EgressID:
			egress := schema.Egress{ID: tag.Value}
			if err := egress.Get(db.WithContext(context.TODO())); err == nil {
				tag.Name = egress.Name
			} else {
				tag.Name = tag.Value
			}
		case models.EgressRange:
			tag.Name = tag.Value
		default:
			tag.Name = tag.Value
		}
	}
}

// ValidateCreateAclReq - validates create req for acl
func ValidateCreateAclReq(req models.Acl) error {
	// check if acl network exists
	err := (&schema.Network{Name: req.NetworkID.String()}).Get(db.WithContext(context.TODO()))
	if err != nil {
		return errors.New("failed to get network details for " + req.NetworkID.String())
	}
	// err = CheckIDSyntax(req.Name)
	// if err != nil {
	// 	return err
	// }
	for _, src := range req.Src {
		if src.ID == models.UserGroupAclID {
			userGroup, err := GetUserGroup(schema.UserGroupID(src.Value))
			if err != nil {
				return err
			}

			_, ok := userGroup.NetworkRoles.Data()[schema.AllNetworks]
			if ok {
				continue
			}

			_, ok = userGroup.NetworkRoles.Data()[req.NetworkID]
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
	acls, _ := ListAclsByNetwork(schema.NetworkID(node.Network))
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
func CreateDefaultAclNetworkPolicies(netID schema.NetworkID) {
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

func getTagMapWithNodesByNetwork(netID schema.NetworkID, withStaticNodes bool) (tagNodesMap map[models.TagID][]models.Node) {
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

func addTagMapWithStaticNodes(netID schema.NetworkID,
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
