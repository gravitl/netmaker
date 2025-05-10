package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func ValidateEgressReq(e *schema.Egress) bool {
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
	} else {
		if len(e.Nodes) > 1 {
			return false
		}
		req := models.InetNodeReq{}

		for k := range e.Nodes {
			inetNode, err := GetNodeByID(k)
			if err != nil {
				return false
			}
			// check if node is acting as egress gw already
			GetNodeEgressInfo(&inetNode)
			if err := ValidateInetGwReq(inetNode, req, false); err != nil {
				fmt.Println("====> Failed to Validate Egress: ", err)
				return false
			}

		}

	}
	if len(e.Nodes) != 0 {
		for k := range e.Nodes {
			egressNode, err := GetNodeByID(k)
			if err != nil {
				fmt.Println("hereee   1")
				return false
			}
			GetNodeEgressInfo(&egressNode)
			if egressNode.InternetGwID != "" {
				fmt.Println("hereee   2")
				return false
			}
		}
	}
	return true
}

func GetInetClientsFromAclPolicies(eID string) (inetClientIDs []string) {
	e := schema.Egress{ID: eID}
	err := e.Get(db.WithContext(context.TODO()))
	if err != nil || !e.Status {
		return
	}
	acls, _ := ListAclsByNetwork(models.NetworkID(e.Network))
	for _, acl := range acls {
		for _, dstI := range acl.Dst {
			if dstI.ID == models.EgressID {
				if dstI.Value != eID {
					continue
				}
				for _, srcI := range acl.Src {
					if srcI.Value == "*" {
						continue
					}
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

func isNodeUsingInternetGw(node *models.Node) {
	nodeTags := maps.Clone(node.Tags)
	nodeTags[models.TagID(node.ID.String())] = struct{}{}
	acls, _ := ListAclsByNetwork(models.NetworkID(node.Network))
	for _, acl := range acls {
		if !acl.Enabled {
			continue
		}
		srcVal := convAclTagToValueMap(acl.Src)
		for _, dstI := range acl.Dst {
			if dstI.ID == models.EgressID {
				e := schema.Egress{ID: dstI.Value}
				err := e.Get(db.WithContext(context.TODO()))
				if err != nil || !e.Status {
					continue
				}

				if e.IsInetGw {
					if _, ok := srcVal[node.ID.String()]; ok {
						for nodeID := range e.Nodes {
							if nodeID == node.ID.String() {
								continue
							}
							node.InternetGwID = nodeID
							return
						}
					}
					for tagID := range nodeTags {
						if _, ok := srcVal[tagID.String()]; ok {
							for nodeID := range e.Nodes {
								if nodeID == node.ID.String() {
									continue
								}
								node.InternetGwID = nodeID
								return
							}
						}
					}
				}
			}
		}
	}
}

func AddEgressInfoToNode(targetNode *models.Node, e schema.Egress) {
	req := models.EgressGatewayRequest{
		NodeID: targetNode.ID.String(),
		NetID:  targetNode.Network,
	}
	if metric, ok := e.Nodes[targetNode.ID.String()]; ok {
		if e.IsInetGw {
			targetNode.IsInternetGateway = true
			targetNode.InetNodeReq = models.InetNodeReq{
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
	if e.Nat {
		req.NatEnabled = "yes"
	}
	targetNode.EgressDetails.IsEgressGateway = true
	targetNode.EgressDetails.EgressGatewayRanges = req.Ranges
	targetNode.EgressDetails.EgressGatewayRequest = req
}

func GetNodeEgressInfo(targetNode *models.Node) {
	if targetNode.Mutex != nil {
		targetNode.Mutex.Lock()
		defer targetNode.Mutex.Unlock()
	}
	eli, _ := (&schema.Egress{Network: targetNode.Network}).ListByNetwork(db.WithContext(context.TODO()))
	req := models.EgressGatewayRequest{
		NodeID: targetNode.ID.String(),
		NetID:  targetNode.Network,
	}
	defer func() {
		isNodeUsingInternetGw(targetNode)
	}()
	for _, e := range eli {
		if !e.Status || e.Network != targetNode.Network {
			continue
		}
		if metric, ok := e.Nodes[targetNode.ID.String()]; ok {
			if e.IsInetGw {
				targetNode.IsInternetGateway = true
				targetNode.InetNodeReq = models.InetNodeReq{
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
	if len(req.Ranges) > 0 {
		targetNode.EgressDetails.IsEgressGateway = true
		targetNode.EgressDetails.EgressGatewayRanges = req.Ranges
		targetNode.EgressDetails.EgressGatewayRequest = req
		targetHost, _ := GetHost(targetNode.HostID.String())
		fmt.Println("TARGET NODE: ", targetHost.Name, targetNode.EgressDetails.EgressGatewayRanges, targetNode.EgressDetails.EgressGatewayRequest)
	}
}

func RemoveNodeFromEgress(node models.Node) {
	egs, _ := (&schema.Egress{}).ListByNetwork(db.WithContext(context.TODO()))
	for _, egI := range egs {
		if _, ok := egI.Nodes[node.ID.String()]; ok {
			delete(egI.Nodes, node.ID.String())
			egI.Update(db.WithContext(context.TODO()))
		}
	}

}
