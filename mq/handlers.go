package mq

import (
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
)

// DefaultHandler default message queue handler  -- NOT USED
func DefaultHandler(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "MQTT Message: Topic: ", string(msg.Topic()), " Message: ", string(msg.Payload()))
}

// Ping message Handler -- handles ping topic from client nodes
func Ping(client mqtt.Client, msg mqtt.Message) {
	go func() {
		id, err := getID(msg.Topic())
		if err != nil {
			logger.Log(0, "error getting node.ID sent on ping topic ")
			return
		}
		node, err := logic.GetNodeByID(id)
		if err != nil {
			logger.Log(0, "mq-ping error getting node: ", err.Error())
			record, err := database.FetchRecord(database.NODES_TABLE_NAME, id)
			if err != nil {
				logger.Log(0, "error reading database ", err.Error())
				return
			}
			logger.Log(0, "record from database")
			logger.Log(0, record)
			return
		}
		decrypted, decryptErr := decryptMsg(&node, msg.Payload())
		if decryptErr != nil {
			logger.Log(0, "error decrypting when updating node ", node.ID, decryptErr.Error())
			return
		}
		var checkin models.NodeCheckin
		if err := json.Unmarshal(decrypted, &checkin); err != nil {
			logger.Log(1, "error unmarshaling payload ", err.Error())
			return
		}
		node.SetLastCheckIn()
		node.Version = checkin.Version
		node.Connected = checkin.Connected
		if err := logic.UpdateNode(&node, &node); err != nil {
			logger.Log(0, "error updating node", node.Name, node.ID, " on checkin", err.Error())
			return
		}

		logger.Log(3, "ping processed for node", node.Name, node.ID)
		// --TODO --set client version once feature is implemented.
		//node.SetClientVersion(msg.Payload())
	}()
}

// UpdateNode  message Handler -- handles updates from client nodes
func UpdateNode(client mqtt.Client, msg mqtt.Message) {
	go func() {
		id, err := getID(msg.Topic())
		if err != nil {
			logger.Log(1, "error getting node.ID sent on ", msg.Topic(), err.Error())
			return
		}
		currentNode, err := logic.GetNodeByID(id)
		if err != nil {
			logger.Log(1, "error getting node ", id, err.Error())
			return
		}
		decrypted, decryptErr := decryptMsg(&currentNode, msg.Payload())
		if decryptErr != nil {
			logger.Log(1, "failed to decrypt message for node ", id, decryptErr.Error())
			return
		}
		var newNode models.Node
		if err := json.Unmarshal(decrypted, &newNode); err != nil {
			logger.Log(1, "error unmarshaling payload ", err.Error())
			return
		}
		newNode.SetLastCheckIn()
		if err := logic.UpdateNode(&currentNode, &newNode); err != nil {
			logger.Log(1, "error saving node", err.Error())
			return
		}
		updateNodePeers(&currentNode)
		logger.Log(1, "updated node", id, newNode.Name)
	}()
}

// UpdateMetrics  message Handler -- handles updates from client nodes for metrics
func UpdateMetrics(client mqtt.Client, msg mqtt.Message) {
	if logic.Is_EE {
		go func() {
			id, err := getID(msg.Topic())
			if err != nil {
				logger.Log(1, "error getting node.ID sent on ", msg.Topic(), err.Error())
				return
			}
			currentNode, err := logic.GetNodeByID(id)
			if err != nil {
				logger.Log(1, "error getting node ", id, err.Error())
				return
			}
			decrypted, decryptErr := decryptMsg(&currentNode, msg.Payload())
			if decryptErr != nil {
				logger.Log(1, "failed to decrypt message for node ", id, decryptErr.Error())
				return
			}

			var newMetrics models.Metrics
			if err := json.Unmarshal(decrypted, &newMetrics); err != nil {
				logger.Log(1, "error unmarshaling payload ", err.Error())
				return
			}

			updateNodeMetrics(&currentNode, &newMetrics)

			if err = logic.UpdateMetrics(id, &newMetrics); err != nil {
				logger.Log(1, "faield to update node metrics", id, currentNode.Name, err.Error())
				return
			}
			if servercfg.IsMetricsExporter() {
				if err := pushMetricsToExporter(newMetrics); err != nil {
					logger.Log(2, fmt.Sprintf("failed to push node: [%s] metrics to exporter, err: %v",
						currentNode.Name, err))
				}
			}

			logger.Log(1, "updated node metrics", id, currentNode.Name)
		}()
	}
}

// ClientPeerUpdate  message handler -- handles updating peers after signal from client nodes
func ClientPeerUpdate(client mqtt.Client, msg mqtt.Message) {
	go func() {
		id, err := getID(msg.Topic())
		if err != nil {
			logger.Log(1, "error getting node.ID sent on ", msg.Topic(), err.Error())
			return
		}
		currentNode, err := logic.GetNodeByID(id)
		if err != nil {
			logger.Log(1, "error getting node ", id, err.Error())
			return
		}
		decrypted, decryptErr := decryptMsg(&currentNode, msg.Payload())
		if decryptErr != nil {
			logger.Log(1, "failed to decrypt message during client peer update for node ", id, decryptErr.Error())
			return
		}
		switch decrypted[0] {
		case ncutils.ACK:
			currentServerNode, err := logic.GetNetworkServerLocal(currentNode.Network)
			if err != nil {
				return
			}
			if err := logic.ServerUpdate(&currentServerNode, false); err != nil {
				logger.Log(1, "server node:", currentServerNode.ID, "failed update")
				return
			}
		case ncutils.DONE:
			updateNodePeers(&currentNode)
		}

		logger.Log(1, "sent peer updates after signal received from", id, currentNode.Name)
	}()
}

func updateNodePeers(currentNode *models.Node) {
	currentServerNode, err := logic.GetNetworkServerLocal(currentNode.Network)
	if err != nil {
		logger.Log(1, "failed to get server node failed update\n", err.Error())
		return
	}
	if err := logic.ServerUpdate(&currentServerNode, false); err != nil {
		logger.Log(1, "server node:", currentServerNode.ID, "failed update")
		return
	}
	if logic.IsLeader(&currentServerNode) {
		if err := PublishPeerUpdate(currentNode, false); err != nil {
			logger.Log(1, "error publishing peer update ", err.Error())
			return
		}
	}
}

func updateNodeMetrics(currentNode *models.Node, newMetrics *models.Metrics) {
	oldMetrics, err := logic.GetMetrics(currentNode.ID)
	if err != nil {
		logger.Log(1, "error finding old metrics for node", currentNode.ID, currentNode.Name)
		return
	}

	var attachedClients []models.ExtClient
	if currentNode.IsIngressGateway == "yes" {
		clients, err := logic.GetExtClientsByID(currentNode.ID, currentNode.Network)
		if err == nil {
			attachedClients = clients
		}
	}
	if len(attachedClients) > 0 {
		// associate ext clients with IDs
		for i := range attachedClients {
			extMetric := newMetrics.Connectivity[attachedClients[i].PublicKey]
			delete(newMetrics.Connectivity, attachedClients[i].PublicKey)
			if extMetric.Connected { // add ext client metrics
				newMetrics.Connectivity[attachedClients[i].ClientID] = extMetric
			}
		}
	}

	// run through metrics for each peer
	for k := range newMetrics.Connectivity {
		currMetric := newMetrics.Connectivity[k]
		oldMetric := oldMetrics.Connectivity[k]
		currMetric.TotalTime += oldMetric.TotalTime
		currMetric.Uptime += oldMetric.Uptime // get the total uptime for this connection
		currMetric.PercentUp = 100.0 * (float64(currMetric.Uptime) / float64(currMetric.TotalTime))
		totalUpMinutes := currMetric.Uptime * 5
		currMetric.ActualUptime = time.Duration(totalUpMinutes) * time.Minute
		delete(oldMetrics.Connectivity, k) // remove from old data
		newMetrics.Connectivity[k] = currMetric
	}

	for k := range oldMetrics.Connectivity { // cleanup any left over data, self healing
		delete(newMetrics.Connectivity, k)
	}
}
