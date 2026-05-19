package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
	"gorm.io/datatypes"
)

var ValidateEgressReq = validateEgressReq

var AssignVirtualRangeToEgress = func(nw *schema.Network, eg *schema.Egress) error {
	return nil
}

func validateEgressReq(e *schema.Egress) error {
	if e.Network == "" {
		return errors.New("network id is empty")
	}
	if e.Nat {
		e.Mode = schema.DirectNAT
	} else {
		e.Mode = ""
		e.VirtualRange = ""
	}
	err := (&schema.Network{Name: e.Network}).Get(db.WithContext(context.TODO()))
	if err != nil {
		return errors.New("failed to get network " + err.Error())
	}

	if !GetFeatureFlags().EnableEgressHA && len(e.Nodes) > 1 {
		return errors.New("can only set one routing node on CE")
	}

	if len(e.Nodes) > 0 {
		for k := range e.Nodes {
			_, err := GetNodeByID(k)
			if err != nil {
				return errors.New("invalid routing node " + err.Error())
			}
		}
	}
	return nil
}

// NormalizeEgressReqDomains validates each domain entry (FQDN or *.suffix),
// lowercases, and deduplicates while preserving input order.
func NormalizeEgressReqDomains(domains []string) ([]string, error) {
	seen := make(map[string]struct{})
	var out []string
	add := func(s string) error {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			return nil
		}
		if !IsEgressDomainPattern(s) {
			return fmt.Errorf("invalid egress domain: %q", s)
		}
		if _, ok := seen[s]; ok {
			return nil
		}
		seen[s] = struct{}{}
		out = append(out, s)
		return nil
	}
	for _, d := range domains {
		if err := add(d); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ConfiguredDomainsForEgress returns configured logical domain names from Domains.
func ConfiguredDomainsForEgress(e schema.Egress) []string {
	if len(e.Domains) == 0 {
		return nil
	}
	out := make([]string, len(e.Domains))
	copy(out, e.Domains)
	return out
}

// ApplyConfiguredDomainsToEgress sets Domains on the egress record.
func ApplyConfiguredDomainsToEgress(e *schema.Egress, domains []string) {
	e.Domains = datatypes.JSONSlice[string](domains)
}

// IsDomainBasedEgress is true when this egress has at least one configured logical domain.
func IsDomainBasedEgress(e schema.Egress) bool {
	return len(ConfiguredDomainsForEgress(e)) > 0
}

// EgressDomainsEqual compares two domain lists as sets (order-independent).
func EgressDomainsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := slices.Clone(a)
	bb := slices.Clone(b)
	slices.Sort(aa)
	slices.Sort(bb)
	return slices.Equal(aa, bb)
}

func DoesUserHaveAccessToEgress(user *schema.User, e *schema.Egress, acls []models.Acl) bool {
	if !e.Status {
		return false
	}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		dstTags := ConvAclTagToValueMap(acl.Dst)
		_, all := dstTags["*"]

		if _, ok := dstTags[e.ID]; ok || all {
			// get all src tags
			for _, srcAcl := range acl.Src {
				if srcAcl.ID == models.UserAclID && srcAcl.Value == user.Username {
					return true
				} else if srcAcl.ID == models.UserGroupAclID {
					// fetch all users in the group
					if _, ok := user.UserGroups.Data()[schema.UserGroupID(srcAcl.Value)]; ok {
						return true
					}
				}
			}
		}
	}
	return false
}

func DoesNodeHaveAccessToEgress(node *models.Node, e *schema.Egress, acls []models.Acl) bool {
	nodeTags := maps.Clone(node.Tags)
	nodeTags[models.TagID(node.ID.String())] = struct{}{}
	nodeTags[models.TagID("*")] = struct{}{}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcVal := ConvAclTagToValueMap(acl.Src)
		for _, dstI := range acl.Dst {
			if (dstI.ID == models.EgressID && dstI.Value == e.ID) || (dstI.ID == models.NodeTagID && dstI.Value == "*") {
				if dstI.ID == models.EgressID {
					e := schema.Egress{ID: dstI.Value}
					err := e.Get(db.WithContext(context.TODO()))
					if err != nil {
						continue
					}
				}

				if node.IsStatic {
					if _, ok := srcVal[node.StaticNode.ClientID]; ok {
						return true
					}
				} else {
					if _, ok := srcVal[node.ID.String()]; ok {
						return true
					}
				}

				for tagID := range nodeTags {
					if _, ok := srcVal[tagID.String()]; ok {
						return true
					}
				}
			}

		}
	}
	return false
}

func doesNodeHaveAccessToEgressByRoutingPolicy(node, targetNode *models.Node, e *schema.Egress, acls []models.Acl) bool {
	if node == nil || targetNode == nil || e == nil {
		return false
	}
	if _, ok := e.Nodes[targetNode.ID.String()]; !ok {
		return false
	}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		if !IsEgressRoutingPolicyAllowedForNodes(acl, *node, *targetNode) {
			continue
		}
		srcEgresses := getEgressesFromPolicyTags(acl.Src, node.Network)
		dstEgresses := getEgressesFromPolicyTags(acl.Dst, node.Network)
		nodeRoutesSrc := targetNodeRoutesAnyEgress(*node, srcEgresses)
		nodeRoutesDst := targetNodeRoutesAnyEgress(*node, dstEgresses)
		targetRoutesSrc := targetNodeRoutesAnyEgress(*targetNode, srcEgresses)
		targetRoutesDst := targetNodeRoutesAnyEgress(*targetNode, dstEgresses)
		if acl.AllowedDirection == models.TrafficDirectionUni {
			if nodeRoutesSrc && targetRoutesDst && egressListContainsID(dstEgresses, e.ID) {
				return true
			}
			continue
		}
		if nodeRoutesSrc && targetRoutesDst && egressListContainsID(dstEgresses, e.ID) {
			return true
		}
		if nodeRoutesDst && targetRoutesSrc && egressListContainsID(srcEgresses, e.ID) {
			return true
		}
	}
	return false
}

func egressListContainsID(egresses []schema.Egress, id string) bool {
	for _, e := range egresses {
		if e.ID == id {
			return true
		}
	}
	return false
}

// snapshotNodeTagIDs copies tag keys from n.Tags. When n.Mutex is set, reads are serialized
// with writers on the same node (shallow copies may share the Tags map). When Mutex is nil,
// tags are still read so tag-based egress matching applies; that matches patterns like
// maps.Clone(node.Tags) elsewhere for nodes without an initialized mutex.
func snapshotNodeTagIDs(n *models.Node) []models.TagID {
	if n == nil {
		return nil
	}
	if n.Mutex != nil {
		n.Mutex.Lock()
		defer n.Mutex.Unlock()
	}
	if len(n.Tags) == 0 {
		return nil
	}
	out := make([]models.TagID, 0, len(n.Tags))
	for tid := range n.Tags {
		out = append(out, tid)
	}
	return out
}

func AddEgressInfoToPeerByAccess(node, targetNode *models.Node, eli []schema.Egress, acls []models.Acl, isDefaultPolicyActive bool) {

	req := models.EgressGatewayRequest{
		NodeID:     targetNode.ID.String(),
		NetID:      targetNode.Network,
		NatEnabled: "yes",
	}
	nodeTagIDs := snapshotNodeTagIDs(targetNode)
	for _, e := range eli {
		if !e.Status || e.Network != targetNode.Network {
			continue
		}
		if !isDefaultPolicyActive {
			if !DoesNodeHaveAccessToEgress(node, &e, acls) &&
				!doesNodeHaveAccessToEgressByRoutingPolicy(node, targetNode, &e, acls) {
				if node.IsRelayed && node.RelayedBy == targetNode.ID.String() {
					if !DoesNodeHaveAccessToEgress(targetNode, &e, acls) {
						continue
					}
				} else {
					continue
				}

			}
		}

		if metric, ok := e.Nodes[targetNode.ID.String()]; ok {
			m64, err := metric.(json.Number).Int64()
			if err != nil {
				m64 = 256
			}
			m := uint32(m64)
			if e.Range != "" {
				// Use virtual NAT range if enabled, otherwise use original range
				egressRange := e.Range
				if e.Nat && e.VirtualRange != "" {
					egressRange = e.VirtualRange
				}
				req.Ranges = append(req.Ranges, egressRange)
			} else {
				req.Ranges = append(req.Ranges, e.DomainAns...)
			}

			if e.Range != "" {
				req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
					EgressID:       e.ID,
					EgressName:     e.Name,
					Network:        e.Range,
					VirtualNetwork: e.VirtualRange,
					Nat:            e.Nat,
					Mode:           e.Mode,
					RouteMetric:    m,
				})
			}
			if IsDomainBasedEgress(e) && len(e.DomainAns) > 0 {
				req.Ranges = append(req.Ranges, e.DomainAns...)
				for _, domainAnsI := range e.DomainAns {
					req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
						EgressID:       e.ID,
						EgressName:     e.Name,
						Network:        domainAnsI,
						VirtualNetwork: e.VirtualRange,
						Nat:            e.Nat,
						Mode:           e.Mode,
						RouteMetric:    m,
					})
				}

			}
		}
		for _, tagID := range nodeTagIDs {
			if metric, ok := e.Tags[tagID.String()]; ok {
				m64, err := metric.(json.Number).Int64()
				if err != nil {
					m64 = 256
				}
				m := uint32(m64)
				if e.Range != "" {
					// Use virtual NAT range if enabled, otherwise use original range
					egressRange := e.Range
					if e.Nat && e.VirtualRange != "" {
						egressRange = e.VirtualRange
					}
					req.Ranges = append(req.Ranges, egressRange)
				} else {
					req.Ranges = append(req.Ranges, e.DomainAns...)
				}

				if e.Range != "" {
					req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
						EgressID:       e.ID,
						EgressName:     e.Name,
						Network:        e.Range,
						VirtualNetwork: e.VirtualRange,
						Nat:            e.Nat,
						Mode:           e.Mode,
						RouteMetric:    m,
					})
				}
				if IsDomainBasedEgress(e) && len(e.DomainAns) > 0 {
					req.Ranges = append(req.Ranges, e.DomainAns...)
					for _, domainAnsI := range e.DomainAns {
						req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
							EgressID:       e.ID,
							EgressName:     e.Name,
							Network:        domainAnsI,
							VirtualNetwork: e.VirtualRange,
							Nat:            e.Nat,
							Mode:           e.Mode,
							RouteMetric:    m,
						})
					}

				}
				break
			}
		}

	}
	if targetNode.Mutex != nil {
		targetNode.Mutex.Lock()
	}
	if len(req.Ranges) > 0 {

		targetNode.EgressDetails.IsEgressGateway = true
		targetNode.EgressDetails.EgressGatewayRanges = req.Ranges
		targetNode.EgressDetails.EgressGatewayRequest = req

	} else {
		targetNode.EgressDetails = models.EgressDetails{}
	}
	if targetNode.Mutex != nil {
		targetNode.Mutex.Unlock()
	}
}

func GetEgressDomainsByAccessForUser(user *schema.User, network schema.NetworkID) (domains []string) {
	acls := ListUserPolicies(network)
	eli, _ := (&schema.Egress{Network: network.String()}).ListByNetwork(db.WithContext(context.TODO()))
	defaultDevicePolicy, _ := GetDefaultPolicy(network, models.UserPolicy)
	isDefaultPolicyActive := defaultDevicePolicy.Enabled
	seen := make(map[string]struct{})
	for _, e := range eli {
		if !e.Status || e.Network != network.String() {
			continue
		}
		if !isDefaultPolicyActive {
			if !DoesUserHaveAccessToEgress(user, &e, acls) {
				continue
			}
		}
		if IsDomainBasedEgress(e) && len(e.DomainAns) > 0 {
			for _, d := range ConfiguredDomainsForEgress(e) {
				bd := BaseDomain(d)
				if _, ok := seen[bd]; ok {
					continue
				}
				seen[bd] = struct{}{}
				domains = append(domains, bd)
			}

		}
	}
	return
}

func GetEgressDomainNSForNode(node *models.Node) (returnNsLi []models.Nameserver) {
	acls := ListDevicePolicies(schema.NetworkID(node.Network))
	eli, _ := (&schema.Egress{Network: node.Network}).ListByNetwork(db.WithContext(context.TODO()))
	defaultDevicePolicy, _ := GetDefaultPolicy(schema.NetworkID(node.Network), models.DevicePolicy)
	isDefaultPolicyActive := defaultDevicePolicy.Enabled
	for _, e := range eli {
		if !e.Status || e.Network != node.Network {
			continue
		}
		if !isDefaultPolicyActive {
			if !DoesNodeHaveAccessToEgress(node, &e, acls) {
				continue
			}
		}
		if IsDomainBasedEgress(e) && len(e.DomainAns) > 0 {
			var routingNodeIPs []string
			// Collect IPs from all routing nodes for this egress
			for nodeID := range e.Nodes {
				routingNode, err := GetNodeByID(nodeID)
				if err != nil {
					continue
				}
				if routingNode.ID == node.ID {
					continue
				}
				if routingNode.Address.IP != nil {
					routingNodeIPs = append(routingNodeIPs, routingNode.Address.IP.String())
				}
				if routingNode.Address6.IP != nil {
					routingNodeIPs = append(routingNodeIPs, routingNode.Address6.IP.String())
				}
			}
			for _, d := range ConfiguredDomainsForEgress(e) {
				returnNsLi = append(returnNsLi, models.Nameserver{
					IPs:            routingNodeIPs,
					MatchDomain:    BaseDomain(d),
					IsSearchDomain: false,
				})
			}

		}
	}
	return
}

func GetNodeEgressInfo(targetNode *models.Node, eli []schema.Egress, acls []models.Acl) {

	req := models.EgressGatewayRequest{
		NodeID:     targetNode.ID.String(),
		NetID:      targetNode.Network,
		NatEnabled: "yes",
	}
	nodeTagIDs := snapshotNodeTagIDs(targetNode)
	for _, e := range eli {
		if !e.Status || e.Network != targetNode.Network {
			continue
		}
		if metric, ok := e.Nodes[targetNode.ID.String()]; ok {
			m64, err := metric.(json.Number).Int64()
			if err != nil {
				m64 = 256
			}
			m := uint32(m64)
			if e.Range != "" {
				// Use virtual NAT range if enabled, otherwise use original range
				egressRange := e.Range
				if e.Nat && e.VirtualRange != "" {
					egressRange = e.VirtualRange
				}
				req.Ranges = append(req.Ranges, egressRange)
				req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
					EgressID:       e.ID,
					EgressName:     e.Name,
					Network:        e.Range,
					VirtualNetwork: e.VirtualRange,
					Nat:            e.Nat,
					Mode:           e.Mode,
					RouteMetric:    m,
				})
			}
			if IsDomainBasedEgress(e) && len(e.DomainAns) > 0 {
				req.Ranges = append(req.Ranges, e.DomainAns...)
				for _, domainAnsI := range e.DomainAns {
					req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
						EgressID:       e.ID,
						EgressName:     e.Name,
						Network:        domainAnsI,
						VirtualNetwork: e.VirtualRange,
						Nat:            e.Nat,
						Mode:           e.Mode,
						RouteMetric:    m,
					})
				}

			}

		}
		for _, tagID := range nodeTagIDs {
			if metric, ok := e.Tags[tagID.String()]; ok {
				m64, err := metric.(json.Number).Int64()
				if err != nil {
					m64 = 256
				}
				m := uint32(m64)
				if e.Range != "" {
					// Use virtual NAT range if enabled, otherwise use original range
					egressRange := e.Range
					if e.Nat && e.VirtualRange != "" {
						egressRange = e.VirtualRange
					}
					req.Ranges = append(req.Ranges, egressRange)
				} else {
					req.Ranges = append(req.Ranges, e.DomainAns...)
				}

				if e.Range != "" {
					req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
						EgressID:       e.ID,
						EgressName:     e.Name,
						Network:        e.Range,
						VirtualNetwork: e.VirtualRange,
						Nat:            e.Nat,
						Mode:           e.Mode,
						RouteMetric:    m,
					})
				}
				if IsDomainBasedEgress(e) && len(e.DomainAns) > 0 {
					req.Ranges = append(req.Ranges, e.DomainAns...)
					for _, domainAnsI := range e.DomainAns {
						req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
							EgressID:       e.ID,
							EgressName:     e.Name,
							Network:        domainAnsI,
							Nat:            e.Nat,
							Mode:           e.Mode,
							VirtualNetwork: e.VirtualRange,
							RouteMetric:    m,
						})
					}

				}
				break
			}
		}
	}
	if targetNode.Mutex != nil {
		targetNode.Mutex.Lock()
	}
	if len(req.Ranges) > 0 {
		targetNode.EgressDetails.IsEgressGateway = true
		targetNode.EgressDetails.EgressGatewayRanges = req.Ranges
		targetNode.EgressDetails.EgressGatewayRequest = req
	} else {
		targetNode.EgressDetails = models.EgressDetails{}
	}
	if targetNode.Mutex != nil {
		targetNode.Mutex.Unlock()
	}
}

func RemoveNodeFromEgress(node models.Node) {
	ctx := db.WithContext(context.TODO())
	egs, err := (&schema.Egress{Network: node.Network}).ListByNetwork(ctx)
	if err != nil {
		slog.Error("RemoveNodeFromEgress: failed to list egresses", "error", err.Error())
		return
	}
	for i := range egs {
		egI := &egs[i]
		if _, ok := egI.Nodes[node.ID.String()]; ok {
			delete(egI.Nodes, node.ID.String())
			// Build a new map to ensure GORM persists the change; in-place modification
			// of the same map reference may not be detected by Updates(&struct).
			newNodes := make(datatypes.JSONMap)
			for k, v := range egI.Nodes {
				newNodes[k] = v
			}
			if err := db.FromContext(ctx).Table(egI.Table()).Where("id = ?", egI.ID).Updates(map[string]any{
				"nodes": newNodes,
			}).Error; err != nil {
				slog.Error("RemoveNodeFromEgress: failed to update egress", "id", egI.ID, "error", err.Error())
			}
		}
	}
}

func RemoveNodeFromEnrollmentKeys(node *models.Node) {
	keys, _ := GetAllEnrollmentKeys()
	for _, key := range keys {
		if key.Relay == node.ID {
			key.Relay = uuid.Nil
			_ = upsertEnrollmentKey(&key)
		}
	}
}

func GetEgressRanges(netID schema.NetworkID) (map[string][]string, map[string]struct{}, error) {

	resultMap := make(map[string]struct{})
	nodeEgressMap := make(map[string][]string)
	networkNodes, err := GetNetworkNodes(netID.String())
	if err != nil {
		return nil, nil, err
	}
	for _, currentNode := range networkNodes {
		if currentNode.Network != netID.String() {
			continue
		}
		if currentNode.EgressDetails.IsEgressGateway { // add the egress gateway range(s) to the result
			if len(currentNode.EgressDetails.EgressGatewayRanges) > 0 {
				nodeEgressMap[currentNode.ID.String()] = currentNode.EgressDetails.EgressGatewayRanges
				for _, egressRangeI := range currentNode.EgressDetails.EgressGatewayRanges {
					resultMap[egressRangeI] = struct{}{}
				}
			}
		}
	}
	extclients, _ := GetNetworkExtClients(netID.String())
	for _, extclient := range extclients {
		if len(extclient.ExtraAllowedIPs) > 0 {
			nodeEgressMap[extclient.ClientID] = extclient.ExtraAllowedIPs
			for _, extraAllowedIP := range extclient.ExtraAllowedIPs {
				resultMap[extraAllowedIP] = struct{}{}
			}
		}
	}
	return nodeEgressMap, resultMap, nil
}

func ListAllByRoutingNodeWithDomain(egs []schema.Egress, nodeID string) (egWithDomain []models.EgressDomain) {
	node, err := GetNodeByID(nodeID)
	if err != nil {
		return
	}
	host := &schema.Host{
		ID: node.HostID,
	}
	err = host.Get(db.WithContext(context.TODO()))
	if err != nil {
		return
	}
	for _, egI := range egs {
		if !egI.Status || !IsDomainBasedEgress(egI) {
			continue
		}
		if _, ok := egI.Nodes[nodeID]; ok {
			for _, d := range ConfiguredDomainsForEgress(egI) {
				egWithDomain = append(egWithDomain, models.EgressDomain{
					ID:     egI.ID,
					Domain: d,
					Node:   node,
					Host:   *host,
				})
			}

		}
	}
	return
}

func BaseDomain(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return host // not a FQDN
	}
	return strings.Join(parts[len(parts)-2:], ".")
}
