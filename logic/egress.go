package logic

import (
	"encoding/json"
	"net"

	"github.com/gravitl/netmaker/models"
)

func ValidateEgressReq(e *models.Egress) bool {
	if e.Network == "" {
		return false
	}
	_, err := GetNetwork(e.Network)
	if err != nil {
		return false
	}
	if !e.IsInetGw {
		if e.Range == "" {
			return false
		}
		_, _, err = net.ParseCIDR(e.Range)
		if err != nil {
			return false
		}
		err = ValidateEgressRange(e.Network, []string{e.Range})
		if err != nil {
			return false
		}
	}
	if len(e.Nodes) != 0 {
		for k := range e.Nodes {
			_, err := GetNodeByID(k)
			if err != nil {
				return false
			}
		}
	}
	return true
}

func GetInetClientsFromAclPolicies(node *models.Node) (inetClientIDs []string) {
	acls, _ := ListAclsByNetwork(models.NetworkID(node.Network))
	for _, acl := range acls {
		dstVal := convAclTagToValueMap(acl.Dst)
		for _, dstI := range acl.Dst {
			if _, ok := dstVal[node.ID.String()]; !ok {
				continue
			}
			if dstI.ID == models.EgressRange && dstI.Value == "*" {
				for _, srcI := range acl.Src {
					if srcI.ID == models.NodeID {
						inetClientIDs = append(inetClientIDs, srcI.Value)
					}
					if srcI.ID == models.NodeTagID {
						inetClientIDs = append(inetClientIDs, GetNodeIDsWithTag(models.TagID(srcI.Value))...)
					}
				}
			}
		}
	}
	return
}

func IsNodeUsingInternetGw(node *models.Node) {
	acls, _ := ListAclsByNetwork(models.NetworkID(node.Network))
	for _, acl := range acls {
		srcVal := convAclTagToValueMap(acl.Src)
		for _, dstI := range acl.Dst {
			if dstI.ID == models.EgressRange && dstI.Value == "*" {
				if _, ok := srcVal[node.ID.String()]; ok {
					for _, dstI := range acl.Dst {
						if dstI.ID == models.NodeID {
							node.InternetGwID = dstI.Value
							return
						}
					}
				}
				for tagID := range node.Tags {
					if _, ok := srcVal[tagID.String()]; ok {
						for _, dstI := range acl.Dst {
							if dstI.ID == models.NodeID {
								node.InternetGwID = dstI.Value
								return
							}
						}
					}
				}
			}
		}
	}
}

func GetNodeEgressInfo(targetNode *models.Node) {
	eli, _ := (&models.Egress{Network: targetNode.Network}).ListByNetwork()
	req := models.EgressGatewayRequest{
		NodeID: targetNode.ID.String(),
		NetID:  targetNode.Network,
	}
	for _, e := range eli {
		if metric, ok := e.Nodes[targetNode.ID.String()]; ok {
			if e.IsInetGw {
				targetNode.IsInternetGateway = true
				targetNode.InetNodeReq = models.InetNodeReq{
					InetNodeClientIDs: GetInetClientsFromAclPolicies(targetNode),
				}
			}
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
	if len(req.Ranges) > 0 {
		targetNode.IsEgressGateway = true
		targetNode.EgressGatewayRanges = req.Ranges
		targetNode.EgressGatewayRequest = req
	}
}
