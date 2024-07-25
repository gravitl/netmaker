package logic

import (
	"encoding/json"
	"math"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

var (
	metricsCacheMutex = &sync.RWMutex{}
	metricsCacheMap   map[string]models.Metrics
)

func getMetricsFromCache(key string) (metrics models.Metrics, ok bool) {
	metricsCacheMutex.RLock()
	metrics, ok = metricsCacheMap[key]
	metricsCacheMutex.RUnlock()
	return
}

func storeMetricsInCache(key string, metrics models.Metrics) {
	metricsCacheMutex.Lock()
	metricsCacheMap[key] = metrics
	metricsCacheMutex.Unlock()
}

func deleteNetworkFromCache(key string) {
	metricsCacheMutex.Lock()
	delete(metricsCacheMap, key)
	metricsCacheMutex.Unlock()
}

func LoadNodeMetricsToCache() error {
	point1 := time.Now()
	if metricsCacheMap == nil {
		metricsCacheMap = map[string]models.Metrics{}
	}

	collection, err := database.FetchRecords(database.METRICS_TABLE_NAME)
	if err != nil {
		return err
	}

	for key, value := range collection {
		var metrics models.Metrics
		if err := json.Unmarshal([]byte(value), &metrics); err != nil {
			slog.Error("parse metric record error", "error", err.Error())
			continue
		}
		if servercfg.CacheEnabled() {
			storeMetricsInCache(key, metrics)
		}
	}

	point3 := time.Now()
	slog.Error("load node metrics done", "Debug", point3.Unix()-point1.Unix(), len(metricsCacheMap))
	return nil
}

// GetMetrics - gets the metrics
func GetMetrics(nodeid string) (*models.Metrics, error) {
	var metrics models.Metrics
	if servercfg.CacheEnabled() {
		if metrics, ok := getMetricsFromCache(nodeid); ok {
			return &metrics, nil
		}
	}
	record, err := database.FetchRecord(database.METRICS_TABLE_NAME, nodeid)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return &metrics, nil
		}
		return &metrics, err
	}
	err = json.Unmarshal([]byte(record), &metrics)
	if err != nil {
		return &metrics, err
	}
	if servercfg.CacheEnabled() {
		storeMetricsInCache(nodeid, metrics)
	}
	return &metrics, nil
}

// UpdateMetrics - updates the metrics of a given client
func UpdateMetrics(nodeid string, metrics *models.Metrics) error {
	data, err := json.Marshal(metrics)
	if err != nil {
		return err
	}
	err = database.Insert(nodeid, string(data), database.METRICS_TABLE_NAME)
	if err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		storeMetricsInCache(nodeid, *metrics)
	}
	return nil
}

// DeleteMetrics - deletes metrics of a given node
func DeleteMetrics(nodeid string) error {
	err := database.DeleteRecord(database.METRICS_TABLE_NAME, nodeid)
	if err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		deleteNetworkFromCache(nodeid)
	}
	return nil
}

// MQUpdateMetricsFallBack - called when mq fallback thread is triggered on client
func MQUpdateMetricsFallBack(nodeid string, newMetrics models.Metrics) {

	currentNode, err := logic.GetNodeByID(nodeid)
	if err != nil {
		slog.Error("error getting node", "id", nodeid, "error", err)
		return
	}

	updateNodeMetrics(&currentNode, &newMetrics)
	if err = logic.UpdateMetrics(nodeid, &newMetrics); err != nil {
		slog.Error("failed to update node metrics", "id", nodeid, "error", err)
		return
	}
	if servercfg.IsMetricsExporter() {
		if err := mq.PushMetricsToExporter(newMetrics); err != nil {
			slog.Error("failed to push node metrics to exporter", "id", currentNode.ID, "error", err)
		}
	}
	slog.Debug("updated node metrics", "id", nodeid)
}

func MQUpdateMetrics(client mqtt.Client, msg mqtt.Message) {
	id, err := mq.GetID(msg.Topic())
	if err != nil {
		slog.Error("error getting ID sent on ", "topic", msg.Topic(), "error", err)
		return
	}
	currentNode, err := logic.GetNodeByID(id)
	if err != nil {
		slog.Error("error getting node", "id", id, "error", err)
		return
	}
	decrypted, decryptErr := mq.DecryptMsg(&currentNode, msg.Payload())
	if decryptErr != nil {
		slog.Error("failed to decrypt message for node", "id", id, "error", decryptErr)
		return
	}

	var newMetrics models.Metrics
	if err := json.Unmarshal(decrypted, &newMetrics); err != nil {
		slog.Error("error unmarshaling payload", "error", err)
		return
	}
	updateNodeMetrics(&currentNode, &newMetrics)
	if err = logic.UpdateMetrics(id, &newMetrics); err != nil {
		slog.Error("failed to update node metrics", "id", id, "error", err)
		return
	}
	if servercfg.IsMetricsExporter() {
		if err := mq.PushMetricsToExporter(newMetrics); err != nil {
			slog.Error("failed to push node metrics to exporter", "id", currentNode.ID, "error", err)
		}
	}
	slog.Debug("updated node metrics", "id", id)
}

func updateNodeMetrics(currentNode *models.Node, newMetrics *models.Metrics) {
	oldMetrics, err := logic.GetMetrics(currentNode.ID.String())
	if err != nil {
		slog.Error("error finding old metrics for node", "id", currentNode.ID, "error", err)
		return
	}

	var attachedClients []models.ExtClient
	if currentNode.IsIngressGateway {
		clients, err := logic.GetExtClientsByID(currentNode.ID.String(), currentNode.Network)
		if err == nil {
			attachedClients = clients
		}
	}
	if newMetrics.Connectivity == nil {
		newMetrics.Connectivity = make(map[string]models.Metric)
	}
	for i := range attachedClients {
		slog.Debug("[metrics] processing attached client", "client", attachedClients[i].ClientID, "public key", attachedClients[i].PublicKey)
		clientMetric := newMetrics.Connectivity[attachedClients[i].PublicKey]
		clientMetric.NodeName = attachedClients[i].ClientID
		newMetrics.Connectivity[attachedClients[i].ClientID] = clientMetric
		delete(newMetrics.Connectivity, attachedClients[i].PublicKey)
		slog.Debug("[metrics] attached client metric", "metric", clientMetric)
	}

	// run through metrics for each peer
	for k := range newMetrics.Connectivity {
		currMetric := newMetrics.Connectivity[k]
		oldMetric := oldMetrics.Connectivity[k]
		currMetric.TotalTime += oldMetric.TotalTime
		currMetric.Uptime += oldMetric.Uptime // get the total uptime for this connection

		if currMetric.TotalReceived < oldMetric.TotalReceived {
			currMetric.TotalReceived += oldMetric.TotalReceived
		} else {
			currMetric.TotalReceived += int64(math.Abs(float64(currMetric.TotalReceived) - float64(oldMetric.TotalReceived)))
		}
		if currMetric.TotalSent < oldMetric.TotalSent {
			currMetric.TotalSent += oldMetric.TotalSent
		} else {
			currMetric.TotalSent += int64(math.Abs(float64(currMetric.TotalSent) - float64(oldMetric.TotalSent)))
		}

		if currMetric.Uptime == 0 || currMetric.TotalTime == 0 {
			currMetric.PercentUp = 0
		} else {
			currMetric.PercentUp = 100.0 * (float64(currMetric.Uptime) / float64(currMetric.TotalTime))
		}
		totalUpMinutes := currMetric.Uptime * ncutils.CheckInInterval
		currMetric.ActualUptime = time.Duration(totalUpMinutes) * time.Minute
		delete(oldMetrics.Connectivity, k) // remove from old data
		newMetrics.Connectivity[k] = currMetric

	}

	slog.Debug("[metrics] node metrics data", "node ID", currentNode.ID, "metrics", newMetrics)
}
