package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	proxy_models "github.com/gravitl/netclient/nmproxy/models"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
)

// PublishPeerUpdate --- determines and publishes a peer update to all the hosts
func PublishPeerUpdate(network string, publishToSelf bool) error {
	if !servercfg.IsMessageQueueBackend() {
		return nil
	}

	hosts, err := logic.GetAllHosts()
	if err != nil {
		logger.Log(1, "err getting all hosts", err.Error())
		return err
	}
	for _, host := range hosts {
		err = PublishSingleHostUpdate(&host)
		if err != nil {
			logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
		}
	}
	return err
}

func PublishProxyPeerUpdate(node *models.Node) error {
	proxyUpdate, err := logic.GetPeersForProxy(node, false)
	if err != nil {
		return err
	}
	proxyUpdate.Action = proxy_models.AddNetwork
	err = ProxyUpdate(&proxyUpdate, node)
	if err != nil {
		logger.Log(1, "failed to send proxy update: ", err.Error())
		return err
	}
	return nil
}

// PublishSingleHostUpdate --- determines and publishes a peer update to one host
func PublishSingleHostUpdate(host *models.Host) error {

	peerUpdate, err := logic.GetPeerUpdateForHost(host)
	if err != nil {
		return err
	}
	if host.ProxyEnabled {
		// proxyUpdate, err := logic.GetPeersForProxy(node, false)
		// if err != nil {
		// 	return err
		// }
		// proxyUpdate.Action = proxy_models.AddNetwork
		// peerUpdate.ProxyUpdate = proxyUpdate

	}

	data, err := json.Marshal(&peerUpdate)
	if err != nil {
		return err
	}
	return publish(host, fmt.Sprintf("peers/host/%s/%s", host.ID.String(), servercfg.GetServer()), data)
}

// PublishPeerUpdate --- publishes a peer update to all the peers of a node
func PublishExtPeerUpdate(node *models.Node) error {

	go PublishPeerUpdate(node.Network, false)
	return nil
}

// NodeUpdate -- publishes a node update
func NodeUpdate(node *models.Node) error {
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		return nil
	}
	if !servercfg.IsMessageQueueBackend() {
		return nil
	}
	logger.Log(3, "publishing node update to "+node.ID.String())

	//if len(node.NetworkSettings.AccessKeys) > 0 {
	//node.NetworkSettings.AccessKeys = []models.AccessKey{} // not to be sent (don't need to spread access keys around the network; we need to know how to reach other nodes, not become them)
	//}

	data, err := json.Marshal(node)
	if err != nil {
		logger.Log(2, "error marshalling node update ", err.Error())
		return err
	}
	if err = publish(host, fmt.Sprintf("update/%s/%s", node.Network, node.ID), data); err != nil {
		logger.Log(2, "error publishing node update to peer ", node.ID.String(), err.Error())
		return err
	}
	if host.ProxyEnabled {
		err = PublishProxyPeerUpdate(node)
		if err != nil {
			logger.Log(1, "failed to publish proxy update to node", node.ID.String(), "on network", node.Network, ":", err.Error())
		}
	}

	return nil
}

// ProxyUpdate -- publishes updates to peers related to proxy
func ProxyUpdate(proxyPayload *proxy_models.ProxyManagerPayload, node *models.Node) error {
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		return nil
	}
	if !servercfg.IsMessageQueueBackend() || !host.ProxyEnabled {
		return nil
	}
	logger.Log(3, "publishing proxy update to "+node.ID.String())

	data, err := json.Marshal(proxyPayload)
	if err != nil {
		logger.Log(2, "error marshalling node update ", err.Error())
		return err
	}
	if err = publish(host, fmt.Sprintf("proxy/%s/%s", node.Network, node.ID), data); err != nil {
		logger.Log(2, "error publishing proxy update to peer ", node.ID.String(), err.Error())
		return err
	}
	return nil
}

// sendPeers - retrieve networks, send peer ports to all peers
func sendPeers() {

	hosts, err := logic.GetAllHosts()
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

		//collectServerMetrics(networks[:])
	}

	for _, host := range hosts {
		if force {
			logger.Log(2, "sending scheduled peer update (5 min)")
			err = PublishSingleHostUpdate(&host)
			if err != nil {
				logger.Log(1, "error publishing peer updates for host: ", host.ID.String(), " Err: ", err.Error())
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
			logger.Log(1, "error when notifying node", nodes[i].ID.String(), "of a server startup")
		}
	}
	return nil
}

// function to collect and store metrics for server nodes
//func collectServerMetrics(networks []models.Network) {
//	if !servercfg.Is_EE {
//		return
//	}
//	if len(networks) > 0 {
//		for i := range networks {
//			currentNetworkNodes, err := logic.GetNetworkNodes(networks[i].NetID)
//			if err != nil {
//				continue
//			}
//			currentServerNodes := logic.GetServerNodes(networks[i].NetID)
//			if len(currentServerNodes) > 0 {
//				for i := range currentServerNodes {
//					if logic.IsLocalServer(&currentServerNodes[i]) {
//						serverMetrics := logic.CollectServerMetrics(currentServerNodes[i].ID, currentNetworkNodes)
//						if serverMetrics != nil {
//							serverMetrics.NodeName = currentServerNodes[i].Name
//							serverMetrics.NodeID = currentServerNodes[i].ID
//							serverMetrics.IsServer = "yes"
//							serverMetrics.Network = currentServerNodes[i].Network
//							if err = metrics.GetExchangedBytesForNode(&currentServerNodes[i], serverMetrics); err != nil {
//								logger.Log(1, fmt.Sprintf("failed to update exchanged bytes info for server: %s, err: %v",
//									currentServerNodes[i].Name, err))
//							}
//							updateNodeMetrics(&currentServerNodes[i], serverMetrics)
//							if err = logic.UpdateMetrics(currentServerNodes[i].ID, serverMetrics); err != nil {
//								logger.Log(1, "failed to update metrics for server node", currentServerNodes[i].ID)
//							}
//							if servercfg.IsMetricsExporter() {
//								logger.Log(2, "-------------> SERVER METRICS: ", fmt.Sprintf("%+v", serverMetrics))
//								if err := pushMetricsToExporter(*serverMetrics); err != nil {
//									logger.Log(2, "failed to push server metrics to exporter: ", err.Error())
//								}
//							}
//						}
//					}
//				}
//			}
//		}
//	}
//}

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
