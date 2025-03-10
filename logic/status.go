package logic

import (
	"time"

	"github.com/gravitl/netmaker/models"
)

var GetNodeStatus = GetNodeCheckInStatus

func GetNodeCheckInStatus(node *models.Node, t bool) {
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
