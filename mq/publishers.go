package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
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

func BroadCastDelPeer(host *models.Host, network string) error {
	//relatedHosts := logic.GetRelatedHosts(host.ID.String())
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
		peerHost, err := logic.GetHost(nodeI.HostID.String())
		if err == nil {
			publish(peerHost, fmt.Sprintf("peer/host/%s/%s", host.ID.String(), servercfg.GetServer()), data)
		}
	}
	return nil
}

func BroadCastAddPeer(host *models.Host, node *models.Node, network string, update bool) error {
	nodes, err := logic.GetNetworkNodes(network)
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
		// update allowed ips, according to the peer node
		p.Peers[0].AllowedIPs = logic.GetAllowedIPs(&nodeI, node, nil)
		data, err := json.Marshal(p)
		if err != nil {
			continue
		}
		peerHost, err := logic.GetHost(nodeI.HostID.String())
		if err == nil {
			publish(peerHost, fmt.Sprintf("peer/host/%s/%s", host.ID.String(), servercfg.GetServer()), data)
		}
	}
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

func PubPeerUpdate(client, relay *models.Client, peers *[]models.Client) {
	fmt.Println("calculating peer update for", client.Host.Name, " with relay ")
	if relay != nil {
		fmt.Println(relay.Host.Name, " with relayed nodes ", relay.Node.RelayedNodes)
	} else {
		fmt.Println("no relay")
	}

	p := models.PeerAction{
		Action: models.UpdatePeer,
	}
	if client.Node.IsRelay {
		pubRelayUpdate(client, peers)
		return
	}
	if relay != nil {
		if logic.StringSliceContains(relay.Node.RelayedNodes, client.Node.ID.String()) {
			pubRelayedUpdate(client, relay, peers)
			return
		}
	}
	for _, peer := range *peers {
		fmt.Println("peer: ", peer.Host.Name)
		if client.Host.ID == peer.Host.ID {
			continue
		}
		update := wgtypes.PeerConfig{
			PublicKey:         peer.Host.PublicKey,
			ReplaceAllowedIPs: true,
			Endpoint: &net.UDPAddr{
				IP:   peer.Host.EndpointIP,
				Port: peer.Host.ListenPort,
			},
			PersistentKeepaliveInterval: &peer.Node.PersistentKeepalive,
		}
		if peer.Node.IsRelay {
			fmt.Println("processing relay peer")
			update.AllowedIPs = append(update.AllowedIPs, getRelayIPs(peer)...)
			fmt.Println("adding relay ips", update.AllowedIPs)
		}
		if relay != nil {
			if peer.Node.IsRelayed && peer.Node.RelayedBy == relay.Node.ID.String() {
				fmt.Println("removing relayed peer", peer.Host.Name, " from ", client.Host.Name)
				update.Remove = true
			}
		}
		if peer.Node.Address.IP != nil {
			peer.Node.Address.Mask = net.CIDRMask(32, 32)
			update.AllowedIPs = append(update.AllowedIPs, peer.Node.Address)
		}
		if peer.Node.Address6.IP != nil {
			peer.Node.Address.Mask = net.CIDRMask(128, 128)
			update.AllowedIPs = append(update.AllowedIPs, peer.Node.Address6)
		}
		if peer.Node.IsEgressGateway {
			update.AllowedIPs = append(update.AllowedIPs, getEgressIPs(peer)...)
		}
		if peer.Node.IsIngressGateway {
			update.AllowedIPs = append(update.AllowedIPs, getIngressIPs(peer)...)
		}
		p.Peers = append(p.Peers, update)
		fmt.Println("update: ", update)

	}
	data, err := json.Marshal(p)
	if err != nil {
		logger.Log(0, "marshal peer update", err.Error())
		return
	}
	fmt.Println("publishing peer update", client.Host.Name, p.Action, len(data))
	publish(&client.Host, fmt.Sprintf("peer/host/%s/%s", client.Host.ID.String(), servercfg.GetServer()), data)
}

func getRelayIPs(peer models.Client) []net.IPNet {
	var relayIPs []net.IPNet
	for _, relayed := range peer.Node.RelayedNodes {
		node, err := logic.GetNodeByID(relayed)
		if err != nil {
			logger.Log(0, "retrieve relayed node", err.Error())
			continue
		}
		if node.Address.IP != nil {
			node.Address.Mask = net.CIDRMask(32, 32)
			relayIPs = append(relayIPs, node.Address)
		}
		if node.Address6.IP != nil {
			node.Address.Mask = net.CIDRMask(128, 128)
			relayIPs = append(relayIPs, node.Address6)
		}
		if node.IsRelay {
			relayIPs = append(relayIPs, getRelayIPs(peer)...)
		}
		if node.IsEgressGateway {
			relayIPs = append(relayIPs, getEgressIPs(peer)...)
		}
		if node.IsIngressGateway {
			relayIPs = append(relayIPs, getIngressIPs(peer)...)
		}

	}
	return relayIPs
}

func getEgressIPs(peer models.Client) []net.IPNet {
	var egressIPs []net.IPNet
	for _, egressRange := range peer.Node.EgressGatewayRanges {
		ip, cidr, err := net.ParseCIDR(egressRange)
		if err != nil {
			logger.Log(0, "parse egress range", err.Error())
			continue
		}
		cidr.IP = ip
		egressIPs = append(egressIPs, *cidr)
	}
	return egressIPs
}

func getIngressIPs(peer models.Client) []net.IPNet {
	var ingressIPs []net.IPNet
	extclients, err := logic.GetNetworkExtClients(peer.Node.Network)
	if err != nil {
		return ingressIPs
	}
	for _, ec := range extclients {
		if ec.IngressGatewayID == peer.Node.ID.String() {
			if ec.Address != "" {
				ip, cidr, err := net.ParseCIDR(ec.Address)
				if err != nil {
					continue
				}
				cidr.IP = ip
				ingressIPs = append(ingressIPs, *cidr)
			}
			if ec.Address6 != "" {
				ip, cidr, err := net.ParseCIDR(ec.Address6)
				if err != nil {
					continue
				}
				cidr.IP = ip
				ingressIPs = append(ingressIPs, *cidr)
			}
		}
	}
	return ingressIPs
}

// publish peer update to a node (client) that is relayed by the relay
func pubRelayedUpdate(client, relay *models.Client, peers *[]models.Client) {
	log.Println("pubRelayedUpdate", client.Host.Name, relay.Host.Name, len(*peers))
	//verify
	if !logic.StringSliceContains(relay.Node.RelayedNodes, client.Node.ID.String()) {
		logger.Log(0, "invalid call to pubRelayed update", client.Host.Name, relay.Host.Name)
		return
	}
	//remove all nodes except relay
	p := models.PeerAction{
		Action: models.RemovePeer,
	}
	log.Println("removing peers ")
	for _, peer := range *peers {
		if peer.Host.ID == relay.Host.ID || peer.Host.ID == client.Host.ID {
			log.Println("skipping removal of ", peer.Host.Name)
			continue
		}
		update := wgtypes.PeerConfig{
			PublicKey: peer.Host.PublicKey,
			Remove:    true,
		}
		p.Peers = append(p.Peers, update)
	}
	data, err := json.Marshal(p)
	if err != nil {
		logger.Log(0, "marshal peer update", err.Error())
		return
	}
	fmt.Println("publishing peer update", client.Host.Name, p.Action, len(data))
	publish(&client.Host, fmt.Sprintf("peer/host/%s/%s", client.Host.ID.String(), servercfg.GetServer()), data)

	//update the relay peer
	p = models.PeerAction{
		Action: models.UpdatePeer,
	}
	update := wgtypes.PeerConfig{
		PublicKey:         relay.Host.PublicKey,
		ReplaceAllowedIPs: true,
		Endpoint: &net.UDPAddr{
			IP:   relay.Host.EndpointIP,
			Port: relay.Host.ListenPort,
		},
		PersistentKeepaliveInterval: &relay.Node.PersistentKeepalive,
	}
	if relay.Node.Address.IP != nil {
		relay.Node.Address.Mask = net.CIDRMask(32, 32)
		update.AllowedIPs = append(update.AllowedIPs, relay.Node.Address)
	}
	if relay.Node.Address6.IP != nil {
		relay.Node.Address6.Mask = net.CIDRMask(128, 128)
		update.AllowedIPs = append(update.AllowedIPs, relay.Node.Address6)
	}
	p.Peers = append(p.Peers, update)
	// add all other peers to allowed ips
	log.Println("adding peers to allowed ips")
	for _, peer := range *peers {
		if peer.Host.ID == relay.Host.ID || peer.Host.ID == client.Host.ID {
			log.Println("skipping ", peer.Host.Name, "in allowedips")
			continue
		}
		log.Println("adding ", peer.Host.Name, peer.Node.Address, "to allowedips")
		if peer.Node.Address.IP != nil {
			peer.Node.Address.Mask = net.CIDRMask(32, 32)
			update.AllowedIPs = append(update.AllowedIPs, peer.Node.Address)
		}
		if peer.Node.Address6.IP != nil {
			peer.Node.Address6.Mask = net.CIDRMask(128, 128)
			update.AllowedIPs = append(update.AllowedIPs, peer.Node.Address6)
		}
		if peer.Node.IsRelay {
			update.AllowedIPs = append(update.AllowedIPs, getRelayIPs(peer)...)
		}
		if peer.Node.IsEgressGateway {
			update.AllowedIPs = append(update.AllowedIPs, getEgressIPs(peer)...)
		}
		if peer.Node.IsIngressGateway {
			update.AllowedIPs = append(update.AllowedIPs, getIngressIPs(peer)...)
		}
	}
	p.Peers = append(p.Peers, update)
	data, err = json.Marshal(p)
	if err != nil {
		logger.Log(0, "marshal peer update", err.Error())
		return
	}
	fmt.Println("publishing peer update", client.Host.Name, p.Action, len(data))
	publish(&client.Host, fmt.Sprintf("peer/host/%s/%s", client.Host.ID.String(), servercfg.GetServer()), data)
}

func pubRelayUpdate(client *models.Client, peers *[]models.Client) {
	if !client.Node.IsRelay {
		return
	}
	// add all peers to allowedips
	p := models.PeerAction{
		Action: models.UpdatePeer,
	}
	for _, peer := range *peers {
		if peer.Host.ID == client.Host.ID {
			continue
		}
		update := wgtypes.PeerConfig{
			PublicKey:         peer.Host.PublicKey,
			ReplaceAllowedIPs: true,
			Remove:            false,
			Endpoint: &net.UDPAddr{
				IP:   peer.Host.EndpointIP,
				Port: peer.Host.ListenPort,
			},
			PersistentKeepaliveInterval: &peer.Node.PersistentKeepalive,
		}
		if peer.Node.Address.IP != nil {
			peer.Node.Address.Mask = net.CIDRMask(32, 32)
			update.AllowedIPs = append(update.AllowedIPs, peer.Node.Address)
		}
		if peer.Node.Address6.IP != nil {
			peer.Node.Address6.Mask = net.CIDRMask(128, 128)
			update.AllowedIPs = append(update.AllowedIPs, peer.Node.Address6)
		}
		if peer.Node.IsRelay {
			update.AllowedIPs = append(update.AllowedIPs, getRelayIPs(peer)...)
		}
		if peer.Node.IsEgressGateway {
			update.AllowedIPs = append(update.AllowedIPs, getEgressIPs(peer)...)
		}
		if peer.Node.IsIngressGateway {
			update.AllowedIPs = append(update.AllowedIPs, getIngressIPs(peer)...)
		}
		p.Peers = append(p.Peers, update)
	}
	data, err := json.Marshal(p)
	if err != nil {
		logger.Log(0, "marshal peer update", err.Error())
		return
	}
	fmt.Println("publishing peer update", client.Host.Name, p.Action, len(data))
	publish(&client.Host, fmt.Sprintf("peer/host/%s/%s", client.Host.ID.String(), servercfg.GetServer()), data)
}
