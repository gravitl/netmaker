package logic

import (
	"context"
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
			node.Status = schema.OfflineSt
			node.StaticNode.Status = schema.OfflineSt
			return
		}
		node.Status = schema.OnlineSt
		node.StaticNode.Status = schema.OnlineSt
		return
	}
	if !node.Connected {
		node.Status = schema.Disconnected
		return
	}
	if time.Since(node.LastCheckIn) > time.Minute*10 {
		node.Status = schema.OfflineSt
		return
	}
	node.Status = schema.OnlineSt
}

func GetNodeStatus(node *models.Node, defaultEnabledPolicy bool) {

	if node.IsStatic {
		if !node.StaticNode.Enabled {
			node.Status = schema.OfflineSt
			node.StaticNode.Status = schema.OfflineSt
			return
		}
		ingNode, err := logic.GetNodeByID(node.StaticNode.IngressGatewayID)
		if err != nil {
			node.Status = schema.OfflineSt
			node.StaticNode.Status = schema.OfflineSt
			return
		}
		if !defaultEnabledPolicy {
			allowed, _ := logic.IsNodeAllowedToCommunicate(*node, ingNode, false)
			if !allowed {
				node.Status = schema.OnlineSt
				node.StaticNode.Status = schema.OnlineSt
				return
			}
		}
		// check extclient connection from metrics
		ingressMetrics, err := GetMetrics(node.StaticNode.IngressGatewayID)
		if err != nil || ingressMetrics == nil || ingressMetrics.Connectivity == nil {
			node.Status = schema.UnKnown
			node.StaticNode.Status = schema.UnKnown
			return
		}

		if metric, ok := ingressMetrics.Connectivity[node.StaticNode.ClientID]; ok {
			if metric.Connected {
				node.Status = schema.OnlineSt
				node.StaticNode.Status = schema.OnlineSt
				return
			} else {
				node.Status = schema.OfflineSt
				node.StaticNode.Status = schema.OfflineSt
				return
			}
		}

		node.Status = schema.UnKnown
		node.StaticNode.Status = schema.UnKnown
		return
	}
	if !node.Connected {
		node.Status = schema.Disconnected
		return
	}
	if time.Since(node.LastCheckIn) > models.LastCheckInThreshold {
		node.Status = schema.OfflineSt
		return
	}
	host := &schema.Host{
		ID: node.HostID,
	}
	err := host.Get(db.WithContext(context.TODO()))
	if err != nil {
		node.Status = schema.UnKnown
		return
	}
	vlt, err := logic.VersionLessThan(host.Version, "v0.30.0")
	if err != nil {
		node.Status = schema.UnKnown
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
			node.Status = schema.OnlineSt
			return
		}
		if node.LastCheckIn.IsZero() {
			node.Status = schema.OfflineSt
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
	peers := buildPeerCache(node, metrics)
	checkPeerConnectivity(node, metrics, defaultEnabledPolicy, peers)

}

// buildPeerCache fetches every node referenced (directly or transitively via
// the metrics cache one level deep) from the source node's connectivity map in
// a single batched DB query. The returned map keys are node IDs.
//
// This collapses the per-peer GetNodeByID storm that previously dominated
// status computation: with P peers the old path issued O(P^2) preloaded
// First() queries; this path issues exactly one IN-query.
func buildPeerCache(node *models.Node, metrics *models.Metrics) map[string]models.Node {
	if metrics == nil || len(metrics.Connectivity) == 0 {
		return map[string]models.Node{}
	}

	idSet := make(map[string]struct{}, len(metrics.Connectivity)*2)
	for peerID := range metrics.Connectivity {
		idSet[peerID] = struct{}{}
		// checkPeerStatus walks the peer's own connectivity map; pre-collect
		// those IDs too so we can resolve everything in one query. GetMetrics
		// is cache-backed so this loop is cheap.
		peerMetrics, err := logic.GetMetrics(peerID)
		if err != nil || peerMetrics == nil {
			continue
		}
		for ppID := range peerMetrics.Connectivity {
			idSet[ppID] = struct{}{}
		}
	}
	if node != nil {
		delete(idSet, node.ID.String())
	}
	if len(idSet) == 0 {
		return map[string]models.Node{}
	}

	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	peers, err := logic.GetNodesByIDs(ids)
	if err != nil {
		return map[string]models.Node{}
	}
	return peers
}

func CheckPeerStatus(node *models.Node, defaultAclPolicy bool, peers map[string]models.Node) {
	peerNotConnectedCnt := 0
	metrics, err := logic.GetMetrics(node.ID.String())
	if err != nil {
		return
	}
	if metrics == nil || metrics.Connectivity == nil {
		if time.Since(node.LastCheckIn) < models.LastCheckInThreshold {
			node.Status = schema.OnlineSt
			return
		}
	}
	for peerID, metric := range metrics.Connectivity {
		peer, ok := peers[peerID]
		if !ok {
			continue
		}

		if !defaultAclPolicy {
			allowed, _ := logic.IsNodeAllowedToCommunicate(*node, peer, false)
			if !allowed {
				continue
			}
		}

		if time.Since(peer.LastCheckIn) > models.LastCheckInThreshold {
			continue
		}
		if metric.Connected {
			continue
		}
		if peer.Status == schema.ErrorSt {
			continue
		}
		peerNotConnectedCnt++

	}
	if peerNotConnectedCnt == 0 {
		node.Status = schema.OnlineSt
		return
	}
	if len(metrics.Connectivity) > 0 && peerNotConnectedCnt == len(metrics.Connectivity) {
		node.Status = schema.ErrorSt
		return
	}
	node.Status = schema.WarningSt
}

func checkPeerConnectivity(node *models.Node, metrics *models.Metrics, defaultAclPolicy bool, peers map[string]models.Node) {
	peerNotConnectedCnt := 0
	for peerID, metric := range metrics.Connectivity {
		peer, ok := peers[peerID]
		if !ok {
			continue
		}

		if !defaultAclPolicy {
			allowed, _ := logic.IsNodeAllowedToCommunicate(*node, peer, false)
			if !allowed {
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
		// checkPeerStatus(&peer, defaultAclPolicy, peers)
		if peer.Status == schema.ErrorSt || peer.Status == schema.WarningSt {
			continue
		}
		peerNotConnectedCnt++

	}

	if peerNotConnectedCnt > len(metrics.Connectivity)/2 {
		node.Status = schema.WarningSt
		return
	}

	if len(metrics.Connectivity) > 0 && peerNotConnectedCnt == len(metrics.Connectivity) {
		node.Status = schema.ErrorSt
		return
	}

	node.Status = schema.OnlineSt

}
