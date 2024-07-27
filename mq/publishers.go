package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

type ServerStatus struct {
	DB               bool      `json:"db_connected"`
	Broker           bool      `json:"broker_connected"`
	IsBrokerConnOpen bool      `json:"is_broker_conn_open"`
	LicenseError     string    `json:"license_error"`
	IsPro            bool      `json:"is_pro"`
	TrialEndDate     time.Time `json:"trial_end_date"`
	IsOnTrialLicense bool      `json:"is_on_trial_license"`
}

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
	for _, host := range hosts {
		host := host
		go func(host models.Host) {
			if err = PublishSingleHostPeerUpdate(&host, allNodes, nil, nil, replacePeers); err != nil {
				logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
			}
		}(host)
	}
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
	for _, host := range hosts {
		host := host
		if err = PublishSingleHostPeerUpdate(&host, allNodes, delNode, nil, false); err != nil {
			logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
		}
	}
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
	for _, host := range hosts {
		host := host
		if host.OS != models.OS_Types.IoT {
			if err = PublishSingleHostPeerUpdate(&host, nodes, nil, []models.ExtClient{*delClient}, false); err != nil {
				logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
			}
		}
	}
	return err
}

// PublishSingleHostPeerUpdate --- determines and publishes a peer update to one host
func PublishSingleHostPeerUpdate(host *models.Host, allNodes []models.Node, deletedNode *models.Node, deletedClients []models.ExtClient, replacePeers bool) error {

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

func ServerStatusUpdate() error {

	licenseErr := ""
	if servercfg.ErrLicenseValidation != nil {
		licenseErr = servercfg.ErrLicenseValidation.Error()
	}
	var trialEndDate time.Time
	var err error
	isOnTrial := false
	if servercfg.IsPro && (servercfg.GetLicenseKey() == "" || servercfg.GetNetmakerTenantID() == "") {
		trialEndDate, err = logic.GetTrialEndDate()
		if err != nil {
			slog.Error("failed to get trial end date", "error", err)
		} else {
			isOnTrial = true
		}
	}
	currentServerStatus := ServerStatus{
		DB:               database.IsConnected(),
		Broker:           IsConnected(),
		IsBrokerConnOpen: IsConnectionOpen(),
		LicenseError:     licenseErr,
		IsPro:            servercfg.IsPro,
		TrialEndDate:     trialEndDate,
		IsOnTrialLicense: isOnTrial,
	}

	data, err := json.Marshal(currentServerStatus)
	if err != nil {
		slog.Error("error marshalling server status update ", err.Error())
		return err
	}

	if mqclient == nil || !mqclient.IsConnected() {
		return errors.New("cannot publish ... mqclient not connected")
	}

	if token := mqclient.Publish("server/status", 0, true, data); token.Wait() && token.Error() != nil {
		slog.Error("could not publish server status", "error", token.Error().Error())
		return token.Error()
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
	if mqclient == nil || !mqclient.IsConnected() {
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
