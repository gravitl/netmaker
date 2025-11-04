package logic

import (
	"context"
	"strconv"
	"time"

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
		metricsInterval, _ := strconv.Atoi(GetServerSettings().MetricInterval)
		if metricsInterval == 0 {
			return
		}

		checkInterval := time.Duration(2*metricsInterval) * time.Minute
		for {
			select {
			case <-time.After(checkInterval):
				metrics, _ := GetAllMetrics()
				for _, metric := range metrics {
					if time.Since(metric.UpdatedAt) >= checkInterval {
						_ = DeleteMetrics(metric.NodeID)
					}
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

var DeleteMetrics = func(string) error {
	return nil
}

var UpdateMetrics = func(string, *models.Metrics) error {
	return nil
}

var GetAllMetrics = func() ([]models.Metrics, error) {
	return []models.Metrics{}, nil
}

var GetMetrics = func(string) (*models.Metrics, error) {
	var metrics models.Metrics
	return &metrics, nil
}
