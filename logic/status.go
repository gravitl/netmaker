package logic

import (
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

var GetNodeStatus = getNodeCheckInStatus

func getNodeCheckInStatus(node *models.Node, t bool) {
	// On CE check only last check-in time
	if node.IsStatic {
		if !node.StaticNode.Enabled {
			node.Status = schema.OfflineSt
			return
		}
		node.Status = schema.OnlineSt
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

func GetNodeCheckInStatus(node *schema.Node) schema.NodeStatus {
	if !node.Connected {
		return schema.Disconnected
	}
	if time.Since(node.LastCheckIn) > time.Minute*10 {
		return schema.OfflineSt
	}
	return schema.OnlineSt
}
