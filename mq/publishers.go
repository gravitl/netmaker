package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/metrics"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
)

// PublishPeerUpdate --- deterines and publishes a peer update to all the peers of a node
func PublishPeerUpdate(newNode *models.Node, publishToSelf bool) error {
	if !servercfg.IsMessageQueueBackend() {
		return nil
	}
	networkNodes, err := logic.GetNetworkNodes(newNode.Network)
	if err != nil {
		logger.Log(1, "err getting Network Nodes", err.Error())
		return err
	}
	for _, node := range networkNodes {

		if node.IsServer == "yes" {
			continue
		}
		if !publishToSelf && newNode.ID == node.ID {
			//skip self
			continue
		}
		err = PublishSinglePeerUpdate(&node)
		if err != nil {
			logger.Log(1, "failed to publish peer update to node", node.Name, "on network", node.Network, ":", err.Error())
		}
	}
	return err
}

// PublishSinglePeerUpdate --- determines and publishes a peer update to one node
func PublishSinglePeerUpdate(node *models.Node) error {
	peerUpdate, err := logic.GetPeerUpdate(node)
	if err != nil {
		return err
	}
	data, err := json.Marshal(&peerUpdate)
	if err != nil {
		return err
	}
	return publish(node, fmt.Sprintf("peers/%s/%s", node.Network, node.ID), data)
}

// PublishPeerUpdate --- publishes a peer update to all the peers of a node
func PublishExtPeerUpdate(node *models.Node) error {
	var err error
	if logic.IsLocalServer(node) {
		if err = logic.ServerUpdate(node, false); err != nil {
			logger.Log(1, "server node:", node.ID, "failed to update peers with ext clients")
			return err
		} else {
			return nil
		}
	}
	if !servercfg.IsMessageQueueBackend() {
		return nil
	}
	peerUpdate, err := logic.GetPeerUpdate(node)
	if err != nil {
		return err
	}
	data, err := json.Marshal(&peerUpdate)
	if err != nil {
		return err
	}
	if err = publish(node, fmt.Sprintf("peers/%s/%s", node.Network, node.ID), data); err != nil {
		return err
	}
	go PublishPeerUpdate(node, false)
	return nil
}

// NodeUpdate -- publishes a node update
func NodeUpdate(node *models.Node) error {
	if !servercfg.IsMessageQueueBackend() || node.IsServer == "yes" {
		return nil
	}
	logger.Log(3, "publishing node update to "+node.Name)

	if len(node.NetworkSettings.AccessKeys) > 0 {
		node.NetworkSettings.AccessKeys = []models.AccessKey{} // not to be sent (don't need to spread access keys around the network; we need to know how to reach other nodes, not become them)
	}

	data, err := json.Marshal(node)
	if err != nil {
		logger.Log(2, "error marshalling node update ", err.Error())
		return err
	}
	if err = publish(node, fmt.Sprintf("update/%s/%s", node.Network, node.ID), data); err != nil {
		logger.Log(2, "error publishing node update to peer ", node.ID, err.Error())
		return err
	}
	return nil
}

// sendPeers - retrieve networks, send peer ports to all peers
func sendPeers() {

	networks, err := logic.GetNetworks()
	if err != nil {
		logger.Log(1, "error retrieving networks for keepalive", err.Error())
	}

	var force bool
	peer_force_send++
	if peer_force_send == 5 {

		// run iptables update to ensure gateways work correctly and mq is forwarded if containerized
		if servercfg.ManageIPTables() != "off" {
			serverctl.InitIPTables(false)
		}
		servercfg.SetHost()

		force = true
		peer_force_send = 0
		err := logic.TimerCheckpoint() // run telemetry & log dumps if 24 hours has passed..
		if err != nil {
			logger.Log(3, "error occurred on timer,", err.Error())
		}

		collectServerMetrics(networks[:])
	}

	for _, network := range networks {
		serverNode, errN := logic.GetNetworkServerLocal(network.NetID)
		if errN == nil {
			serverNode.SetLastCheckIn()
			if err := logic.UpdateNode(&serverNode, &serverNode); err != nil {
				logger.Log(0, "failed checkin for server node", serverNode.Name, "on network", network.NetID, err.Error())
			}
		}
		isLeader := logic.IsLeader(&serverNode)
		if errN == nil && isLeader {
			if network.DefaultUDPHolePunch == "yes" {
				if logic.ShouldPublishPeerPorts(&serverNode) || force {
					if force {
						logger.Log(2, "sending scheduled peer update (5 min)")
					}
					err = PublishPeerUpdate(&serverNode, false)
					if err != nil {
						logger.Log(1, "error publishing udp port updates for network", network.NetID)
						logger.Log(1, errN.Error())
					}
				}
			}
		} else {
			if isLeader {
				logger.Log(1, "unable to retrieve leader for network ", network.NetID)
			}
			logger.Log(2, "server checkin complete for server", serverNode.Name, "on network", network.NetID)
			serverctl.SyncServerNetwork(network.NetID)
			if errN != nil {
				logger.Log(1, errN.Error())
			}
		}
	}
}

// ServerStartNotify - notifies all non server nodes to pull changes after a restart
func ServerStartNotify() error {
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return err
	}
	for i := range nodes {
		nodes[i].Action = models.NODE_FORCE_UPDATE
		if err = NodeUpdate(&nodes[i]); err != nil {
			logger.Log(1, "error when notifying node", nodes[i].Name, " - ", nodes[i].ID, "of a server startup")
		}
	}
	return nil
}

// function to collect and store metrics for server nodes
func collectServerMetrics(networks []models.Network) {
	if !servercfg.Is_EE {
		return
	}
	if len(networks) > 0 {
		for i := range networks {
			currentNetworkNodes, err := logic.GetNetworkNodes(networks[i].NetID)
			if err != nil {
				continue
			}
			currentServerNodes := logic.GetServerNodes(networks[i].NetID)
			if len(currentServerNodes) > 0 {
				for i := range currentServerNodes {
					if logic.IsLocalServer(&currentServerNodes[i]) {
						serverMetrics := logic.CollectServerMetrics(currentServerNodes[i].ID, currentNetworkNodes)
						if serverMetrics != nil {
							serverMetrics.NodeName = currentServerNodes[i].Name
							serverMetrics.NodeID = currentServerNodes[i].ID
							serverMetrics.IsServer = "yes"
							serverMetrics.Network = currentServerNodes[i].Network
							if err = metrics.GetExchangedBytesForNode(&currentServerNodes[i], serverMetrics); err != nil {
								logger.Log(1, fmt.Sprintf("failed to update exchanged bytes info for server: %s, err: %v",
									currentServerNodes[i].Name, err))
							}
							updateNodeMetrics(&currentServerNodes[i], serverMetrics)
							if err = logic.UpdateMetrics(currentServerNodes[i].ID, serverMetrics); err != nil {
								logger.Log(1, "failed to update metrics for server node", currentServerNodes[i].ID)
							}
							if servercfg.IsMetricsExporter() {
								logger.Log(2, "-------------> SERVER METRICS: ", fmt.Sprintf("%+v", serverMetrics))
								if err := pushMetricsToExporter(*serverMetrics); err != nil {
									logger.Log(2, "failed to push server metrics to exporter: ", err.Error())
								}
							}
						}
					}
				}
			}
		}
	}
}

func pushMetricsToExporter(metrics models.Metrics) error {
	logger.Log(2, "----> Pushing metrics to exporter")
	data, err := json.Marshal(metrics)
	if err != nil {
		return errors.New("failed to marshal metrics: " + err.Error())
	}
	if token := mqclient.Publish("metrics_exporter", 2, true, data); !token.WaitTimeout(MQ_TIMEOUT*time.Second) || token.Error() != nil {
		var err error
		if token.Error() == nil {
			err = errors.New("connection timeout")
		} else {
			err = token.Error()
		}
		return err
	}
	return nil
}
