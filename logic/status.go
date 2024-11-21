package logic

import (
	"time"

	"github.com/gravitl/netmaker/models"
)

func GetNodeStatus(node *models.Node) {
	// On CE check only last check-in time
	if time.Since(node.LastCheckIn) > time.Minute*10 {
		node.Status = models.OfflineSt
		return
	}
	node.Status = models.OnlineSt
}
