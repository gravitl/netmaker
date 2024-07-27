package logic

import (
	"github.com/gravitl/netmaker/models"
)

var DeleteMetrics = func(string) error {
	return nil
}

var UpdateMetrics = func(string, *models.Metrics) error {
	return nil
}

var WriteMetricsCacheToDB = func() error {
	return nil
}

var GetMetrics = func(string) (*models.Metrics, error) {
	var metrics models.Metrics
	return &metrics, nil
}
