package logic

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/wireguard"
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

// CollectServerMetrics - collects metrics for given server node
func CollectServerMetrics(serverID string, networkNodes []models.Node) *models.Metrics {
	newServerMetrics := models.Metrics{}
	newServerMetrics.Connectivity = make(map[string]models.Metric)
	var serverNode models.Node
	for i := range networkNodes {
		currNodeID := networkNodes[i].ID
		if currNodeID == serverID {
			serverNode = networkNodes[i]
			continue
		}
		if currMetrics, err := GetMetrics(currNodeID); err == nil {
			if currMetrics.Connectivity != nil && currMetrics.Connectivity[serverID].Connected {
				metrics := currMetrics.Connectivity[serverID]
				metrics.NodeName = networkNodes[i].Name
				metrics.IsServer = "no"
				newServerMetrics.Connectivity[currNodeID] = metrics
			}
		} else {
			newServerMetrics.Connectivity[currNodeID] = models.Metric{
				Connected: false,
				Latency:   999,
			}
		}
	}

	if serverNode.IsIngressGateway == "yes" {
		clients, err := GetExtClientsByID(serverID, serverNode.Network)
		if err == nil {
			peers, err := wireguard.GetDevicePeers(serverNode.Interface)
			if err == nil {
				for i := range clients {
					for j := range peers {
						if clients[i].PublicKey == peers[j].PublicKey.String() {
							if peers[j].ReceiveBytes > 0 &&
								peers[j].TransmitBytes > 0 {
								newServerMetrics.Connectivity[clients[i].ClientID] = models.Metric{
									NodeName:      clients[i].ClientID,
									TotalTime:     5,
									Uptime:        5,
									IsServer:      "no",
									TotalReceived: peers[j].ReceiveBytes,
									TotalSent:     peers[j].TransmitBytes,
									Connected:     true,
									Latency:       -1, // can not determine latency on server currently
								}
							} else {
								newServerMetrics.Connectivity[clients[i].ClientID] = models.Metric{
									NodeName:  clients[i].ClientID,
									TotalTime: 5,
									Uptime:    0,
									IsServer:  "no",
									Connected: false,
									Latency:   999,
								}
							}
						}
					}
				}
			}
		}
	}

	return &newServerMetrics
}
