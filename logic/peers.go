package logic

import (
	"errors"
	"net"
	"net/netip"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slices"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// NodePeersInfo - fetches node's peers with their ids and addrs.
func NodePeersInfo(client *models.Client) (models.NodePeersInfo, error) {
	nodePeersInfo := models.NodePeersInfo{
		PeerIDs: make(models.PeerMap),
		Peers:   []wgtypes.PeerConfig{},
	}
	nodes, err := GetNetworkNodes(client.Node.Network)
	if err != nil {
		return models.NodePeersInfo{}, err
	}

	for _, peer := range nodes {
		if peer.ID == client.Node.ID {
			continue
		}
		if peer.Action == models.NODE_DELETE || peer.PendingDelete || !peer.Connected ||
			!nodeacls.AreNodesAllowed(nodeacls.NetworkID(peer.Network), nodeacls.NodeID(client.Node.ID.String()), nodeacls.NodeID(peer.ID.String())) {
			continue
		}
		peerHost, err := GetHost(peer.HostID.String())
		if err != nil {
			continue
		}
		var peerConfig wgtypes.PeerConfig
		peerConfig.PublicKey = peerHost.PublicKey
		peerConfig.PersistentKeepaliveInterval = &peer.PersistentKeepalive
		peerConfig.ReplaceAllowedIPs = true
		uselocal := false
		if client.Host.EndpointIP.String() == peerHost.EndpointIP.String() {
			// peer is on same network
			// set to localaddress
			uselocal = true
			if client.Node.LocalAddress.IP == nil {
				// use public endpint
				uselocal = false
			}
			if client.Node.LocalAddress.String() == peer.LocalAddress.String() {
				uselocal = false
			}
		}
		peerConfig.Endpoint = &net.UDPAddr{
			IP:   peerHost.EndpointIP,
			Port: getPeerWgListenPort(peerHost),
		}

		if uselocal {
			peerConfig.Endpoint.IP = peer.LocalAddress.IP
			peerConfig.Endpoint.Port = peerHost.ListenPort
		}
		allowedips := GetNetworkAllowedIPs(*client, models.Client{Host: *peerHost, Node: peer})

		peerConfig.AllowedIPs = allowedips
		nodePeersInfo.Peers = append(nodePeersInfo.Peers, peerConfig)
		nodePeersInfo.PeerIDs[peerHost.PublicKey.String()] = models.IDandAddr{
			ID:         peer.ID.String(),
			Address:    peer.Address.IP.String(),
			Name:       peerHost.Name,
			Network:    peer.Network,
			ListenPort: GetPeerListenPort(peerHost),
		}
	}
	if client.Node.IsIngressGateway {
		extPeers, extPeerIDAndAddrs, err := GetExtPeers(&client.Node)
		if err == nil {
			nodePeersInfo.Peers = append(nodePeersInfo.Peers, extPeers...)
			for _, extPeerIdAndAddr := range extPeerIDAndAddrs {
				extPeerIdAndAddr := extPeerIdAndAddr
				nodePeersInfo.PeerIDs[extPeerIdAndAddr.ID] = extPeerIdAndAddr
			}
		} else if !database.IsEmptyRecord(err) {
			logger.Log(1, "error retrieving external clients:", err.Error())
		}
	}
	return nodePeersInfo, nil
}

// GetPeerUpdateForHost - gets the consolidated peer update for the host from all networks
func GetPeerUpdateForHost(host *models.Host) (models.HostPeerUpdate, error) {
	if host == nil {
		return models.HostPeerUpdate{}, errors.New("host is nil")
	}
	allNodes, err := GetAllNodes()
	if err != nil {
		return models.HostPeerUpdate{}, err
	}
	// track which nodes are deleted
	// after peer calculation, if peer not in list, add delete config of peer
	hostPeerUpdate := models.HostPeerUpdate{
		Host:            *host,
		Server:          servercfg.GetServer(),
		ServerVersion:   servercfg.GetVersion(),
		Peers:           []wgtypes.PeerConfig{},
		HostNetworkInfo: models.HostInfoMap{},
	}

	logger.Log(1, "peer update for host", host.ID.String())
	peerIndexMap := make(map[string]int)
	for _, nodeID := range host.Nodes {
		nodeID := nodeID
		node, err := GetNodeByID(nodeID)
		if err != nil {
			continue
		}
		if !node.Connected || node.PendingDelete || node.Action == models.NODE_DELETE {
			continue
		}
		currentPeers := GetNetworkNodesMemory(allNodes, node.Network)
		for _, peer := range currentPeers {
			peer := peer
			if peer.ID.String() == node.ID.String() {
				logger.Log(2, "peer update, skipping self")
				//skip yourself
				continue
			}
			peerHost, err := GetHost(peer.HostID.String())
			if err != nil {
				logger.Log(1, "no peer host", peer.HostID.String(), err.Error())
				return models.HostPeerUpdate{}, err
			}
			peerConfig := wgtypes.PeerConfig{
				PublicKey:                   peerHost.PublicKey,
				PersistentKeepaliveInterval: &peer.PersistentKeepalive,
				ReplaceAllowedIPs:           true,
			}
			if (node.IsRelayed && node.RelayedBy != peer.ID.String()) || (peer.IsRelayed && peer.RelayedBy != node.ID.String()) || ShouldRemovePeer(node, peer) {
				// if node is relayed and peer is not the relay, set remove to true
				peerConfig.Remove = true
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, peerConfig)
				peerIndexMap[peerHost.PublicKey.String()] = len(hostPeerUpdate.Peers) - 1
				continue
			}
			uselocal := false
			if host.EndpointIP.String() == peerHost.EndpointIP.String() {
				// peer is on same network
				// set to localaddress
				uselocal = true
				if node.LocalAddress.IP == nil {
					// use public endpint
					uselocal = false
				}
				if node.LocalAddress.String() == peer.LocalAddress.String() {
					uselocal = false
				}
			}
			peerConfig.Endpoint = &net.UDPAddr{
				IP:   peerHost.EndpointIP,
				Port: getPeerWgListenPort(peerHost),
			}

			if uselocal {
				peerConfig.Endpoint.IP = peer.LocalAddress.IP
				peerConfig.Endpoint.Port = peerHost.ListenPort
			}
			peerConfig.AllowedIPs = GetNetworkAllowedIPs(models.Client{Host: *host, Node: node}, models.Client{Host: *peerHost, Node: peer})

			if _, ok := peerIndexMap[peerHost.PublicKey.String()]; !ok {
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, peerConfig)
				peerIndexMap[peerHost.PublicKey.String()] = len(hostPeerUpdate.Peers) - 1
				hostPeerUpdate.HostNetworkInfo[peerHost.PublicKey.String()] = models.HostNetworkInfo{
					Interfaces: peerHost.Interfaces,
				}
			} else {
				peerAllowedIPs := hostPeerUpdate.Peers[peerIndexMap[peerHost.PublicKey.String()]].AllowedIPs
				peerAllowedIPs = append(peerAllowedIPs, peerConfig.AllowedIPs...)
				hostPeerUpdate.Peers[peerIndexMap[peerHost.PublicKey.String()]].Remove = false
				hostPeerUpdate.Peers[peerIndexMap[peerHost.PublicKey.String()]].AllowedIPs = peerAllowedIPs
			}

		}

		if node.IsIngressGateway {
			extPeers, _, err := GetExtPeers(&node)
			if err == nil {
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, extPeers...)
			} else if !database.IsEmptyRecord(err) {
				logger.Log(1, "error retrieving external clients:", err.Error())
			}
		}
	}

	return hostPeerUpdate, nil
}

func ShouldRemovePeer(node, peer models.Node) (remove bool) {
	if peer.Action == models.NODE_DELETE || peer.PendingDelete || !peer.Connected ||
		!nodeacls.AreNodesAllowed(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()), nodeacls.NodeID(peer.ID.String())) {
		remove = true
	}
	return
}

// GetFwUpdate - fetches the firewall update for the gateway nodes on the host
func GetFwUpdate(host *models.Host) (models.FwUpdate, error) {
	fwUpdate := models.FwUpdate{
		IngressInfo: models.IngressInfo{
			ExtPeers: make(map[string]models.ExtClientInfo),
		},
		EgressInfo: make(map[string]models.EgressInfo),
	}
	allNodes, err := GetAllNodes()
	if err != nil {
		return fwUpdate, err
	}
	for _, nodeID := range host.Nodes {
		nodeID := nodeID
		node, err := GetNodeByID(nodeID)
		if err != nil {
			continue
		}
		if !node.Connected || node.PendingDelete || node.Action == models.NODE_DELETE {
			continue
		}
		currentPeers := GetNetworkNodesMemory(allNodes, node.Network)
		var nodePeerMap map[string]models.PeerRouteInfo
		if node.IsIngressGateway || node.IsEgressGateway {
			nodePeerMap = make(map[string]models.PeerRouteInfo)
		}
		for _, peer := range currentPeers {
			peer := peer
			if peer.ID.String() == node.ID.String() {
				logger.Log(2, "fw update, skipping self")
				//skip yourself
				continue
			}
			peerHost, err := GetHost(peer.HostID.String())
			if err != nil {
				logger.Log(1, "no peer host", peer.HostID.String(), err.Error())
				continue
			}
			if node.IsIngressGateway || node.IsEgressGateway {
				if peer.IsIngressGateway {
					_, extPeerIDAndAddrs, err := GetExtPeers(&peer)
					if err == nil {
						for _, extPeerIdAndAddr := range extPeerIDAndAddrs {
							extPeerIdAndAddr := extPeerIdAndAddr
							nodePeerMap[extPeerIdAndAddr.ID] = models.PeerRouteInfo{
								PeerAddr: net.IPNet{
									IP:   net.ParseIP(extPeerIdAndAddr.Address),
									Mask: getCIDRMaskFromAddr(extPeerIdAndAddr.Address),
								},
								PeerKey: extPeerIdAndAddr.ID,
								Allow:   true,
								ID:      extPeerIdAndAddr.ID,
							}
						}
					}
				}
				if node.IsIngressGateway && peer.IsEgressGateway {
					fwUpdate.IngressInfo.EgressRanges = append(fwUpdate.IngressInfo.EgressRanges,
						peer.EgressGatewayRanges...)
				}
				nodePeerMap[peerHost.PublicKey.String()] = models.PeerRouteInfo{
					PeerAddr: net.IPNet{
						IP:   net.ParseIP(peer.PrimaryAddress()),
						Mask: getCIDRMaskFromAddr(peer.PrimaryAddress()),
					},
					PeerKey: peerHost.PublicKey.String(),
					Allow:   true,
					ID:      peer.ID.String(),
				}
			}
		}
		var extPeerIDAndAddrs []models.IDandAddr
		if node.IsIngressGateway {
			fwUpdate.IsIngressGw = true
			_, extPeerIDAndAddrs, err = GetExtPeers(&node)
			if err == nil {
				for _, extPeerIdAndAddr := range extPeerIDAndAddrs {
					extPeerIdAndAddr := extPeerIdAndAddr
					nodePeerMap[extPeerIdAndAddr.ID] = models.PeerRouteInfo{
						PeerAddr: net.IPNet{
							IP:   net.ParseIP(extPeerIdAndAddr.Address),
							Mask: getCIDRMaskFromAddr(extPeerIdAndAddr.Address),
						},
						PeerKey: extPeerIdAndAddr.ID,
						Allow:   true,
						ID:      extPeerIdAndAddr.ID,
					}
				}
				for _, extPeerIdAndAddr := range extPeerIDAndAddrs {
					extPeerIdAndAddr := extPeerIdAndAddr

					fwUpdate.IngressInfo.ExtPeers[extPeerIdAndAddr.ID] = models.ExtClientInfo{
						Masquerade: true,
						IngGwAddr: net.IPNet{
							IP:   net.ParseIP(node.PrimaryAddress()),
							Mask: getCIDRMaskFromAddr(node.PrimaryAddress()),
						},
						Network: node.PrimaryNetworkRange(),
						ExtPeerAddr: net.IPNet{
							IP:   net.ParseIP(extPeerIdAndAddr.Address),
							Mask: getCIDRMaskFromAddr(extPeerIdAndAddr.Address),
						},
						ExtPeerKey: extPeerIdAndAddr.ID,
						Peers:      filterNodeMapForClientACLs(extPeerIdAndAddr.ID, node.Network, nodePeerMap),
					}
				}
			} else if !database.IsEmptyRecord(err) {
				logger.Log(1, "error retrieving external clients:", err.Error())
			}
		}
		if node.IsEgressGateway {
			fwUpdate.IsEgressGw = true
			fwUpdate.EgressInfo[node.ID.String()] = models.EgressInfo{
				EgressID: node.ID.String(),
				Network:  node.PrimaryNetworkRange(),
				EgressGwAddr: net.IPNet{
					IP:   net.ParseIP(node.PrimaryAddress()),
					Mask: getCIDRMaskFromAddr(node.PrimaryAddress()),
				},
				GwPeers:     nodePeerMap,
				EgressGWCfg: node.EgressGatewayRequest,
			}
		}
	}
	return fwUpdate, nil
}

// getPeerWgListenPort - fetches the wg listen port for the host
func getPeerWgListenPort(host *models.Host) int {
	peerPort := host.ListenPort
	if host.WgPublicListenPort != 0 {
		peerPort = host.WgPublicListenPort
	}
	return peerPort
}

// GetPeerListenPort - given a host, retrieve it's appropriate listening port
func GetPeerListenPort(host *models.Host) int {
	peerPort := host.ListenPort
	if host.WgPublicListenPort != 0 {
		peerPort = host.WgPublicListenPort
	}
	return peerPort
}

func GetExtPeers(node *models.Node) ([]wgtypes.PeerConfig, []models.IDandAddr, error) {
	var peers []wgtypes.PeerConfig
	var idsAndAddr []models.IDandAddr
	extPeers, err := GetNetworkExtClients(node.Network)
	if err != nil {
		return peers, idsAndAddr, err
	}
	host, err := GetHost(node.HostID.String())
	if err != nil {
		return peers, idsAndAddr, err
	}
	for _, extPeer := range extPeers {
		extPeer := extPeer
		pubkey, err := wgtypes.ParseKey(extPeer.PublicKey)
		if err != nil {
			logger.Log(1, "error parsing ext pub key:", err.Error())
			continue
		}

		if host.PublicKey.String() == extPeer.PublicKey ||
			extPeer.IngressGatewayID != node.ID.String() || !extPeer.Enabled {
			continue
		}

		var allowedips []net.IPNet
		var peer wgtypes.PeerConfig
		if extPeer.Address != "" {
			var peeraddr = net.IPNet{
				IP:   net.ParseIP(extPeer.Address),
				Mask: net.CIDRMask(32, 32),
			}
			if peeraddr.IP != nil && peeraddr.Mask != nil {
				allowedips = append(allowedips, peeraddr)
			}
		}

		if extPeer.Address6 != "" {
			var addr6 = net.IPNet{
				IP:   net.ParseIP(extPeer.Address6),
				Mask: net.CIDRMask(128, 128),
			}
			if addr6.IP != nil && addr6.Mask != nil {
				allowedips = append(allowedips, addr6)
			}
		}

		primaryAddr := extPeer.Address
		if primaryAddr == "" {
			primaryAddr = extPeer.Address6
		}
		peer = wgtypes.PeerConfig{
			PublicKey:         pubkey,
			ReplaceAllowedIPs: true,
			AllowedIPs:        allowedips,
		}
		peers = append(peers, peer)
		idsAndAddr = append(idsAndAddr, models.IDandAddr{
			ID:          peer.PublicKey.String(),
			Name:        extPeer.ClientID,
			Address:     primaryAddr,
			IsExtclient: true,
		})
	}
	return peers, idsAndAddr, nil

}

func getNodeByNetworkFromHost(h *models.Host, network string) *models.Node {
	for _, nodeID := range h.Nodes {
		node, err := GetNodeByID(nodeID)
		if err == nil && node.Network == network {
			return &node
		}
	}
	return nil
}

// GetAllowedIPs - calculates the wireguard allowedip field for a peer of a node based on the peer and node settings
func GetAllowedIPs(client, peer models.Client) []net.IPNet {
	var allowedips []net.IPNet
	for _, nodeID := range peer.Host.Nodes {
		node, err := GetNodeByID(nodeID)
		if err != nil {
			continue
		}
		clientNode := getNodeByNetworkFromHost(&client.Host, node.Network)
		if clientNode == nil {
			continue
		}
		client.Node = *clientNode
		peer.Node = node
		if ShouldRemovePeer(*clientNode, peer.Node) {
			continue
		}
		if peer.Node.Address.IP != nil {
			allowed := net.IPNet{
				IP:   peer.Node.Address.IP,
				Mask: net.CIDRMask(32, 32),
			}
			allowedips = append(allowedips, allowed)
		}
		if peer.Node.Address6.IP != nil {
			allowed := net.IPNet{
				IP:   peer.Node.Address6.IP,
				Mask: net.CIDRMask(128, 128),
			}
			allowedips = append(allowedips, allowed)
		}
		// handle egress gateway peers
		if peer.Node.IsEgressGateway {
			allowedips = append(allowedips, getEgressIPs(peer)...)
		}
		if peer.Node.IsRelay {
			allowedips = append(allowedips, getRelayAllowedIPs(client, peer)...)
		}
		// handle ingress gateway peers
		if peer.Node.IsIngressGateway {
			allowedips = append(allowedips, getIngressIPs(peer)...)
		}
	}

	return allowedips
}

// GetNetworkAllowedIPs - calculates the wireguard allowedip field for a peer of a node based on the peer and node settings
func GetNetworkAllowedIPs(client, peer models.Client) []net.IPNet {
	var allowedips []net.IPNet
	if peer.Node.Address.IP != nil {
		allowed := net.IPNet{
			IP:   peer.Node.Address.IP,
			Mask: net.CIDRMask(32, 32),
		}
		allowedips = append(allowedips, allowed)
	}
	if peer.Node.Address6.IP != nil {
		allowed := net.IPNet{
			IP:   peer.Node.Address6.IP,
			Mask: net.CIDRMask(128, 128),
		}
		allowedips = append(allowedips, allowed)
	}
	// handle egress gateway peers
	if peer.Node.IsEgressGateway {
		egressIPs := getEgressIPs(peer)
		allowedips = append(allowedips, egressIPs...)

	}
	if peer.Node.IsRelay {
		allowedips = append(allowedips, getRelayAllowedIPs(client, peer)...)
	}
	// handle ingress gateway peers
	if peer.Node.IsIngressGateway {
		allowedips = append(allowedips, getIngressIPs(peer)...)
	}
	return allowedips
}

// getEgressIPs - gets the egress IPs for a client
func getEgressIPs(client models.Client) []net.IPNet {

	//check for internet gateway
	internetGateway := false
	if slices.Contains(client.Node.EgressGatewayRanges, "0.0.0.0/0") || slices.Contains(client.Node.EgressGatewayRanges, "::/0") {
		internetGateway = true
	}
	allowedips := []net.IPNet{}
	for _, iprange := range client.Node.EgressGatewayRanges { // go through each cidr for egress gateway
		ip, cidr, err := net.ParseCIDR(iprange) // confirming it's valid cidr
		if err != nil {
			logger.Log(1, "could not parse gateway IP range. Not adding ", iprange)
			continue // if can't parse CIDR
		}
		cidr.IP = ip
		// getting the public ip of node
		if cidr.Contains(client.Host.EndpointIP) && !internetGateway { // ensuring egress gateway range does not contain endpoint of node
			logger.Log(2, "egress IP range of ", iprange, " overlaps with ", client.Host.EndpointIP.String(), ", omitting")
			continue // skip adding egress range if overlaps with node's ip
		}
		// TODO: Could put in a lot of great logic to avoid conflicts / bad routes
		if cidr.Contains(client.Node.LocalAddress.IP) && !internetGateway { // ensuring egress gateway range does not contain public ip of node
			logger.Log(2, "egress IP range of ", iprange, " overlaps with ", client.Node.LocalAddress.String(), ", omitting")
			continue // skip adding egress range if overlaps with node's local ip
		}
		if err != nil {
			logger.Log(1, "error encountered when setting egress range", err.Error())
		} else {
			allowedips = append(allowedips, *cidr)
		}
	}
	return allowedips
}

func getCIDRMaskFromAddr(addr string) net.IPMask {
	cidr := net.CIDRMask(32, 32)
	ipAddr, err := netip.ParseAddr(addr)
	if err != nil {
		return cidr
	}
	if ipAddr.Is6() {
		cidr = net.CIDRMask(128, 128)
	}
	return cidr
}

// accounts for ext client ACLs
func filterNodeMapForClientACLs(publicKey, network string, nodePeerMap map[string]models.PeerRouteInfo) map[string]models.PeerRouteInfo {
	if !isEE {
		return nodePeerMap
	}
	if nodePeerMap == nil {
		return map[string]models.PeerRouteInfo{}
	}

	if len(publicKey) == 0 || len(network) == 0 {
		return nodePeerMap
	}

	client, err := GetExtClientByPubKey(publicKey, network)
	if err != nil {
		return nodePeerMap
	}
	for k := range nodePeerMap {
		currNodePeer := nodePeerMap[k]
		if _, ok := client.ACLs[currNodePeer.ID]; ok {
			delete(nodePeerMap, k)
		}
	}
	return nodePeerMap
}

// getRelayAllowedIPs returns the list of allowedips for a peer that is a relay
func getRelayAllowedIPs(client, relayPeer models.Client) []net.IPNet {
	var relayIPs []net.IPNet
	if !relayPeer.Node.IsRelay {
		logger.Log(0, "getRelayAllowedIPs called for a non-relay node", relayPeer.Host.Name)
		return relayIPs
	}
	for _, relayed := range relayPeer.Node.RelayedNodes {
		relayedNode, err := GetNodeByID(relayed)
		if err != nil {
			logger.Log(0, "retrieve relayed node", err.Error())
			continue
		}
		if relayedNode.ID == client.Node.ID {
			relayIPs = append(relayIPs, getAllowedIpsForRelayed(client, relayPeer)...)
			continue
		}
		if relayedNode.Address.IP != nil {
			relayedNode.Address.Mask = net.CIDRMask(32, 32)
			relayIPs = append(relayIPs, relayedNode.Address)
		}
		if relayedNode.Address6.IP != nil {
			relayedNode.Address.Mask = net.CIDRMask(128, 128)
			relayIPs = append(relayIPs, relayedNode.Address6)
		}
		if relayedNode.IsEgressGateway {
			relayedHost, err := GetHost(relayedNode.HostID.String())
			if err == nil {
				relayIPs = append(relayIPs, getEgressIPs(models.Client{
					Host: *relayedHost,
					Node: relayedNode,
				})...)
			}

		}

	}
	return relayIPs
}

// getAllowedIpsForRelayed - returns the peerConfig for a node relayed by relay
func getAllowedIpsForRelayed(relayed, relay models.Client) (allowedIPs []net.IPNet) {
	if relayed.Node.RelayedBy != relay.Node.ID.String() {
		logger.Log(0, "RelayedByRelay called with invalid parameters")
		return
	}
	peers, err := GetNetworkClients(relay.Node.Network)
	if err != nil {
		logger.Log(0, "error getting network clients", err.Error())
		return
	}
	for _, peer := range peers {
		if peer.Node.ID == relayed.Node.ID || peer.Node.ID == relay.Node.ID {
			continue
		}
		if nodeacls.AreNodesAllowed(nodeacls.NetworkID(relayed.Node.Network), nodeacls.NodeID(relayed.Node.ID.String()), nodeacls.NodeID(peer.Node.ID.String())) {
			allowedIPs = append(allowedIPs, GetAllowedIPs(relayed, peer)...)
		}
	}
	return
}

// getIngressIPs returns the additional allowedips (ext client addresses) that need
// to be included for an ingress gateway peer
// TODO:  add ExtraAllowedIPs
func getIngressIPs(peer models.Client) []net.IPNet {
	var ingressIPs []net.IPNet
	extclients, err := GetNetworkExtClients(peer.Node.Network)
	if err != nil {
		return ingressIPs
	}
	for _, ec := range extclients {
		if ec.IngressGatewayID == peer.Node.ID.String() {
			if ec.Address != "" {
				var peeraddr = net.IPNet{
					IP:   net.ParseIP(ec.Address),
					Mask: net.CIDRMask(32, 32),
				}
				if peeraddr.IP != nil && peeraddr.Mask != nil {
					ingressIPs = append(ingressIPs, peeraddr)
				}
			}

			if ec.Address6 != "" {
				var addr6 = net.IPNet{
					IP:   net.ParseIP(ec.Address6),
					Mask: net.CIDRMask(128, 128),
				}
				if addr6.IP != nil && addr6.Mask != nil {
					ingressIPs = append(ingressIPs, addr6)
				}
			}
		}
	}
	return ingressIPs
}
