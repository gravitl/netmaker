package logic

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func getNodeStatusOld(node *models.Node) {
	// On CE check only last check-in time
	if node.IsStatic {
		if !node.StaticNode.Enabled {
			node.Status = models.OfflineSt
			return
		}
		node.Status = models.OnlineSt
		return
	}
	if !node.Connected {
		node.Status = models.Disconnected
		return
	}
	if time.Since(node.LastCheckIn) > time.Minute*10 {
		node.Status = models.OfflineSt
		return
	}
	node.Status = models.OnlineSt
}

func GetNodeStatus(node *models.Node, defaultEnabledPolicy bool) {

	if node.IsStatic {
		if !node.StaticNode.Enabled {
			node.Status = models.OfflineSt
			return
		}
		ingNode, err := logic.GetNodeByID(node.StaticNode.IngressGatewayID)
		if err != nil {
			node.Status = models.OfflineSt
			return
		}
		if !defaultEnabledPolicy {
			allowed, _ := logic.IsNodeAllowedToCommunicate(*node, ingNode, false)
			if !allowed {
				node.Status = models.OnlineSt
				return
			}
		}
		// check extclient connection from metrics
		ingressMetrics, err := GetMetrics(node.StaticNode.IngressGatewayID)
		if err != nil || ingressMetrics == nil || ingressMetrics.Connectivity == nil {
			node.Status = models.UnKnown
			return
		}

		if metric, ok := ingressMetrics.Connectivity[node.StaticNode.ClientID]; ok {
			if metric.Connected {
				node.Status = models.OnlineSt
				return
			} else {
				node.Status = models.OfflineSt
				return
			}
		}

		node.Status = models.UnKnown
		return
	}
	if !node.Connected {
		node.Status = models.Disconnected
		return
	}
	if time.Since(node.LastCheckIn) > models.LastCheckInThreshold {
		node.Status = models.OfflineSt
		return
	}
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		node.Status = models.UnKnown
		return
	}
	vlt, err := logic.VersionLessThan(host.Version, "v0.30.0")
	if err != nil {
		node.Status = models.UnKnown
		return
	}
	if vlt {
		getNodeStatusOld(node)
		return
	}
	metrics, err := logic.GetMetrics(node.ID.String())
	if err != nil {
		return
	}
	if metrics == nil || metrics.Connectivity == nil || len(metrics.Connectivity) == 0 {
		if time.Since(node.LastCheckIn) < models.LastCheckInThreshold {
			node.Status = models.OnlineSt
			return
		}
		if node.LastCheckIn.IsZero() {
			node.Status = models.OfflineSt
			return
		}
	}
	// if node.IsFailOver {
	// 	if time.Since(node.LastCheckIn) < models.LastCheckInThreshold {
	// 		node.Status = models.OnlineSt
	// 		return
	// 	}
	// }
	// If all Peers are able to reach me and and the peer is not able to reached by any peer then return online
	/* 1. FailOver Exists
		a. check connectivity to failover Node - if no connection return warning
		b. if getting failedover and still no connection to any of the peers - then show error
		c. if getting failedOver and has connections to some peers - show warning
	2. FailOver Doesn't Exist
		a. check connectivity to pu

	*/

	// failoverNode, exists := FailOverExists(node.Network)
	// if exists && failoverNode.FailedOverBy != uuid.Nil {
	// 	// check connectivity to failover Node
	// 	if metric, ok := metrics.Connectivity[failoverNode.ID.String()]; ok {
	// 		if time.Since(failoverNode.LastCheckIn) < models.LastCheckInThreshold {
	// 			if metric.Connected {
	// 				node.Status = models.OnlineSt
	// 				return
	// 			} else {
	// 				checkPeerConnectivity(node, metrics)
	// 				return
	// 			}
	// 		}
	// 	} else {
	// 		node.Status = models.OnlineSt
	// 		return
	// 	}

	// }
	checkPeerConnectivity(node, metrics, defaultEnabledPolicy)

}

// aclCheckContext holds pre-computed data for efficient ACL checks
type aclCheckContext struct {
	node      models.Node // minimal node representation for ACL checks
	nodeTags  map[models.TagID]struct{}
	policies  []models.Acl
	networkID models.NetworkID
}

// filterPoliciesForNode filters policies to only those that could apply to this node
// A policy is relevant if the node matches its Src tags (for node->peer) or Dst tags (for peer->node if bidirectional)
func filterPoliciesForNode(policies []models.Acl, nodeID string, nodeTags map[models.TagID]struct{}) []models.Acl {
	if len(policies) == 0 {
		return policies
	}

	filtered := make([]models.Acl, 0, len(policies))
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}

		// Convert policy tags to maps for quick lookup
		srcMap := logic.ConvAclTagToValueMap(policy.Src)
		dstMap := logic.ConvAclTagToValueMap(policy.Dst)

		// Check if policy has wildcard - if so, it applies to all nodes
		_, srcAll := srcMap["*"]
		_, dstAll := dstMap["*"]

		// Check if node matches Src (for node->peer direction)
		nodeMatchesSrc := srcAll
		if !nodeMatchesSrc {
			// Check if node ID matches
			if _, ok := srcMap[nodeID]; ok {
				nodeMatchesSrc = true
			} else {
				// Check if any node tag matches
				for tagID := range nodeTags {
					if _, ok := srcMap[tagID.String()]; ok {
						nodeMatchesSrc = true
						break
					}
				}
			}
		}

		// For bidirectional policies, also check if node matches Dst (for peer->node direction)
		nodeMatchesDst := false
		if policy.AllowedDirection == models.TrafficDirectionBi {
			nodeMatchesDst = dstAll
			if !nodeMatchesDst {
				// Check if node ID matches
				if _, ok := dstMap[nodeID]; ok {
					nodeMatchesDst = true
				} else {
					// Check if any node tag matches
					for tagID := range nodeTags {
						if _, ok := dstMap[tagID.String()]; ok {
							nodeMatchesDst = true
							break
						}
					}
				}
			}
		}

		// Include policy if node matches Src or (for bidirectional) Dst
		if nodeMatchesSrc || nodeMatchesDst {
			filtered = append(filtered, policy)
		}
	}

	return filtered
}

// buildACLCheckContext pre-computes node tags and fetches policies once
func buildACLCheckContext(node *models.Node) *aclCheckContext {
	// Create a minimal node representation for ACL checks
	// Convert static node if needed (same as IsPeerAllowed does)
	checkNode := *node
	if node.IsStatic {
		checkNode = node.StaticNode.ConvertToStaticNode()
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

	var nodeID string
	if node.IsStatic {
		nodeID = node.StaticNode.ClientID
	} else {
		nodeID = node.ID.String()
	}
	nodeTags[models.TagID(nodeID)] = struct{}{}
	if node.IsGw {
		nodeTags[models.TagID(fmt.Sprintf("%s.%s", node.Network, models.GwTagName))] = struct{}{}
	}

	// Fetch all policies and filter to only those relevant to this node
	allPolicies := logic.ListDevicePolicies(models.NetworkID(node.Network))
	filteredPolicies := filterPoliciesForNode(allPolicies, nodeID, nodeTags)

	return &aclCheckContext{
		node:      checkNode,
		nodeTags:  nodeTags,
		policies:  filteredPolicies,
		networkID: models.NetworkID(node.Network),
	}
}

// isNodeAllowedToPeerFast performs an optimized ACL check using pre-fetched data
func isNodeAllowedToPeerFast(ctx *aclCheckContext, peer models.Node) bool {
	var peerID string
	if peer.IsStatic {
		peerID = peer.StaticNode.ClientID
	} else {
		peerID = peer.ID.String()
	}

	var peerTags map[models.TagID]struct{}
	if peer.Mutex != nil {
		peer.Mutex.Lock()
		peerTags = maps.Clone(peer.Tags)
		peer.Mutex.Unlock()
	} else {
		peerTags = maps.Clone(peer.Tags)
	}
	if peerTags == nil {
		peerTags = make(map[models.TagID]struct{})
	}
	peerTags[models.TagID(peerID)] = struct{}{}
	if peer.IsGw {
		peerTags[models.TagID(fmt.Sprintf("%s.%s", peer.Network, models.GwTagName))] = struct{}{}
	}

	srcMap := make(map[string]struct{})
	dstMap := make(map[string]struct{})
	defer func() {
		srcMap = nil
		dstMap = nil
	}()

	for _, policy := range ctx.policies {
		if !policy.Enabled {
			continue
		}

		srcMap = logic.ConvAclTagToValueMap(policy.Src)
		dstMap = logic.ConvAclTagToValueMap(policy.Dst)
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

		// Convert peer if static (same as IsPeerAllowed does)
		checkPeer := peer
		if peer.IsStatic {
			checkPeer = peer.StaticNode.ConvertToStaticNode()
		}

		if logic.CheckTagGroupPolicy(srcMap, dstMap, ctx.node, checkPeer, ctx.nodeTags, peerTags) {
			return true
		}
	}
	return false
}

func checkPeerStatus(node *models.Node, defaultAclPolicy bool) {
	peerNotConnectedCnt := 0
	metrics, err := logic.GetMetrics(node.ID.String())
	if err != nil {
		return
	}
	if metrics == nil || metrics.Connectivity == nil {
		if time.Since(node.LastCheckIn) < models.LastCheckInThreshold {
			node.Status = models.OnlineSt
			return
		}
	}

	// Pre-build ACL context if needed
	var aclCtx *aclCheckContext
	if !defaultAclPolicy {
		aclCtx = buildACLCheckContext(node)
	}

	for peerID, metric := range metrics.Connectivity {
		peer, err := logic.GetNodeByID(peerID)
		if err != nil {
			continue
		}

		if !defaultAclPolicy {
			if !isNodeAllowedToPeerFast(aclCtx, peer) {
				continue
			}
		}

		if time.Since(peer.LastCheckIn) > models.LastCheckInThreshold {
			continue
		}
		if metric.Connected {
			continue
		}
		if peer.Status == models.ErrorSt {
			continue
		}
		peerNotConnectedCnt++

	}
	if peerNotConnectedCnt == 0 {
		node.Status = models.OnlineSt
		return
	}
	if len(metrics.Connectivity) > 0 && peerNotConnectedCnt == len(metrics.Connectivity) {
		node.Status = models.ErrorSt
		return
	}
	node.Status = models.WarningSt
}

func checkPeerConnectivity(node *models.Node, metrics *models.Metrics, defaultAclPolicy bool) {
	peerNotConnectedCnt := 0

	// Pre-build ACL context if needed - fetch policies once for all peers
	var aclCtx *aclCheckContext
	if !defaultAclPolicy {
		aclCtx = buildACLCheckContext(node)
	}

	for peerID, metric := range metrics.Connectivity {
		peer, err := logic.GetNodeByID(peerID)
		if err != nil {
			continue
		}

		if !defaultAclPolicy {
			if !isNodeAllowedToPeerFast(aclCtx, peer) {
				continue
			}
		}

		if time.Since(peer.LastCheckIn) > models.LastCheckInThreshold {
			continue
		}
		if metric.Connected {
			continue
		}
		// check if peer is in error state
		checkPeerStatus(&peer, defaultAclPolicy)
		if peer.Status == models.ErrorSt || peer.Status == models.WarningSt {
			continue
		}
		peerNotConnectedCnt++

	}

	if peerNotConnectedCnt > len(metrics.Connectivity)/2 {
		node.Status = models.WarningSt
		return
	}

	if len(metrics.Connectivity) > 0 && peerNotConnectedCnt == len(metrics.Connectivity) {
		node.Status = models.ErrorSt
		return
	}

	node.Status = models.OnlineSt

}
