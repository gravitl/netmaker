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
	allNodes, err := logic.GetAllNodes()
	if err != nil {
		return err
	}
	for _, host := range hosts {
		host := host
		if err = PublishSingleHostPeerUpdate(&host, allNodes, nil, nil); err != nil {
			logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
		}
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
		if err = PublishSingleHostPeerUpdate(&host, allNodes, delNode, nil); err != nil {
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
		if err = PublishSingleHostPeerUpdate(&host, nodes, nil, []models.ExtClient{*delClient}); err != nil {
			logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
		}
	}
	return err
}

// PublishSingleHostPeerUpdate --- determines and publishes a peer update to one host
func PublishSingleHostPeerUpdate(host *models.Host, allNodes []models.Node, deletedNode *models.Node, deletedClients []models.ExtClient) error {

	peerUpdate, err := logic.GetPeerUpdateForHost("", host, allNodes, deletedNode, deletedClients)
	if err != nil {
		return err
	}
	if len(peerUpdate.Peers) == 0 { // no peers to send
		return nil
	}

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

// PublishDNSUpdate publishes a dns update to all nodes on a network
func PublishDNSUpdate(network string, dns models.DNSUpdate) error {
	nodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		host, err := logic.GetHost(node.HostID.String())
		if err != nil {
			logger.Log(0, "error retrieving host for dns update", node.HostID.String(), err.Error())
			continue
		}
		data, err := json.Marshal(dns)
		if err != nil {
			logger.Log(0, "failed to encode dns data for node", node.ID.String(), err.Error())
		}
		if err := publish(host, "dns/update/"+host.ID.String()+"/"+servercfg.GetServer(), data); err != nil {
			logger.Log(0, "error publishing dns update to host", host.ID.String(), err.Error())
			continue
		}
		logger.Log(3, "published dns update to host", host.ID.String())
	}
	return nil
}

// PublishAllDNS publishes an array of dns updates (ip / host.network) for each peer to a node joining a network
func PublishAllDNS(newnode *models.Node) error {
	alldns := []models.DNSUpdate{}
	newnodeHost, err := logic.GetHost(newnode.HostID.String())
	if err != nil {
		return fmt.Errorf("error retrieving host for dns update %w", err)
	}
	alldns = append(alldns, getNodeDNS(newnode.Network)...)
	alldns = append(alldns, getExtClientDNS(newnode.Network)...)
	alldns = append(alldns, getCustomDNS(newnode.Network)...)
	data, err := json.Marshal(alldns)
	if err != nil {
		return fmt.Errorf("error encoding dns data %w", err)
	}
	if err := publish(newnodeHost, "dns/all/"+newnodeHost.ID.String()+"/"+servercfg.GetServer(), data); err != nil {
		return fmt.Errorf("error publishing full dns update to %s, %w", newnodeHost.ID.String(), err)
	}
	logger.Log(3, "published full dns update to %s", newnodeHost.ID.String())
	return nil
}

// PublishDNSDelete publish a dns update deleting a node to all hosts on a network
func PublishDNSDelete(node *models.Node, host *models.Host) error {
	dns := models.DNSUpdate{
		Action: models.DNSDeleteByIP,
		Name:   host.Name + "." + node.Network,
	}
	if node.Address.IP != nil {
		dns.Address = node.Address.IP.String()
		if err := PublishDNSUpdate(node.Network, dns); err != nil {
			return fmt.Errorf("dns update node deletion %w", err)
		}
	}
	if node.Address6.IP != nil {
		dns.Address = node.Address6.IP.String()
		if err := PublishDNSUpdate(node.Network, dns); err != nil {
			return fmt.Errorf("dns update node deletion %w", err)
		}
	}
	return nil
}

// PublishReplaceDNS publish a dns update to replace a dns entry on all hosts in network
func PublishReplaceDNS(oldNode, newNode *models.Node, host *models.Host) error {
	dns := models.DNSUpdate{
		Action: models.DNSReplaceIP,
		Name:   host.Name + "." + oldNode.Network,
	}
	if !oldNode.Address.IP.Equal(newNode.Address.IP) {
		dns.Address = oldNode.Address.IP.String()
		dns.NewAddress = newNode.Address.IP.String()
		if err := PublishDNSUpdate(oldNode.Network, dns); err != nil {
			return err
		}
	}
	if !oldNode.Address6.IP.Equal(newNode.Address6.IP) {
		dns.Address = oldNode.Address6.IP.String()
		dns.NewAddress = newNode.Address6.IP.String()
		if err := PublishDNSUpdate(oldNode.Network, dns); err != nil {
			return err
		}
	}
	return nil
}

// PublishExtClientDNS publish dns update for new extclient
func PublishExtCLientDNS(client *models.ExtClient) error {
	errMsgs := models.DNSError{}
	dns := models.DNSUpdate{
		Action:  models.DNSInsert,
		Name:    client.ClientID + "." + client.Network,
		Address: client.Address,
	}
	if client.Address != "" {
		dns.Address = client.Address
		if err := PublishDNSUpdate(client.Network, dns); err != nil {
			errMsgs.ErrorStrings = append(errMsgs.ErrorStrings, err.Error())
		}

	}
	if client.Address6 != "" {
		dns.Address = client.Address6
		if err := PublishDNSUpdate(client.Network, dns); err != nil {
			errMsgs.ErrorStrings = append(errMsgs.ErrorStrings, err.Error())
		}
	}
	if len(errMsgs.ErrorStrings) > 0 {
		return errMsgs
	}
	return nil
}

// PublishExtClientDNSUpdate update for extclient name change
func PublishExtClientDNSUpdate(old, new models.ExtClient, network string) error {
	dns := models.DNSUpdate{
		Action:  models.DNSReplaceName,
		Name:    old.ClientID + "." + network,
		NewName: new.ClientID + "." + network,
	}
	if err := PublishDNSUpdate(network, dns); err != nil {
		return err
	}
	return nil
}

// PublishDeleteExtClientDNS publish dns update to delete extclient entry
func PublishDeleteExtClientDNS(client *models.ExtClient) error {
	dns := models.DNSUpdate{
		Action: models.DNSDeleteByName,
		Name:   client.ClientID + "." + client.Network,
	}
	if err := PublishDNSUpdate(client.Network, dns); err != nil {
		return err
	}
	return nil
}

// PublishCustomDNS publish dns update for new custom dns entry
func PublishCustomDNS(entry *models.DNSEntry) error {
	dns := models.DNSUpdate{
		Action: models.DNSInsert,
		Name:   entry.Name + "." + entry.Network,
		//entry.Address6 is never used
		Address: entry.Address,
	}
	if err := PublishDNSUpdate(entry.Network, dns); err != nil {
		return err
	}
	return nil
}

// PublishHostDNSUpdate publishes dns update on host name change
func PublishHostDNSUpdate(old, new *models.Host, networks []string) error {
	errMsgs := models.DNSError{}
	for _, network := range networks {
		dns := models.DNSUpdate{
			Action:  models.DNSReplaceName,
			Name:    old.Name + "." + network,
			NewName: new.Name + "." + network,
		}
		if err := PublishDNSUpdate(network, dns); err != nil {
			errMsgs.ErrorStrings = append(errMsgs.ErrorStrings, err.Error())
		}
	}
	if len(errMsgs.ErrorStrings) > 0 {
		return errMsgs
	}
	return nil
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

func getNodeDNS(network string) []models.DNSUpdate {
	alldns := []models.DNSUpdate{}
	dns := models.DNSUpdate{}
	nodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		logger.Log(0, "error retreiving network nodes for network", network, err.Error())
	}
	for _, node := range nodes {
		host, err := logic.GetHost(node.HostID.String())
		if err != nil {
			logger.Log(0, "error retrieving host for dns update", node.HostID.String(), err.Error())
			continue
		}
		dns.Action = models.DNSInsert
		dns.Name = host.Name + "." + node.Network
		if node.Address.IP != nil {
			dns.Address = node.Address.IP.String()
			alldns = append(alldns, dns)
		}
		if node.Address6.IP != nil {
			dns.Address = node.Address6.IP.String()
			alldns = append(alldns, dns)
		}
	}
	return alldns
}

func getExtClientDNS(network string) []models.DNSUpdate {
	alldns := []models.DNSUpdate{}
	dns := models.DNSUpdate{}
	clients, err := logic.GetNetworkExtClients(network)
	if err != nil {
		logger.Log(0, "error retrieving extclients", err.Error())
	}
	for _, client := range clients {
		dns.Action = models.DNSInsert
		dns.Name = client.ClientID + "." + client.Network
		if client.Address != "" {
			dns.Address = client.Address
			alldns = append(alldns, dns)
		}
		if client.Address6 != "" {
			dns.Address = client.Address
			alldns = append(alldns, dns)
		}
	}
	return alldns
}

func getCustomDNS(network string) []models.DNSUpdate {
	alldns := []models.DNSUpdate{}
	dns := models.DNSUpdate{}
	customdns, err := logic.GetCustomDNS(network)
	if err != nil {
		logger.Log(0, "error retrieving custom dns entries", err.Error())
	}
	for _, custom := range customdns {
		dns.Action = models.DNSInsert
		dns.Address = custom.Address
		dns.Name = custom.Name + "." + custom.Network
		alldns = append(alldns, dns)
	}
	return alldns
}

// sendPeers - retrieve networks, send peer ports to all peers
func sendPeers() {

	hosts, err := logic.GetAllHosts()
	if err != nil && len(hosts) > 0 {
		logger.Log(1, "error retrieving networks for keepalive", err.Error())
	}
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return
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
	if force {
		for _, host := range hosts {
			host := host
			logger.Log(2, "sending scheduled peer update (5 min)")
			if err = PublishSingleHostPeerUpdate(&host, nodes, nil, nil); err != nil {
				logger.Log(1, "error publishing peer updates for host: ", host.ID.String(), " Err: ", err.Error())
			}
		}
	}
}
