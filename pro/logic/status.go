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
	peerNotConnectedCnt := 0
	for peerID, metric := range metrics.Connectivity {
		peer, err := logic.GetNodeByID(peerID)
		if err != nil {
			continue
		}
		if time.Since(peer.LastCheckIn) > models.LastCheckInThreshold {
			continue
		}
		if metric.Connected {
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
