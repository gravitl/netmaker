package logic

import (
	"time"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func GetNodeStatus(node *models.Node) {
	if time.Since(node.LastCheckIn) > models.LastCheckInThreshold {
		node.Status = models.OfflineSt
		return
	}
	if node.IsStatic {
		if !node.StaticNode.Enabled {
			node.Status = models.OfflineSt
			return
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
	checkPeerConnectivity(node, metrics)

}

func checkPeerStatus(node *models.Node) {
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
	for peerID, metric := range metrics.Connectivity {
		peer, err := logic.GetNodeByID(peerID)
		if err != nil {
			continue
		}
		if !logic.IsNodeAllowedToCommunicate(*node, peer) {
			continue
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
	if peerNotConnectedCnt == len(metrics.Connectivity) {
		node.Status = models.ErrorSt
		return
	}
	node.Status = models.WarningSt
}

func checkPeerConnectivity(node *models.Node, metrics *models.Metrics) {
	peerNotConnectedCnt := 0
	for peerID, metric := range metrics.Connectivity {
		peer, err := logic.GetNodeByID(peerID)
		if err != nil {
			continue
		}
		if !logic.IsNodeAllowedToCommunicate(*node, peer) {
			continue
		}

		if time.Since(peer.LastCheckIn) > models.LastCheckInThreshold {
			continue
		}
		if metric.Connected {
			continue
		}
		// check if peer is in error state
		checkPeerStatus(&peer)
		if peer.Status == models.ErrorSt {
			continue
		}
		peerNotConnectedCnt++

	}
	if peerNotConnectedCnt == 0 {
		node.Status = models.OnlineSt
		return
	}
	if peerNotConnectedCnt == len(metrics.Connectivity) {
		node.Status = models.ErrorSt
		return
	}
	node.Status = models.WarningSt
}
