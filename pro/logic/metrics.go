package logic

import (
	"encoding/json"
	"math"
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

// GetMetrics - gets the metrics
func GetMetrics(nodeid string) (*models.Metrics, error) {
	var metrics models.Metrics
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
	return &metrics, nil
}

// UpdateMetrics - updates the metrics of a given client
func UpdateMetrics(nodeid string, metrics *models.Metrics) error {
	data, err := json.Marshal(metrics)
	if err != nil {
		return err
	}
	return database.Insert(nodeid, string(data), database.METRICS_TABLE_NAME)
}

// DeleteMetrics - deletes metrics of a given node
func DeleteMetrics(nodeid string) error {
	return database.DeleteRecord(database.METRICS_TABLE_NAME, nodeid)
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

	shouldUpdate := updateNodeMetrics(&currentNode, &newMetrics)

	if err = logic.UpdateMetrics(id, &newMetrics); err != nil {
		slog.Error("failed to update node metrics", "id", id, "error", err)
		return
	}
	if servercfg.IsMetricsExporter() {
		if err := mq.PushMetricsToExporter(newMetrics); err != nil {
			slog.Error("failed to push node metrics to exporter", "id", currentNode.ID, "error", err)
		}
	}

	if shouldUpdate {
		slog.Info("updating peers after node detected connectivity issues", "id", currentNode.ID, "network", currentNode.Network)
		host, err := logic.GetHost(currentNode.HostID.String())
		if err == nil {
			nodes, err := logic.GetAllNodes()
			if err != nil {
				return
			}
			if err = mq.PublishSingleHostPeerUpdate(host, nodes, nil, nil, false); err != nil {
				slog.Warn("failed to publish update after failover peer change for node", "id", currentNode.ID, "network", currentNode.Network, "error", err)
			}
		}
	}
	slog.Debug("updated node metrics", "id", id)
}

func updateNodeMetrics(currentNode *models.Node, newMetrics *models.Metrics) bool {
	if newMetrics.FailoverPeers == nil {
		newMetrics.FailoverPeers = make(map[string]string)
	}
	oldMetrics, err := logic.GetMetrics(currentNode.ID.String())
	if err != nil {
		slog.Error("error finding old metrics for node", "id", currentNode.ID, "error", err)
		return false
	}
	if oldMetrics.FailoverPeers == nil {
		oldMetrics.FailoverPeers = make(map[string]string)
	}

	var attachedClients []models.ExtClient
	if currentNode.IsIngressGateway {
		clients, err := logic.GetExtClientsByID(currentNode.ID.String(), currentNode.Network)
		if err == nil {
			attachedClients = clients
		}
	}
	if len(attachedClients) > 0 {
		// associate ext clients with IDs
		for i := range attachedClients {
			extMetric := newMetrics.Connectivity[attachedClients[i].PublicKey]
			if len(extMetric.NodeName) == 0 &&
				len(newMetrics.Connectivity[attachedClients[i].ClientID].NodeName) > 0 { // cover server clients
				extMetric = newMetrics.Connectivity[attachedClients[i].ClientID]
				if extMetric.TotalReceived > 0 && extMetric.TotalSent > 0 {
					extMetric.Connected = true
				}
			}
			extMetric.NodeName = attachedClients[i].ClientID
			delete(newMetrics.Connectivity, attachedClients[i].PublicKey)
			newMetrics.Connectivity[attachedClients[i].ClientID] = extMetric
		}
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

	shouldUpdate := len(oldMetrics.FailoverPeers) == 0 && len(newMetrics.FailoverPeers) > 0
	for k, v := range oldMetrics.FailoverPeers {
		if len(newMetrics.FailoverPeers[k]) > 0 && len(v) == 0 {
			shouldUpdate = true
		}

		if len(v) > 0 && len(newMetrics.FailoverPeers[k]) == 0 {
			newMetrics.FailoverPeers[k] = v
		}
	}

	for k := range oldMetrics.Connectivity { // cleanup any left over data, self healing
		delete(newMetrics.Connectivity, k)
	}
	return shouldUpdate
}
