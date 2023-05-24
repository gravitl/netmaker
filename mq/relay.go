package mq

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// PubPeeUpdate publishes a peer update to the client
// relay is set to a newly created relay node or nil for otherr peer updates
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
			update.AllowedIPs = append(update.AllowedIPs, getAllowedIPs(peer)...)
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

// getAllowedIPs returns the list of allowedips for a given peer
func getAllowedIPs(peer models.Client) []net.IPNet {
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
			relayIPs = append(relayIPs, getAllowedIPs(peer)...)
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

// getEgressIPs returns the additional allowedips (egress ranges) that need
// to be included for an egress gateway peer
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

// getIngressIPs returns the additional allowedips (ext client addresses) that need
// to be included for an ingress gateway peer
// TODO:  add ExtraAllowedIPs
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

// pubRelayedUpdate - publish peer update to a node (client) that is relayed by the relay
func pubRelayedUpdate(client, relay *models.Client, peers *[]models.Client) {
	//verify
	if !logic.StringSliceContains(relay.Node.RelayedNodes, client.Node.ID.String()) {
		logger.Log(0, "invalid call to pubRelayed update", client.Host.Name, relay.Host.Name)
		return
	}
	//remove all nodes except relay
	p := models.PeerAction{
		Action: models.RemovePeer,
	}
	for _, peer := range *peers {
		if peer.Host.ID == relay.Host.ID || peer.Host.ID == client.Host.ID {
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
	for _, peer := range *peers {
		if peer.Host.ID == relay.Host.ID || peer.Host.ID == client.Host.ID {
			continue
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
			update.AllowedIPs = append(update.AllowedIPs, getAllowedIPs(peer)...)
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

// pubRelayUpdate - publish peer update to a relay
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
			update.AllowedIPs = append(update.AllowedIPs, getAllowedIPs(peer)...)
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
