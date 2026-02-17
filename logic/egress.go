package logic

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"strings"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
	"gorm.io/datatypes"
)

var ValidateEgressReq = validateEgressReq

var AssignVirtualRangeToEgress = func(nw *models.Network, eg *schema.Egress) error {
	return nil
}

func validateEgressReq(e *schema.Egress) error {
	if e.Network == "" {
		return errors.New("network id is empty")
	}
	if e.Nat {
		e.Mode = models.DirectNAT
	} else {
		e.Mode = ""
		e.VirtualRange = ""
	}
	_, err := GetNetwork(e.Network)
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

func DoesUserHaveAccessToEgress(user *models.User, e *schema.Egress, acls []models.Acl) bool {
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
				if srcAcl.ID == models.UserAclID && srcAcl.Value == user.UserName {
					return true
				} else if srcAcl.ID == models.UserGroupAclID {
					// fetch all users in the group
					if _, ok := user.UserGroups[models.UserGroupID(srcAcl.Value)]; ok {
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

func AddEgressInfoToPeerByAccess(node, targetNode *models.Node, eli []schema.Egress, acls []models.Acl, isDefaultPolicyActive bool) {

	req := models.EgressGatewayRequest{
		NodeID:     targetNode.ID.String(),
		NetID:      targetNode.Network,
		NatEnabled: "yes",
	}
	for _, e := range eli {
		if !e.Status || e.Network != targetNode.Network {
			continue
		}
		if !isDefaultPolicyActive {
			if !DoesNodeHaveAccessToEgress(node, &e, acls) {
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
			if e.Domain != "" && len(e.DomainAns) > 0 {
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
		for tagID := range targetNode.Tags {
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
				if e.Domain != "" && len(e.DomainAns) > 0 {
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

func GetEgressDomainsByAccessForUser(user *models.User, network models.NetworkID) (domains []string) {
	acls := ListUserPolicies(network)
	eli, _ := (&schema.Egress{Network: network.String()}).ListByNetwork(db.WithContext(context.TODO()))
	defaultDevicePolicy, _ := GetDefaultPolicy(network, models.UserPolicy)
	isDefaultPolicyActive := defaultDevicePolicy.Enabled
	for _, e := range eli {
		if !e.Status || e.Network != network.String() {
			continue
		}
		if !isDefaultPolicyActive {
			if !DoesUserHaveAccessToEgress(user, &e, acls) {
				continue
			}
		}
		if e.Domain != "" && len(e.DomainAns) > 0 {
			domains = append(domains, BaseDomain(e.Domain))

		}
	}
	return
}

func GetEgressDomainNSForNode(node *models.Node) (returnNsLi []models.Nameserver) {
	acls := ListDevicePolicies(models.NetworkID(node.Network))
	eli, _ := (&schema.Egress{Network: node.Network}).ListByNetwork(db.WithContext(context.TODO()))
	defaultDevicePolicy, _ := GetDefaultPolicy(models.NetworkID(node.Network), models.DevicePolicy)
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
		if e.Domain != "" && len(e.DomainAns) > 0 {
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
			returnNsLi = append(returnNsLi, models.Nameserver{
				IPs:            routingNodeIPs,
				MatchDomain:    BaseDomain(e.Domain),
				IsSearchDomain: false,
			})

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
			if e.Domain != "" && len(e.DomainAns) > 0 {
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
		for tagID := range targetNode.Tags {
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
				if e.Domain != "" && len(e.DomainAns) > 0 {
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

func GetEgressRanges(netID models.NetworkID) (map[string][]string, map[string]struct{}, error) {

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
	host, err := GetHost(node.HostID.String())
	if err != nil {
		return
	}
	for _, egI := range egs {
		if !egI.Status || egI.Domain == "" {
			continue
		}
		if _, ok := egI.Nodes[nodeID]; ok {

			egWithDomain = append(egWithDomain, models.EgressDomain{
				ID:     egI.ID,
				Domain: egI.Domain,
				Node:   node,
				Host:   *host,
			})

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
