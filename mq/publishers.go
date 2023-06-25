package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// PublishHostPeerUpdate --- determines and publishes a peer update to one host
func PublishHostPeerUpdate(host *models.Host, allNodes []models.Node) error {

	peerUpdate, err := logic.GetPeerUpdateForHost(host, allNodes)
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

// FlushNetworkPeersToHost - sends all the peers in the network to the host.
func FlushNetworkPeersToHost(client models.Client, networkClients []models.Client) error {
	logger.Log(0, "flushing network peers to host: ", client.Host.ID.String(), client.Node.Network)
	addPeerAction := models.PeerAction{
		Action: models.AddPeer,
		Peers:  []wgtypes.PeerConfig{},
	}
	rmPeerAction := models.PeerAction{
		Action: models.RemovePeer,
		Peers:  []wgtypes.PeerConfig{},
	}
	for _, clientI := range networkClients {
		clientI := clientI
		if clientI.Node.ID == client.Node.ID {
			// skip self
			continue
		}
		allowedIPs := logic.GetAllowedIPs(client, clientI)
		peerCfg := wgtypes.PeerConfig{
			PublicKey: clientI.Host.PublicKey,
			Endpoint: &net.UDPAddr{
				IP:   clientI.Host.EndpointIP,
				Port: logic.GetPeerListenPort(&clientI.Host),
			},
			PersistentKeepaliveInterval: &clientI.Node.PersistentKeepalive,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  allowedIPs,
		}

		if len(peerCfg.AllowedIPs) == 0 || (client.Node.IsRelayed && (client.Node.RelayedBy != clientI.Node.ID.String())) {
			// remove peer if not allowed
			rmPeerAction.Peers = append(rmPeerAction.Peers, wgtypes.PeerConfig{
				PublicKey: clientI.Host.PublicKey,
				Remove:    true,
			})
			continue
		}

		addPeerAction.Peers = append(addPeerAction.Peers, peerCfg)
	}
	if client.Node.IsIngressGateway {
		extPeers, _, err := logic.GetExtPeers(&client.Node)
		if err == nil {
			addPeerAction.Peers = append(addPeerAction.Peers, extPeers...)
		}
	}
	if len(rmPeerAction.Peers) > 0 {
		data, err := json.Marshal(rmPeerAction)
		if err != nil {
			return err
		}
		publish(&client.Host, fmt.Sprintf("peer/host/%s/%s", client.Host.ID.String(), servercfg.GetServer()), data)
	}
	if len(addPeerAction.Peers) > 0 {
		data, err := json.Marshal(addPeerAction)
		if err != nil {
			return err
		}
		publish(&client.Host, fmt.Sprintf("peer/host/%s/%s", client.Host.ID.String(), servercfg.GetServer()), data)
	}
	// send fw update if gw host
	if client.Node.IsIngressGateway || client.Node.IsEgressGateway {
		f, err := logic.GetFwUpdate(&client.Host)
		if err == nil {
			PublishFwUpdate(&client.Host, &f)
		}

	}
	return nil
}

// BroadcastDelPeer - notifys all the hosts in the network to remove peer
func BroadcastDelPeer(host *models.Host, networkClients []models.Client) error {

	p := models.PeerAction{
		Action: models.RemovePeer,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey: host.PublicKey,
				Endpoint: &net.UDPAddr{
					IP:   host.EndpointIP,
					Port: logic.GetPeerListenPort(host),
				},
				ReplaceAllowedIPs: true,
				UpdateOnly:        true,
				Remove:            true,
			},
		},
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	for _, clientI := range networkClients {
		if clientI.Host.ID == host.ID {
			// skip self...
			continue
		}
		allowedIPs := logic.GetAllowedIPs(clientI, models.Client{Host: *host})
		if len(allowedIPs) != 0 {
			p.Peers[0].Remove = false
			p.Peers[0].AllowedIPs = allowedIPs

		}
		publish(&clientI.Host, fmt.Sprintf("peer/host/%s/%s", clientI.Host.ID.String(), servercfg.GetServer()), data)
		if clientI.Node.IsIngressGateway || clientI.Node.IsEgressGateway {
			go func(peerHost models.Host) {
				f, err := logic.GetFwUpdate(&peerHost)
				if err == nil {
					PublishFwUpdate(&peerHost, &f)
				}
			}(clientI.Host)
		}

	}
	return nil
}

// BroadcastAclUpdate - sends new acl updates to peers
func BroadcastAclUpdate(network string) error {
	clients, err := logic.GetNetworkClients(network)
	if err != nil {
		return err
	}
	for _, client := range clients {
		client := client
		go FlushNetworkPeersToHost(client, clients)
	}
	return err
}

// BroadcastHostUpdate - notifys the hosts in the network to update peer.
func BroadcastHostUpdate(host *models.Host, remove bool) error {

	p := models.PeerAction{
		Action: models.UpdatePeer,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey: host.PublicKey,
				Endpoint: &net.UDPAddr{
					IP:   host.EndpointIP,
					Port: logic.GetPeerListenPort(host),
				},
				ReplaceAllowedIPs: true,
				Remove:            remove,
			},
		},
	}
	if remove {
		p.Action = models.RemovePeer
	}
	peerHosts := logic.GetRelatedHosts(host.ID.String())
	for _, peerHost := range peerHosts {
		if !remove {
			p.Peers[0].AllowedIPs = logic.GetAllowedIPs(models.Client{Host: peerHost}, models.Client{Host: *host})
		}
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		publish(&peerHost, fmt.Sprintf("peer/host/%s/%s", peerHost.ID.String(), servercfg.GetServer()), data)
	}
	return nil
}

// BroadcastAddOrUpdateNetworkPeer - notifys the hosts in the network to add or update peer.
func BroadcastAddOrUpdateNetworkPeer(client models.Client, update bool) error {
	clients, err := logic.GetNetworkClients(client.Node.Network)
	if err != nil {
		return err
	}

	p := models.PeerAction{
		Action: models.AddPeer,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey: client.Host.PublicKey,
				Endpoint: &net.UDPAddr{
					IP:   client.Host.EndpointIP,
					Port: logic.GetPeerListenPort(&client.Host),
				},
				PersistentKeepaliveInterval: &client.Node.PersistentKeepalive,
				ReplaceAllowedIPs:           true,
			},
		},
	}
	var relayPeerCfg models.PeerAction
	var relayClient models.Client
	if client.Node.IsRelayed {
		relayNode, err := logic.GetNodeByID(client.Node.RelayedBy)
		if err != nil {
			return err
		}
		relayHost, err := logic.GetHost(relayNode.HostID.String())
		if err != nil {
			return err
		}
		relayClient = models.Client{
			Host: *relayHost,
			Node: relayNode,
		}
		relayPeerCfg = models.PeerAction{
			Action: models.AddPeer,
			Peers: []wgtypes.PeerConfig{
				{
					PublicKey: relayHost.PublicKey,
					Endpoint: &net.UDPAddr{
						IP:   relayHost.EndpointIP,
						Port: logic.GetPeerListenPort(relayHost),
					},
					PersistentKeepaliveInterval: &relayNode.PersistentKeepalive,
					ReplaceAllowedIPs:           true,
				},
			},
		}

	}
	if update {
		p.Action = models.UpdatePeer
		relayPeerCfg.Action = models.UpdatePeer
	}
	for _, clientI := range clients {
		clientI := clientI
		if clientI.Node.ID.String() == client.Node.ID.String() {
			// skip self...
			continue
		}
		// update allowed ips, according to the peer node
		p.Peers[0].AllowedIPs = logic.GetAllowedIPs(clientI, client)
		if client.Node.IsRelayed {
			relayPeerCfg.Peers[0].AllowedIPs = logic.GetAllowedIPs(clientI, relayClient)
		}
		if update && len(p.Peers[0].AllowedIPs) == 0 {
			// remove peer
			p.Action = models.RemovePeer
			p.Peers[0].Remove = true

		}
		peerHost, err := logic.GetHost(clientI.Host.ID.String())
		if err != nil {
			continue
		}

		if clientI.Node.IsRelayed {
			r := models.PeerAction{
				Action: models.AddPeer,
			}
			// update the relay peer on this node
			relayNode, err := logic.GetNodeByID(clientI.Node.RelayedBy)
			if err != nil {
				continue
			}
			relayHost, err := logic.GetHost(relayNode.HostID.String())
			if err != nil {
				continue
			}
			relayedClient := models.Client{
				Host: *peerHost,
				Node: clientI.Node,
			}
			relayClient := models.Client{
				Host: *relayHost,
				Node: relayNode,
			}
			rPeerCfg := logic.GetPeerConfForRelayed(relayedClient, relayClient)
			if update {
				r.Action = models.UpdatePeer
			}
			r.Peers = append(r.Peers, rPeerCfg)
			data, err := json.Marshal(r)
			if err != nil {
				continue
			}
			publish(peerHost, fmt.Sprintf("peer/host/%s/%s", peerHost.ID.String(), servercfg.GetServer()), data)

		} else {
			var data []byte
			if client.Node.IsRelayed && client.Node.RelayedBy != clientI.Node.ID.String() {
				data, err = json.Marshal(relayPeerCfg)
				if err != nil {
					continue
				}
			} else {
				data, err = json.Marshal(p)
				if err != nil {
					continue
				}
			}

			publish(peerHost, fmt.Sprintf("peer/host/%s/%s", peerHost.ID.String(), servercfg.GetServer()), data)
		}
		if clientI.Node.IsIngressGateway || clientI.Node.IsEgressGateway {
			go func(peerHost models.Host) {
				f, err := logic.GetFwUpdate(&peerHost)
				if err == nil {
					PublishFwUpdate(&peerHost, &f)
				}
			}(*peerHost)
		}

	}
	return nil
}

// BroadcastExtClient - publishes msg to add/updates ext client in the network
func BroadcastExtClient(ingressClient models.Client) error {

	clients, err := logic.GetNetworkClients(ingressClient.Node.Network)
	if err != nil {
		return err
	}
	//flush peers to ingress host
	go FlushNetworkPeersToHost(ingressClient, clients)
	// broadcast to update ingress peer to other hosts
	go BroadcastAddOrUpdateNetworkPeer(ingressClient, true)
	return nil
}

// BroadcastDelExtClient - published msg to remove ext client from network
func BroadcastDelExtClient(ingressClient models.Client, extclients []models.ExtClient) error {
	go BroadcastAddOrUpdateNetworkPeer(ingressClient, true)
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
	err = publish(&ingressClient.Host, fmt.Sprintf("peer/host/%s/%s", ingressClient.Host.ID.String(), servercfg.GetServer()), data)
	if err != nil {
		return err
	}
	return nil
}

// NodeUpdate -- publishes a node update
func NodeUpdate(node *models.Node) error {
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
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
	logger.Log(4, "publishing host update to "+hostUpdate.Host.ID.String())

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

// PublishFwUpdate - publishes fw update to host
func PublishFwUpdate(gwHost *models.Host, f *models.FwUpdate) error {
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
		allNodes, err := logic.GetAllNodes()
		if err != nil {
			return
		}
		for _, host := range hosts {
			host := host
			logger.Log(2, "sending scheduled peer update (5 min)")
			if err = PublishHostPeerUpdate(&host, allNodes); err != nil {
				logger.Log(1, "error publishing peer updates for host: ", host.ID.String(), " Err: ", err.Error())
			}
		}
	}
}
