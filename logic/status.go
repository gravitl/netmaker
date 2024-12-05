package logic

import (
	"time"

	"github.com/gravitl/netmaker/models"
)

var GetNodeStatus = getNodeStatus

func getNodeStatus(node *models.Node) {
	// On CE check only last check-in time
	if node.IsStatic {
		if !node.StaticNode.Enabled {
			node.Status = models.OfflineSt
			return
		}
		node.Status = models.OnlineSt
		return
	}
	if time.Since(node.LastCheckIn) > time.Minute*10 {
		node.Status = models.OfflineSt
		return
	}
	node.Status = models.OnlineSt
}
