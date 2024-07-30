package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

type ServerStatus struct {
	DB               bool            `json:"db_connected"`
	Broker           bool            `json:"broker_connected"`
	IsBrokerConnOpen bool            `json:"is_broker_conn_open"`
	LicenseError     string          `json:"license_error"`
	IsPro            bool            `json:"is_pro"`
	TrialEndDate     time.Time       `json:"trial_end_date"`
	IsOnTrialLicense bool            `json:"is_on_trial_license"`
	Failover         map[string]bool `json:"is_failover_existed"`
}

const batchSize = 50

// PublishPeerUpdate --- determines and publishes a peer update to all the hosts
func PublishPeerUpdate(replacePeers bool) error {
	if !servercfg.IsMessageQueueBackend() {
		return nil
	}

	hosts, err := logic.GetAllHosts()
	if err != nil {
		logger.Log(1, "err getting all hosts", err.Error())
		return err
	}
	allNodes, err := logic.GetAllNodes()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	hostLen := len(hosts)
	batch := batchSize
	div := hostLen / batch
	mod := hostLen % batch

	if div == 0 {
		wg.Add(hostLen)
		for i := 0; i < hostLen; i++ {
			host := hosts[i]
			go func(host models.Host) {
				if err = PublishSingleHostPeerUpdate(&host, allNodes, nil, nil, replacePeers, &wg); err != nil {
					logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
				}
			}(host)
		}
		wg.Wait()
	} else {
		for i := 0; i < div*batch; i += batch {
			wg.Add(batch)
			for j := 0; j < batch; j++ {
				host := hosts[i+j]
				go func(host models.Host) {
					if err = PublishSingleHostPeerUpdate(&host, allNodes, nil, nil, replacePeers, &wg); err != nil {
						logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
					}
				}(host)
			}
			wg.Wait()
		}
		if mod != 0 {
			wg.Add(hostLen - (div * batch))
			for k := div * batch; k < hostLen; k++ {
				host := hosts[k]
				go func(host models.Host) {
					if err = PublishSingleHostPeerUpdate(&host, allNodes, nil, nil, replacePeers, &wg); err != nil {
						logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
					}
				}(host)
			}
			wg.Wait()
		}
	}

	// for _, host := range hosts {
	// 	host := host
	// 	go func(host models.Host) {
	// 		if err = PublishSingleHostPeerUpdate(&host, allNodes, nil, nil, replacePeers, nil); err != nil {
	// 			logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
	// 		}
	// 	}(host)
	// }
	return err
}

// PublishDeletedNodePeerUpdate --- determines and publishes a peer update
// to all the hosts with a deleted node to account for
func PublishDeletedNodePeerUpdate(delNode *models.Node) error {
	if !servercfg.IsMessageQueueBackend() {
		return nil
	}

	hosts, err := logic.GetAllHosts()
	if err != nil {
		logger.Log(1, "err getting all hosts", err.Error())
		return err
	}
	allNodes, err := logic.GetAllNodes()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	hostLen := len(hosts)
	batch := batchSize
	div := hostLen / batch
	mod := hostLen % batch

	if div == 0 {
		wg.Add(hostLen)
		for i := 0; i < hostLen; i++ {
			host := hosts[i]
			go func(host models.Host) {
				if err = PublishSingleHostPeerUpdate(&host, allNodes, delNode, nil, false, &wg); err != nil {
					logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
				}
			}(host)
		}
		wg.Wait()
	} else {
		for i := 0; i < div*batch; i += batch {
			wg.Add(batch)
			for j := 0; j < batch; j++ {
				host := hosts[i+j]
				go func(host models.Host) {
					if err = PublishSingleHostPeerUpdate(&host, allNodes, delNode, nil, false, &wg); err != nil {
						logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
					}
				}(host)
			}
			wg.Wait()
		}
		if mod != 0 {
			wg.Add(hostLen - (div * batch))
			for k := div * batch; k < hostLen; k++ {
				host := hosts[k]
				go func(host models.Host) {
					if err = PublishSingleHostPeerUpdate(&host, allNodes, delNode, nil, false, &wg); err != nil {
						logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
					}
				}(host)
			}
			wg.Wait()
		}
	}
	// for _, host := range hosts {
	// 	host := host
	// 	if err = PublishSingleHostPeerUpdate(&host, allNodes, delNode, nil, false, nil); err != nil {
	// 		logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
	// 	}
	// }
	return err
}

// PublishDeletedClientPeerUpdate --- determines and publishes a peer update
// to all the hosts with a deleted ext client to account for
func PublishDeletedClientPeerUpdate(delClient *models.ExtClient) error {
	if !servercfg.IsMessageQueueBackend() {
		return nil
	}

	hosts, err := logic.GetAllHosts()
	if err != nil {
		logger.Log(1, "err getting all hosts", err.Error())
		return err
	}
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	hostLen := len(hosts)
	batch := batchSize
	div := hostLen / batch
	mod := hostLen % batch

	if div == 0 {
		wg.Add(hostLen)
		for i := 0; i < hostLen; i++ {
			host := hosts[i]
			go func(host models.Host) {
				if host.OS != models.OS_Types.IoT {
					if err = PublishSingleHostPeerUpdate(&host, nodes, nil, []models.ExtClient{*delClient}, false, &wg); err != nil {
						logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
					}
				}
			}(host)
		}
		wg.Wait()
	} else {
		for i := 0; i < div*batch; i += batch {
			wg.Add(batch)
			for j := 0; j < batch; j++ {
				host := hosts[i+j]
				go func(host models.Host) {
					if host.OS != models.OS_Types.IoT {
						if err = PublishSingleHostPeerUpdate(&host, nodes, nil, []models.ExtClient{*delClient}, false, &wg); err != nil {
							logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
						}
					}
				}(host)
			}
			wg.Wait()
		}
		if mod != 0 {
			wg.Add(hostLen - (div * batch))
			for k := div * batch; k < hostLen; k++ {
				host := hosts[k]
				go func(host models.Host) {
					if host.OS != models.OS_Types.IoT {
						if err = PublishSingleHostPeerUpdate(&host, nodes, nil, []models.ExtClient{*delClient}, false, &wg); err != nil {
							logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
						}
					}
				}(host)
			}
			wg.Wait()
		}
	}

	// for _, host := range hosts {
	// 	host := host
	// 	if host.OS != models.OS_Types.IoT {
	// 		if err = PublishSingleHostPeerUpdate(&host, nodes, nil, []models.ExtClient{*delClient}, false, nil); err != nil {
	// 			logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
	// 		}
	// 	}
	// }
	return err
}

// PublishSingleHostPeerUpdate --- determines and publishes a peer update to one host
func PublishSingleHostPeerUpdate(host *models.Host, allNodes []models.Node, deletedNode *models.Node, deletedClients []models.ExtClient, replacePeers bool, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}
	peerUpdate, err := logic.GetPeerUpdateForHost("", host, allNodes, deletedNode, deletedClients)
	if err != nil {
		return err
	}
	peerUpdate.ReplacePeers = replacePeers
	data, err := json.Marshal(&peerUpdate)
	if err != nil {
		return err
	}
	return publish(host, fmt.Sprintf("peers/host/%s/%s", host.ID.String(), servercfg.GetServer()), data)
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
	if err = publish(host, fmt.Sprintf("node/update/%s/%s", node.Network, node.ID), data); err != nil {
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

// PublishMqUpdatesForDeletedNode - published all the required updates for deleted node
func PublishMqUpdatesForDeletedNode(node models.Node, sendNodeUpdate bool, gwClients []models.ExtClient) {
	// notify of peer change
	node.PendingDelete = true
	node.Action = models.NODE_DELETE
	if sendNodeUpdate {
		if err := NodeUpdate(&node); err != nil {
			slog.Error("error publishing node update to node", "node", node.ID, "error", err)
		}
	}
	if err := PublishDeletedNodePeerUpdate(&node); err != nil {
		logger.Log(1, "error publishing peer update ", err.Error())
	}
	if servercfg.IsDNSMode() {
		logic.SetDNS()
	}

}

func PushMetricsToExporter(metrics models.Metrics) error {
	logger.Log(2, "----> Pushing metrics to exporter")
	data, err := json.Marshal(metrics)
	if err != nil {
		return errors.New("failed to marshal metrics: " + err.Error())
	}
	if mqclient == nil || !mqclient.IsConnectionOpen() {
		return errors.New("cannot publish ... mqclient not connected")
	}
	if token := mqclient.Publish("metrics_exporter", 0, true, data); !token.WaitTimeout(MQ_TIMEOUT*time.Second) || token.Error() != nil {
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

// sendPeers - retrieve networks, send peer ports to all peers
func sendPeers() {

	// hosts, err := logic.GetAllHosts()
	// if err != nil && len(hosts) > 0 {
	// 	logger.Log(1, "error retrieving networks for keepalive", err.Error())
	// }

	peer_force_send++
	if peer_force_send == 5 {
		servercfg.SetHost()
		peer_force_send = 0
		err := logic.TimerCheckpoint() // run telemetry & log dumps if 24 hours has passed..
		if err != nil {
			logger.Log(3, "error occurred on timer,", err.Error())
		}
	}
}
