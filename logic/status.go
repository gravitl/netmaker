package logic

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
)

// StaleStatusCheckInterval - how often MarkStaleNodesOffline scans the nodes
// table looking for nodes whose last check-in is older than
// models.LastCheckInThreshold.
const StaleStatusCheckInterval = 5 * time.Minute

var GetNodeStatus = getNodeCheckInStatus

func getNodeCheckInStatus(node *models.Node, t bool) {
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
	if time.Since(node.LastCheckIn) > models.LastCheckInThreshold {
		node.Status = schema.OfflineSt
		return
	}
	node.Status = schema.OnlineSt
}

func GetNodeCheckInStatus(node *schema.Node) schema.NodeStatus {
	if !node.Connected {
		return schema.Disconnected
	}
	if time.Since(node.LastCheckIn) > models.LastCheckInThreshold {
		return schema.OfflineSt
	}
	return schema.OnlineSt
}

// MarkStaleNodesOffline runs on a ticker and bulk-updates the status of every
// node whose last_check_in is older than models.LastCheckInThreshold to
// schema.OfflineSt. Promotion back to OnlineSt happens in
// mq.HandleHostCheckin when a node recovers; the metrics-driven path in
// pro/logic refines the status further on the next metrics message.
//
// Intended to be started once from the master pod.
func MarkStaleNodesOffline(ctx context.Context) {
	ticker := time.NewTicker(StaleStatusCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			threshold := time.Now().UTC().Add(-models.LastCheckInThreshold)
			n, err := (&schema.Node{}).MarkStaleOffline(db.WithContext(ctx), threshold)
			if err != nil {
				slog.Error("mark stale nodes offline failed", "error", err)
				continue
			}
			if n > 0 {
				slog.Info("marked stale nodes offline", "count", n, "threshold", threshold)
			}
		}
	}
}
