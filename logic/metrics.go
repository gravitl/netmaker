package logic

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
)

type MetricsMonitor struct {
	cancel context.CancelFunc
}

var metricsMonitor MetricsMonitor

func GetMetricsMonitor() *MetricsMonitor {
	return &metricsMonitor
}

func (m *MetricsMonitor) Start() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}

	var ctx context.Context
	ctx, m.cancel = context.WithCancel(context.Background())

	go func(ctx context.Context) {
		scopedCtx := DefaultScope(db.WithContext(ctx))
		metricsInterval, _ := strconv.Atoi(GetServerSettings(scopedCtx).MetricInterval)
		if metricsInterval == 0 {
			return
		}

		checkInterval := time.Duration(2*metricsInterval) * time.Minute
		for {
			select {
			case <-time.After(checkInterval):
				nodes, _ := GetAllNodes(scopedCtx)
				for _, node := range nodes {
					if node.Connected || node.PendingDelete {
						continue
					}

					nodeMetrics, err := GetMetrics(scopedCtx, node.ID.String())
					if err == nil {
						inc := math.Round(float64(time.Since(nodeMetrics.UpdatedAt)) / float64(time.Minute))
						for peer, peerMetrics := range nodeMetrics.Connectivity {
							peerMetrics.TotalTime += int64(inc)
							peerMetrics.PercentUp = 100.0 * (float64(peerMetrics.Uptime) / float64(peerMetrics.TotalTime))
							nodeMetrics.Connectivity[peer] = peerMetrics
						}

						_ = UpdateMetrics(scopedCtx, node.ID.String(), nodeMetrics)
					}
					go SetPeerMetricsDisconnected(scopedCtx, node.ID.String())
				}
			case <-ctx.Done():
				return
			}
		}
	}(ctx)
}

func (m *MetricsMonitor) Stop() {
	m.cancel()
	m.cancel = nil
}

var DeleteMetrics = func(context.Context, string) error {
	return nil
}

var UpdateMetrics = func(context.Context, string, *models.Metrics) error {
	return nil
}

var GetMetrics = func(context.Context, string) (*models.Metrics, error) {
	var metrics models.Metrics
	return &metrics, nil
}

var DeleteNodeMetricsFromPeers = func(context.Context, string) {}

var SetPeerMetricsDisconnected = func(context.Context, string) {}

// TriggerCollectMetrics - asks the client to push metrics now. Overridden in Pro.
// reason is a short label (e.g. "join", "reconnect", "checkin_recovered") used for logging.
var TriggerCollectMetrics = func(hostID, nodeID, reason string) {}

var LoadMetricsIntoCache = func(ctx context.Context) error {
	return nil
}
