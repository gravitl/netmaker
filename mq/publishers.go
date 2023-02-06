package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// PublishPeerUpdate --- determines and publishes a peer update to all the hosts
func PublishPeerUpdate() error {
	if !servercfg.IsMessageQueueBackend() {
		return nil
	}

	hosts, err := logic.GetAllHosts()
	if err != nil {
		logger.Log(1, "err getting all hosts", err.Error())
		return err
	}
	for _, host := range hosts {
		host := host
		err = PublishSingleHostUpdate(&host)
		if err != nil {
			logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
		}
	}
	return err
}

// PublishSingleHostUpdate --- determines and publishes a peer update to one host
func PublishSingleHostUpdate(host *models.Host) error {

	peerUpdate, err := logic.GetPeerUpdateForHost(host)
	if err != nil {
		return err
	}
	if host.ProxyEnabled {
		proxyUpdate, err := logic.GetProxyUpdateForHost(host)
		if err != nil {
			return err
		}
		proxyUpdate.Action = models.ProxyUpdate
		peerUpdate.ProxyUpdate = proxyUpdate
	}

	data, err := json.Marshal(&peerUpdate)
	if err != nil {
		return err
	}
	return publish(host, fmt.Sprintf("peers/host/%s/%s", host.ID.String(), servercfg.GetServer()), data)
}

// PublishPeerUpdate --- publishes a peer update to all the peers of a node
func PublishExtPeerUpdate(node *models.Node) error {

	go PublishPeerUpdate()
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

	return nil
}

// HostUpdate -- publishes a host update to clients
func HostUpdate(hostUpdate *models.HostUpdate) error {
	if !servercfg.IsMessageQueueBackend() {
		return nil
	}
	logger.Log(3, "publishing host update to "+hostUpdate.Host.ID.String())

	data, err := json.Marshal(hostUpdate)
	if err != nil {
		logger.Log(2, "error marshalling node update ", err.Error())
		return err
	}
	if err = publish(&hostUpdate.Host, fmt.Sprintf("host/update/%s/%s", hostUpdate.Host.ID.String(), servercfg.GetServer()), data); err != nil {
		logger.Log(2, "error publishing host update to", hostUpdate.Host.ID.String(), err.Error())
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
			host := host
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

func PublishDNSUpdate(network string, dns models.DNSUpdate) error {
	nodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		host, err := logic.GetHost(node.HostID.String())
		if err != nil {
			logger.Log(0, "error retrieving host for dns update", host.ID.String(), err.Error())
			continue
		}
		data, err := json.Marshal(dns)
		if err != nil {
			logger.Log(0, "failed to encode dns data for node", node.ID.String(), err.Error())
		}
		if err := publish(host, "network/"+host.ID.String()+"/dns", data); err != nil {
			logger.Log(0, "error publishing dns update to host", host.ID.String(), err.Error())
			continue
		}
		logger.Log(3, "published dns update to host", host.ID.String())
	}
	return nil
}

func PublishAllDNS(newnode *models.Node) error {
	alldns := []models.DNSUpdate{}
	dns := models.DNSUpdate{}
	newnodeHost, err := logic.GetHost(newnode.HostID.String())
	if err != nil {
		return fmt.Errorf("error retrieving host for dns update %w", err)
	}
	nodes, err := logic.GetNetworkNodes(newnode.Network)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.ID == newnode.ID {
			//skip self
			continue
		}
		host, err := logic.GetHost(node.HostID.String())
		if err != nil {
			logger.Log(0, "error retrieving host for dns update", host.ID.String(), err.Error())
			continue
		}
		if node.Address.IP != nil {
			dns.Action = models.DNSInsert
			dns.Name = host.Name + "." + node.Network
			dns.Address = node.Address.IP.String()
			alldns = append(alldns, dns)
		}
		if node.Address6.IP != nil {
			dns.Action = models.DNSInsert
			dns.Name = host.Name + "." + node.Network
			dns.Address = node.Address6.IP.String()
			alldns = append(alldns, dns)
		}
	}
	entries, err := logic.GetCustomDNS(newnode.Network)
	if err != nil {
		logger.Log(0, "error retrieving custom dns entries", err.Error())
	}
	for _, entry := range entries {
		dns.Action = models.DNSInsert
		dns.Address = entry.Address
		dns.Name = entry.Name
		alldns = append(alldns, dns)
	}
	data, err := json.Marshal(alldns)
	if err != nil {
		return fmt.Errorf("error encoding dnd data %w", err)
	}
	if err := publish(newnodeHost, "network/"+newnodeHost.ID.String()+"/fulldns", data); err != nil {
		return fmt.Errorf("error publish full dns update to %s, %w", newnodeHost.ID.String(), err)
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
