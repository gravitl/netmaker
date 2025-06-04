package logic

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func ValidateEgressReq(e *schema.Egress) error {
	if e.Network == "" {
		return errors.New("network id is empty")
	}
	_, err := GetNetwork(e.Network)
	if err != nil {
		return errors.New("failed to get network " + err.Error())
	}
	if !e.IsInetGw {
		if e.Range == "" {
			return errors.New("egress range is empty")
		}
		_, _, err = net.ParseCIDR(e.Range)
		if err != nil {
			return errors.New("invalid egress range " + err.Error())
		}
		err = ValidateEgressRange(e.Network, []string{e.Range})
		if err != nil {
			return errors.New("invalid egress range " + err.Error())
		}
	} else {
		if len(e.Nodes) > 1 {
			return errors.New("can only set one internet routing node")
		}
		req := models.InetNodeReq{}
		eli, _ := (&schema.Egress{Network: e.Network}).ListByNetwork(db.WithContext(context.TODO()))
		for k := range e.Nodes {
			inetNode, err := GetNodeByID(k)
			if err != nil {
				return errors.New("invalid routing node " + err.Error())
			}
			// check if node is acting as egress gw already

			GetNodeEgressInfo(&inetNode, eli)
			if err := ValidateInetGwReq(inetNode, req, false); err != nil {
				return err
			}

		}

	}
	if len(e.Nodes) != 0 {
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
	if !e.IsInetGw {
		nodeTags[models.TagID("*")] = struct{}{}
	}

	if !e.IsInetGw {
		defaultDevicePolicy, _ := GetDefaultPolicy(models.NetworkID(node.Network), models.DevicePolicy)
		if defaultDevicePolicy.Enabled {
			return true
		}
	}
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcVal := ConvAclTagToValueMap(acl.Src)
		if !e.IsInetGw && acl.AllowedDirection == models.TrafficDirectionBi {
			if _, ok := srcVal["*"]; ok {
				return true
			}
		}
		for _, dstI := range acl.Dst {

			if !e.IsInetGw && dstI.ID == models.NodeTagID && dstI.Value == "*" {
				return true
			}
			if dstI.ID == models.EgressID && dstI.Value == e.ID {
				e := schema.Egress{ID: dstI.Value}
				err := e.Get(db.WithContext(context.TODO()))
				if err != nil {
					continue
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
		NodeID: targetNode.ID.String(),
		NetID:  targetNode.Network,
	}
	defer func() {
		if targetNode.Mutex != nil {
			targetNode.Mutex.Lock()
		}
		IsNodeUsingInternetGw(targetNode)
		if targetNode.Mutex != nil {
			targetNode.Mutex.Unlock()
		}
	}()

	for _, e := range eli {
		if !e.Status || e.Network != targetNode.Network {
			continue
		}
		if !isDefaultPolicyActive && !e.IsInetGw {
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
			if e.IsInetGw {
				targetNode.EgressDetails.IsInternetGateway = true
				targetNode.EgressDetails.InetNodeReq = models.InetNodeReq{
					InetNodeClientIDs: GetInetClientsFromAclPolicies(e.ID),
				}
				req.Ranges = append(req.Ranges, "0.0.0.0/0")
				req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
					Network:     "0.0.0.0/0",
					Nat:         true,
					RouteMetric: 256,
				})
				req.Ranges = append(req.Ranges, "::/0")
				req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
					Network:     "::/0",
					Nat:         true,
					RouteMetric: 256,
				})
			} else {
				m64, err := metric.(json.Number).Int64()
				if err != nil {
					m64 = 256
				}
				m := uint32(m64)
				req.Ranges = append(req.Ranges, e.Range)
				req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
					Network:     e.Range,
					Nat:         e.Nat,
					RouteMetric: m,
				})
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

// TODO
func GetNetworkEgressInfo(network models.NetworkID) (egressNodes map[string]models.Node) {
	eli, _ := (&schema.Egress{Network: network.String()}).ListByNetwork(db.WithContext(context.TODO()))
	egressNodes = make(map[string]models.Node)
	var err error
	for _, e := range eli {
		if !e.Status || e.Nodes == nil {
			continue
		}

		for nodeID, metric := range e.Nodes {

			targetNode, ok := egressNodes[nodeID]
			if !ok {
				targetNode, err = GetNodeByID(nodeID)
				if err != nil {
					continue
				}
			}
			req := models.EgressGatewayRequest{
				NodeID: targetNode.ID.String(),
				NetID:  targetNode.Network,
			}
			IsNodeUsingInternetGw(&targetNode)
			if e.IsInetGw {
				targetNode.EgressDetails.IsInternetGateway = true
				targetNode.EgressDetails.InetNodeReq = models.InetNodeReq{
					InetNodeClientIDs: GetInetClientsFromAclPolicies(e.ID),
				}
				req.Ranges = append(req.Ranges, "0.0.0.0/0")
				req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
					Network:     "0.0.0.0/0",
					Nat:         true,
					RouteMetric: 256,
				})
				req.Ranges = append(req.Ranges, "::/0")
				req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
					Network:     "::/0",
					Nat:         true,
					RouteMetric: 256,
				})
			} else {
				m64, err := metric.(json.Number).Int64()
				if err != nil {
					m64 = 256
				}
				m := uint32(m64)
				req.Ranges = append(req.Ranges, e.Range)
				req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
					Network:     e.Range,
					Nat:         e.Nat,
					RouteMetric: m,
				})
			}
			if targetNode.Mutex != nil {
				targetNode.Mutex.Lock()
			}
			if len(req.Ranges) > 0 {
				targetNode.EgressDetails.IsEgressGateway = true
				targetNode.EgressDetails.EgressGatewayRanges = append(targetNode.EgressDetails.EgressGatewayRanges, req.Ranges...)
				targetNode.EgressDetails.EgressGatewayRequest.Ranges = append(targetNode.EgressDetails.EgressGatewayRequest.Ranges, req.Ranges...)
				targetNode.EgressDetails.EgressGatewayRequest.RangesWithMetric = append(targetNode.EgressDetails.EgressGatewayRequest.RangesWithMetric,
					req.RangesWithMetric...)
				targetNode.EgressDetails.EgressGatewayRequest = req
				egressNodes[targetNode.ID.String()] = targetNode
			}
			if targetNode.Mutex != nil {
				targetNode.Mutex.Unlock()
			}

		}

	}
	return
}

func GetNodeEgressInfo(targetNode *models.Node, eli []schema.Egress) {

	req := models.EgressGatewayRequest{
		NodeID: targetNode.ID.String(),
		NetID:  targetNode.Network,
	}
	defer func() {
		if targetNode.Mutex != nil {
			targetNode.Mutex.Lock()
		}
		IsNodeUsingInternetGw(targetNode)
		if targetNode.Mutex != nil {
			targetNode.Mutex.Unlock()
		}
	}()
	for _, e := range eli {
		if !e.Status || e.Network != targetNode.Network {
			continue
		}
		if metric, ok := e.Nodes[targetNode.ID.String()]; ok {
			if e.IsInetGw {
				targetNode.EgressDetails.IsInternetGateway = true
				targetNode.EgressDetails.InetNodeReq = models.InetNodeReq{
					InetNodeClientIDs: GetInetClientsFromAclPolicies(e.ID),
				}
				req.Ranges = append(req.Ranges, "0.0.0.0/0")
				req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
					Network:     "0.0.0.0/0",
					Nat:         true,
					RouteMetric: 256,
				})
				req.Ranges = append(req.Ranges, "::/0")
				req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
					Network:     "::/0",
					Nat:         true,
					RouteMetric: 256,
				})
			} else {
				m64, err := metric.(json.Number).Int64()
				if err != nil {
					m64 = 256
				}
				m := uint32(m64)
				req.Ranges = append(req.Ranges, e.Range)
				req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
					Network:     e.Range,
					Nat:         e.Nat,
					RouteMetric: m,
				})
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
