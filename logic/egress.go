package logic

import (
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

func GetNodeEgressInfo(targetNode *models.Node) {
	eli, _ := (&models.Egress{}).ListByNetwork()
	req := models.EgressGatewayRequest{
		NodeID: targetNode.ID.String(),
		NetID:  targetNode.Network,
	}
	for _, e := range eli {
		if metric, ok := e.Nodes[targetNode.ID.String()]; ok {
			req.Ranges = append(req.Ranges, e.Range)
			req.RangesWithMetric = append(req.RangesWithMetric, models.EgressRangeMetric{
				Network:     e.Range,
				Nat:         e.Nat,
				RouteMetric: metric.(uint32),
			})
		}
	}
	if len(req.Ranges) > 0 {
		targetNode.IsEgressGateway = true
		targetNode.EgressGatewayRanges = req.Ranges
		targetNode.EgressGatewayRequest = req
	}
}
