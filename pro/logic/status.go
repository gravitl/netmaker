package logic

import (
	"time"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func GetNodeStatus(node *models.Node) {
	// On CE check only last check-in time
	if time.Since(node.LastCheckIn) > models.LastCheckInThreshold {
		node.Status = models.OfflineSt
		return
	}
	metrics, err := logic.GetMetrics(node.ID.String())
	if err != nil {
		return
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
