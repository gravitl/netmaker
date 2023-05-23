package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
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
	logic.ResetPeerUpdateContext()
	for _, host := range hosts {
		host := host
		if err = PublishSingleHostPeerUpdate(logic.PeerUpdateCtx, &host, nil, nil); err != nil {
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
	logic.ResetPeerUpdateContext()
	for _, host := range hosts {
		host := host
		if err = PublishSingleHostPeerUpdate(logic.PeerUpdateCtx, &host, delNode, nil); err != nil {
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
	logic.ResetPeerUpdateContext()
	for _, host := range hosts {
		host := host
		if err = PublishSingleHostPeerUpdate(logic.PeerUpdateCtx, &host, nil, []models.ExtClient{*delClient}); err != nil {
			logger.Log(1, "failed to publish peer update to host", host.ID.String(), ": ", err.Error())
		}
	}
	return err
}

// PublishSingleHostPeerUpdate --- determines and publishes a peer update to one host
func PublishSingleHostPeerUpdate(ctx context.Context, host *models.Host, deletedNode *models.Node, deletedClients []models.ExtClient) error {

	peerUpdate, err := logic.GetPeerUpdateForHost(ctx, "", host, deletedNode, deletedClients)
	if err != nil {
		return err
	}
	if len(peerUpdate.Peers) == 0 { // no peers to send
		return nil
	}
	proxyUpdate, err := logic.GetProxyUpdateForHost(ctx, host)
	if err != nil {
		return err
	}
	proxyUpdate.Server = servercfg.GetServer()
	if host.ProxyEnabled {
		proxyUpdate.Action = models.ProxyUpdate
	} else {
		proxyUpdate.Action = models.NoProxy
	}

	peerUpdate.ProxyUpdate = proxyUpdate

	data, err := json.Marshal(&peerUpdate)
	if err != nil {
		return err
	}
	return publish(host, fmt.Sprintf("peers/host/%s/%s", host.ID.String(), servercfg.GetServer()), data)
}

// FlushNetworkPeersToHost - sends all the peers in the network to the host.
func FlushNetworkPeersToHost(host *models.Host, hNode *models.Node, networkNodes []models.Node) error {
	logger.Log(0, "flushing network peers to host: ", host.ID.String(), hNode.Network)
	addPeerAction := models.PeerAction{
		Action: models.AddPeer,
		Peers:  []wgtypes.PeerConfig{},
	}
	rmPeerAction := models.PeerAction{
		Action: models.RemovePeer,
		Peers:  []wgtypes.PeerConfig{},
	}
	for _, node := range networkNodes {
		if node.ID == hNode.ID {
			// skip self
			continue
		}
		peerHost, err := logic.GetHost(node.HostID.String())
		if err != nil {
			continue
		}

		if !nodeacls.AreNodesAllowed(nodeacls.NetworkID(node.Network), nodeacls.NodeID(hNode.ID.String()), nodeacls.NodeID(node.ID.String())) ||
			hNode.Action == models.NODE_DELETE || hNode.PendingDelete || !hNode.Connected {
			// remove peer if not allowed
			rmPeerAction.Peers = append(rmPeerAction.Peers, wgtypes.PeerConfig{
				PublicKey: peerHost.PublicKey,
				Remove:    true,
			})
			continue

		}
		peerCfg := wgtypes.PeerConfig{
			PublicKey: peerHost.PublicKey,
			Endpoint: &net.UDPAddr{
				IP:   peerHost.EndpointIP,
				Port: logic.GetPeerListenPort(peerHost),
			},
			PersistentKeepaliveInterval: &node.PersistentKeepalive,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  logic.GetAllowedIPs(hNode, &node, nil),
		}
		addPeerAction.Peers = append(addPeerAction.Peers, peerCfg)
	}
	if hNode.IsIngressGateway {
		extPeers, _, err := logic.GetExtPeers(hNode)
		if err == nil {
			addPeerAction.Peers = append(addPeerAction.Peers, extPeers...)
		}

	}
	if len(rmPeerAction.Peers) > 0 {
		data, err := json.Marshal(rmPeerAction)
		if err != nil {
			return err
		}
		publish(host, fmt.Sprintf("peer/host/%s/%s", host.ID.String(), servercfg.GetServer()), data)
	}
	if len(addPeerAction.Peers) > 0 {
		data, err := json.Marshal(addPeerAction)
		if err != nil {
			return err
		}
		publish(host, fmt.Sprintf("peer/host/%s/%s", host.ID.String(), servercfg.GetServer()), data)
	}
	// send fw update if gw host
	if hNode.IsIngressGateway || hNode.IsEgressGateway {
		f, err := logic.GetFwUpdate(host)
		if err == nil {
			PublishFwUpdate(host, &f)
		}

	}
	return nil
}

// BroadcastDelPeer - notifys all the hosts in the network to remove peer
func BroadcastDelPeer(host *models.Host, network string) error {
	nodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		return err
	}
	p := models.PeerAction{
		Action: models.RemovePeer,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey: host.PublicKey,
				Remove:    true,
			},
		},
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	for _, nodeI := range nodes {
		if nodeI.HostID == host.ID {
			// skip self...
			continue
		}
		peerHost, err := logic.GetHost(nodeI.HostID.String())
		if err == nil {
			publish(peerHost, fmt.Sprintf("peer/host/%s/%s", peerHost.ID.String(), servercfg.GetServer()), data)
			if nodeI.IsIngressGateway {
				// TODO: FW
			}
		}
	}
	return nil
}

// BroadcastAclUpdate - sends new acl updates to peers
func BroadcastAclUpdate(network string) error {
	nodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		return err
	}
	for _, nodeI := range nodes {
		nodeI := nodeI
		h, err := logic.GetHost(nodeI.HostID.String())
		if err == nil {
			go FlushNetworkPeersToHost(h, &nodeI, nodes)
		}
	}
	return err
}

// BroadcastAddOrUpdatePeer - notifys the hosts in the network to add or update peer.
func BroadcastAddOrUpdatePeer(host *models.Host, node *models.Node, update bool) error {
	nodes, err := logic.GetNetworkNodes(node.Network)
	if err != nil {
		return err
	}

	p := models.PeerAction{
		Action: models.AddPeer,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey: host.PublicKey,
				Endpoint: &net.UDPAddr{
					IP:   host.EndpointIP,
					Port: logic.GetPeerListenPort(host),
				},
				PersistentKeepaliveInterval: &node.PersistentKeepalive,
				ReplaceAllowedIPs:           true,
			},
		},
	}
	if update {
		p.Action = models.UpdatePeer
	}
	for _, nodeI := range nodes {
		if nodeI.ID.String() == node.ID.String() {
			// skip self...
			continue
		}
		// update allowed ips, according to the peer node
		p.Peers[0].AllowedIPs = logic.GetAllowedIPs(&nodeI, node, nil)
		if update && (!nodeacls.AreNodesAllowed(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()), nodeacls.NodeID(nodeI.ID.String())) ||
			node.Action == models.NODE_DELETE || node.PendingDelete || !node.Connected) {
			// remove peer
			p.Action = models.RemovePeer
			p.Peers[0].Remove = true
		}
		data, err := json.Marshal(p)
		if err != nil {
			continue
		}
		peerHost, err := logic.GetHost(nodeI.HostID.String())
		if err == nil {
			publish(peerHost, fmt.Sprintf("peer/host/%s/%s", peerHost.ID.String(), servercfg.GetServer()), data)
		}
	}
	return nil
}

// BroadcastExtClient - publishes msg to add/updates ext client in the network
func BroadcastExtClient(ingressHost *models.Host, ingressNode *models.Node) error {

	nodes, err := logic.GetNetworkNodes(ingressNode.Network)
	if err != nil {
		return err
	}
	//flush peers to ingress host
	go FlushNetworkPeersToHost(ingressHost, ingressNode, nodes)
	// broadcast to update ingress peer to other hosts
	go BroadcastAddOrUpdatePeer(ingressHost, ingressNode, true)
	return nil
}

// BroadcastDelExtClient - published msg to remove ext client from network
func BroadcastDelExtClient(ingressHost *models.Host, ingressNode *models.Node, extclients []models.ExtClient) error {
	// TODO - send fw update
	go BroadcastAddOrUpdatePeer(ingressHost, ingressNode, true)
	peers := []wgtypes.PeerConfig{}
	for _, extclient := range extclients {
		extPubKey, err := wgtypes.ParseKey(extclient.PublicKey)
		if err != nil {
			continue
		}
		peers = append(peers, wgtypes.PeerConfig{
			PublicKey: extPubKey,
			Remove:    true,
		})

	}
	p := models.PeerAction{
		Action: models.RemovePeer,
		Peers:  peers,
	}

	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	err = publish(ingressHost, fmt.Sprintf("peer/host/%s/%s", ingressHost.ID.String(), servercfg.GetServer()), data)
	if err != nil {
		return err
	}
	return PublishFwUpdate(ingressHost, &models.FwAction{
		Action:  models.FwIngressDelExtClient,
		PeerKey: extclient.PublicKey,
	})
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
			logger.Log(0, "error retrieving host for dns update", host.ID.String(), err.Error())
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

func PublishFwUpdate(gwHost *models.Host, f *models.FwAction) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return publish(gwHost, fmt.Sprintf("fw/host/%s/%s", gwHost.ID.String(), servercfg.GetServer()), data)
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
			logger.Log(0, "error retrieving host for dns update", host.ID.String(), err.Error())
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
		logic.ResetPeerUpdateContext()
		for _, host := range hosts {
			host := host
			logger.Log(2, "sending scheduled peer update (5 min)")
			if err = PublishSingleHostPeerUpdate(logic.PeerUpdateCtx, &host, nil, nil); err != nil {
				logger.Log(1, "error publishing peer updates for host: ", host.ID.String(), " Err: ", err.Error())
			}
		}
	}
}
