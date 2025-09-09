package logic

import (
	"context"
	"encoding/json"
	"errors"
	"maps"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
)

func ValidateEgressReq(e *schema.Egress) error {
	if e.Network == "" {
		return errors.New("network id is empty")
	}
	_, err := GetNetwork(e.Network)
	if err != nil {
		return errors.New("failed to get network " + err.Error())
	}

	if !servercfg.IsPro && len(e.Nodes) > 1 {
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
				req.Ranges = append(req.Ranges, e.Range)
			} else {
				req.Ranges = append(req.Ranges, e.DomainAns...)
			}

			if e.Range != "" {
				req.Ranges = append(req.Ranges, e.Range)
				req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
					Network:     e.Range,
					Nat:         e.Nat,
					RouteMetric: m,
				})
			}
			if e.Domain != "" && len(e.DomainAns) > 0 {
				req.Ranges = append(req.Ranges, e.DomainAns...)
				for _, domainAnsI := range e.DomainAns {
					req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
						Network:     domainAnsI,
						Nat:         e.Nat,
						RouteMetric: m,
					})
				}

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

func GetEgressDomainsByAccess(node *models.Node) (domains []string) {
	acls, _ := ListAclsByNetwork(models.NetworkID(node.Network))
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
			domains = append(domains, e.Domain)
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
				req.Ranges = append(req.Ranges, e.Range)
				req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
					Network:     e.Range,
					Nat:         e.Nat,
					RouteMetric: m,
				})
			}
			if e.Domain != "" && len(e.DomainAns) > 0 {
				req.Ranges = append(req.Ranges, e.DomainAns...)
				for _, domainAnsI := range e.DomainAns {
					req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
						Network:     domainAnsI,
						Nat:         e.Nat,
						RouteMetric: m,
					})
				}

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
	egs, _ := (&schema.Egress{
		Network: node.Network,
	}).ListByNetwork(db.WithContext(context.TODO()))
	for _, egI := range egs {
		if _, ok := egI.Nodes[node.ID.String()]; ok {
			delete(egI.Nodes, node.ID.String())
			egI.Update(db.WithContext(context.TODO()))
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

func ListAllByRoutingNodeWithDomain(ctx context.Context, egs []schema.Egress, nodeID string) (egWithDomain []models.EgressDomain) {
	for _, egI := range egs {
		if egI.Domain == "" {
			continue
		}
		if _, ok := egI.Nodes[nodeID]; ok {
			node, err := GetNodeByID(nodeID)
			if err != nil {
				continue
			}
			host, err := GetHost(node.HostID.String())
			if err != nil {
				continue
			}
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
