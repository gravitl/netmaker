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
			logger.Log(0, "error decrypting when updating node ", node.ID.String(), decryptErr.Error())
			return
		}
		var checkin models.NodeCheckin
		if err := json.Unmarshal(decrypted, &checkin); err != nil {
			logger.Log(1, "error unmarshaling payload ", err.Error())
			return
		}
		host, err := logic.GetHost(node.HostID.String())
		if err != nil {
			logger.Log(0, "error retrieving host for node ", node.ID.String(), err.Error())
			return
		}
		node.SetLastCheckIn()
		host.Version = checkin.Version
		node.Connected = checkin.Connected
		host.Interfaces = checkin.Ifaces
		for i := range host.Interfaces {
			host.Interfaces[i].AddressString = host.Interfaces[i].Address.String()
		}
		if err := logic.UpdateNode(&node, &node); err != nil {
			logger.Log(0, "error updating node", node.ID.String(), " on checkin", err.Error())
			return
		}

		logger.Log(3, "ping processed for node", node.ID.String())
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

		ifaceDelta := logic.IfaceDelta(&currentNode, &newNode)
		if servercfg.Is_EE && ifaceDelta {
			if err = logic.EnterpriseResetAllPeersFailovers(currentNode.ID.String(), currentNode.Network); err != nil {
				logger.Log(1, "failed to reset failover list during node update", currentNode.ID.String(), currentNode.Network)
			}
		}
		newNode.SetLastCheckIn()
		if err := logic.UpdateNode(&currentNode, &newNode); err != nil {
			logger.Log(1, "error saving node", err.Error())
			return
		}
		if ifaceDelta { // reduce number of unneeded updates, by only sending on iface changes
			if err = PublishPeerUpdate(); err != nil {
				logger.Log(0, "error updating peers when node", currentNode.ID.String(), "informed the server of an interface change", err.Error())
			}
		}

		logger.Log(1, "updated node", id, newNode.ID.String())

	}()
}

// UpdateMetrics  message Handler -- handles updates from client nodes for metrics
func UpdateMetrics(client mqtt.Client, msg mqtt.Message) {
	if servercfg.Is_EE {
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

			shouldUpdate := updateNodeMetrics(&currentNode, &newMetrics)

			if err = logic.UpdateMetrics(id, &newMetrics); err != nil {
				logger.Log(1, "faield to update node metrics", id, err.Error())
				return
			}
			if servercfg.IsMetricsExporter() {
				if err := pushMetricsToExporter(newMetrics); err != nil {
					logger.Log(2, fmt.Sprintf("failed to push node: [%s] metrics to exporter, err: %v",
						currentNode.ID, err))
				}
			}

			if newMetrics.Connectivity != nil {
				err := logic.EnterpriseFailoverFunc(&currentNode)
				if err != nil {
					logger.Log(0, "failed to failover for node", currentNode.ID.String(), "on network", currentNode.Network, "-", err.Error())
				}
			}

			if shouldUpdate {
				logger.Log(2, "updating peers after node", currentNode.ID.String(), currentNode.Network, "detected connectivity issues")
				host, err := logic.GetHost(currentNode.HostID.String())
				if err == nil {
					if err = PublishSingleHostUpdate(host); err != nil {
						logger.Log(0, "failed to publish update after failover peer change for node", currentNode.ID.String(), currentNode.Network)
					}
				}

			}

			logger.Log(1, "updated node metrics", id)
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
			//do we still need this
		case ncutils.DONE:
			updateNodePeers(&currentNode)
		}

		logger.Log(1, "sent peer updates after signal received from", id)
	}()
}

func updateNodePeers(currentNode *models.Node) {
	if err := PublishPeerUpdate(); err != nil {
		logger.Log(1, "error publishing peer update ", err.Error())
		return
	}
}

func updateNodeMetrics(currentNode *models.Node, newMetrics *models.Metrics) bool {
	if newMetrics.FailoverPeers == nil {
		newMetrics.FailoverPeers = make(map[string]string)
	}
	oldMetrics, err := logic.GetMetrics(currentNode.ID.String())
	if err != nil {
		logger.Log(1, "error finding old metrics for node", currentNode.ID.String())
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
			extMetric.IsServer = "no"
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

	// add nodes that need failover
	nodes, err := logic.GetNetworkNodes(currentNode.Network)
	if err != nil {
		logger.Log(0, "failed to retrieve nodes while updating metrics")
		return false
	}
	for _, node := range nodes {
		if !newMetrics.Connectivity[node.ID.String()].Connected &&
			len(newMetrics.Connectivity[node.ID.String()].NodeName) > 0 &&
			node.Connected == true &&
			len(node.FailoverNode) > 0 &&
			!node.Failover {
			newMetrics.FailoverPeers[node.ID.String()] = node.FailoverNode.String()
		}
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
