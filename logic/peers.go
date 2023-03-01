package logic

import (
	"errors"
	"fmt"
	"log"
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

// GetProxyUpdateForHost - gets the proxy update for host
func GetProxyUpdateForHost(host *models.Host) (models.ProxyManagerPayload, error) {
	proxyPayload := models.ProxyManagerPayload{
		Action: models.ProxyUpdate,
	}
	peerConfMap := make(map[string]models.PeerConf)
	if host.IsRelayed {
		relayHost, err := GetHost(host.RelayedBy)
		if err == nil {
			relayEndpoint, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", relayHost.EndpointIP, GetPeerListenPort(relayHost)))
			if err != nil {
				logger.Log(1, "failed to resolve relay node endpoint: ", err.Error())
			}
			proxyPayload.IsRelayed = true
			proxyPayload.RelayedTo = relayEndpoint
		} else {
			logger.Log(0, "couldn't find relay host for:  ", host.ID.String())
		}
	}
	if host.IsRelay {
		relayedHosts := GetRelayedHosts(host)
		relayPeersMap := make(map[string]models.RelayedConf)
		for _, relayedHost := range relayedHosts {
			relayedHost := relayedHost
			payload, err := GetPeerUpdateForHost("", &relayedHost, nil)
			if err == nil {
				relayedEndpoint, udpErr := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", relayedHost.EndpointIP, GetPeerListenPort(&relayedHost)))
				if udpErr == nil {
					relayPeersMap[relayedHost.PublicKey.String()] = models.RelayedConf{
						RelayedPeerEndpoint: relayedEndpoint,
						RelayedPeerPubKey:   relayedHost.PublicKey.String(),
						Peers:               payload.Peers,
					}
				}

			}
		}
		proxyPayload.IsRelay = true
		proxyPayload.RelayedPeerConf = relayPeersMap

	}
	var ingressStatus bool
	for _, nodeID := range host.Nodes {

		node, err := GetNodeByID(nodeID)
		if err != nil {
			continue
		}
		currentPeers, err := GetNetworkNodes(node.Network)
		if err != nil {
			continue
		}
		for _, peer := range currentPeers {
			if peer.ID == node.ID {
				//skip yourself
				continue
			}
			peerHost, err := GetHost(peer.HostID.String())
			if err != nil {
				continue
			}
			var currPeerConf models.PeerConf
			var found bool
			if currPeerConf, found = peerConfMap[peerHost.PublicKey.String()]; !found {
				currPeerConf = models.PeerConf{
					Proxy:            peerHost.ProxyEnabled,
					PublicListenPort: int32(GetPeerListenPort(peerHost)),
				}
			}

			if peerHost.IsRelayed && peerHost.RelayedBy != host.ID.String() {
				relayHost, err := GetHost(peerHost.RelayedBy)
				if err == nil {
					relayTo, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", relayHost.EndpointIP, GetPeerListenPort(relayHost)))
					if err == nil {
						currPeerConf.IsRelayed = true
						currPeerConf.RelayedTo = relayTo
					}

				}
			}

			peerConfMap[peerHost.PublicKey.String()] = currPeerConf
		}
		if node.IsIngressGateway {
			ingressStatus = true
			_, peerConfMap, err = getExtPeersForProxy(&node, peerConfMap)
			if err == nil {

			} else if !database.IsEmptyRecord(err) {
				logger.Log(1, "error retrieving external clients:", err.Error())
			}
		}

	}
	proxyPayload.IsIngress = ingressStatus
	proxyPayload.PeerMap = peerConfMap
	return proxyPayload, nil
}

// GetPeerUpdateForHost - gets the consolidated peer update for the host from all networks
func GetPeerUpdateForHost(network string, host *models.Host, deletedNode *models.Node) (models.HostPeerUpdate, error) {
	if host == nil {
		return models.HostPeerUpdate{}, errors.New("host is nil")
	}
	// track which nodes are deleted
	// after peer calculation, if peer not in list, add delete config of peer
	hostPeerUpdate := models.HostPeerUpdate{
		Host:          *host,
		Server:        servercfg.GetServer(),
		HostPeerIDs:   make(models.HostPeerMap, 0),
		ServerVersion: servercfg.GetVersion(),
		ServerAddrs:   []models.ServerAddr{},
		IngressInfo: models.IngressInfo{
			ExtPeers: make(map[string]models.ExtClientInfo),
		},
		EgressInfo: make(map[string]models.EgressInfo),
		PeerIDs:    make(models.PeerMap, 0),
		Peers:      []wgtypes.PeerConfig{},
		NodePeers:  []wgtypes.PeerConfig{},
	}
	var deletedNodes = []models.Node{} // used to track deleted nodes
	if deletedNode != nil {
		deletedNodes = append(deletedNodes, *deletedNode)
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
		currentPeers, err := GetNetworkNodes(node.Network)
		if err != nil {
			log.Println("no network nodes")
			return models.HostPeerUpdate{}, err
		}
		var nodePeerMap map[string]models.PeerRouteInfo
		if node.IsIngressGateway || node.IsEgressGateway {
			nodePeerMap = make(map[string]models.PeerRouteInfo)
		}
		for _, peer := range currentPeers {
			peer := peer
			if peer.ID.String() == node.ID.String() {
				logger.Log(2, "peer update, skipping self")
				//skip yourself
				continue
			}
			if peer.Action == models.NODE_DELETE || peer.PendingDelete {
				deletedNodes = append(deletedNodes, peer) // track deleted node for peer update
				continue
			}
			var peerConfig wgtypes.PeerConfig
			peerHost, err := GetHost(peer.HostID.String())
			if err != nil {
				logger.Log(1, "no peer host", peer.HostID.String(), err.Error())
				return models.HostPeerUpdate{}, err
			}

			if !peer.Connected {
				logger.Log(2, "peer update, skipping unconnected node", peer.ID.String())
				//skip unconnected nodes
				continue
			}
			if !nodeacls.AreNodesAllowed(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()), nodeacls.NodeID(peer.ID.String())) {
				logger.Log(2, "peer update, skipping node for acl")
				//skip if not permitted by acl
				continue
			}
			peerConfig.PublicKey = peerHost.PublicKey
			peerConfig.PersistentKeepaliveInterval = &peer.PersistentKeepalive
			peerConfig.ReplaceAllowedIPs = true
			uselocal := false
			if host.EndpointIP.String() == peerHost.EndpointIP.String() {
				//peer is on same network
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
				Port: GetPeerListenPort(peerHost),
			}

			if uselocal {
				peerConfig.Endpoint.IP = peer.LocalAddress.IP
			}
			allowedips := GetAllowedIPs(&node, &peer, nil)
			if peer.IsIngressGateway {
				for _, entry := range peer.IngressGatewayRange {
					_, cidr, err := net.ParseCIDR(string(entry))
					if err == nil {
						allowedips = append(allowedips, *cidr)
					}
				}
			}
			if peer.IsEgressGateway {
				allowedips = append(allowedips, getEgressIPs(&node, &peer)...)
			}
			peerConfig.AllowedIPs = allowedips
			if node.IsIngressGateway || node.IsEgressGateway {
				if peer.IsIngressGateway {
					_, extPeerIDAndAddrs, err := getExtPeers(&peer)
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
							}
						}
					}
				}
				if node.IsIngressGateway && peer.IsEgressGateway {
					hostPeerUpdate.IngressInfo.EgressRanges = append(hostPeerUpdate.IngressInfo.EgressRanges,
						peer.EgressGatewayRanges...)
				}
				nodePeerMap[peerHost.PublicKey.String()] = models.PeerRouteInfo{
					PeerAddr: net.IPNet{
						IP:   net.ParseIP(peer.PrimaryAddress()),
						Mask: getCIDRMaskFromAddr(peer.PrimaryAddress()),
					},
					PeerKey: peerHost.PublicKey.String(),
					Allow:   true,
				}
			}

			var nodePeer wgtypes.PeerConfig
			if _, ok := hostPeerUpdate.HostPeerIDs[peerHost.PublicKey.String()]; !ok {
				hostPeerUpdate.HostPeerIDs[peerHost.PublicKey.String()] = make(map[string]models.IDandAddr)
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, peerConfig)
				peerIndexMap[peerHost.PublicKey.String()] = len(hostPeerUpdate.Peers) - 1
				hostPeerUpdate.HostPeerIDs[peerHost.PublicKey.String()][peer.ID.String()] = models.IDandAddr{
					ID:      peer.ID.String(),
					Address: peer.PrimaryAddress(),
					Name:    peerHost.Name,
					Network: peer.Network,
				}
				nodePeer = peerConfig
			} else {
				peerAllowedIPs := hostPeerUpdate.Peers[peerIndexMap[peerHost.PublicKey.String()]].AllowedIPs
				peerAllowedIPs = append(peerAllowedIPs, allowedips...)
				hostPeerUpdate.Peers[peerIndexMap[peerHost.PublicKey.String()]].AllowedIPs = peerAllowedIPs
				hostPeerUpdate.HostPeerIDs[peerHost.PublicKey.String()][peer.ID.String()] = models.IDandAddr{
					ID:      peer.ID.String(),
					Address: peer.PrimaryAddress(),
					Name:    peerHost.Name,
					Network: peer.Network,
				}
				nodePeer = hostPeerUpdate.Peers[peerIndexMap[peerHost.PublicKey.String()]]
			}

			if node.Network == network { // add to peers map for metrics
				hostPeerUpdate.PeerIDs[peerHost.PublicKey.String()] = models.IDandAddr{
					ID:      peer.ID.String(),
					Address: peer.PrimaryAddress(),
					Name:    peerHost.Name,
					Network: peer.Network,
				}
				hostPeerUpdate.NodePeers = append(hostPeerUpdate.NodePeers, nodePeer)
			}
		}
		var extPeers []wgtypes.PeerConfig
		var extPeerIDAndAddrs []models.IDandAddr
		if node.IsIngressGateway {
			extPeers, extPeerIDAndAddrs, err = getExtPeers(&node)
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
					}
				}
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, extPeers...)
				for _, extPeerIdAndAddr := range extPeerIDAndAddrs {
					extPeerIdAndAddr := extPeerIdAndAddr
					hostPeerUpdate.HostPeerIDs[extPeerIdAndAddr.ID] = make(map[string]models.IDandAddr)
					hostPeerUpdate.HostPeerIDs[extPeerIdAndAddr.ID][extPeerIdAndAddr.ID] = models.IDandAddr{
						ID:      extPeerIdAndAddr.ID,
						Address: extPeerIdAndAddr.Address,
						Name:    extPeerIdAndAddr.Name,
						Network: node.Network,
					}
					hostPeerUpdate.IngressInfo.ExtPeers[extPeerIdAndAddr.ID] = models.ExtClientInfo{
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
						Peers:      nodePeerMap,
					}
					if node.Network == network {
						hostPeerUpdate.PeerIDs[extPeerIdAndAddr.ID] = extPeerIdAndAddr
						hostPeerUpdate.NodePeers = append(hostPeerUpdate.NodePeers, extPeers...)
					}
				}
			} else if !database.IsEmptyRecord(err) {
				logger.Log(1, "error retrieving external clients:", err.Error())
			}
		}
		if node.IsEgressGateway {
			hostPeerUpdate.EgressInfo[node.ID.String()] = models.EgressInfo{
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

	// run through delete nodes
	if len(deletedNodes) > 0 {
		for i := range deletedNodes {
			delNode := deletedNodes[i]
			delHost, err := GetHost(delNode.HostID.String())
			if err != nil {
				continue
			}
			if _, ok := peerIndexMap[delHost.PublicKey.String()]; !ok {
				var peerConfig = wgtypes.PeerConfig{}
				peerConfig.PublicKey = delHost.PublicKey
				peerConfig.Endpoint = &net.UDPAddr{
					IP:   delHost.EndpointIP,
					Port: GetPeerListenPort(delHost),
				}
				peerConfig.Remove = true
				peerConfig.AllowedIPs = []net.IPNet{delNode.Address, delNode.Address6}
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, peerConfig)
			}
		}
	}

	return hostPeerUpdate, nil
}

// GetPeerListenPort - given a host, retrieve it's appropriate listening port
func GetPeerListenPort(host *models.Host) int {
	peerPort := host.ListenPort
	if host.ProxyEnabled {
		if host.PublicListenPort != 0 {
			peerPort = host.PublicListenPort
		} else if host.ProxyListenPort != 0 {
			peerPort = host.ProxyListenPort
		}
	}
	return peerPort
}

func getExtPeers(node *models.Node) ([]wgtypes.PeerConfig, []models.IDandAddr, error) {
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
			ID:      peer.PublicKey.String(),
			Name:    extPeer.ClientID,
			Address: primaryAddr,
		})
	}
	return peers, idsAndAddr, nil

}

func getExtPeersForProxy(node *models.Node, proxyPeerConf map[string]models.PeerConf) ([]wgtypes.PeerConfig, map[string]models.PeerConf, error) {
	var peers []wgtypes.PeerConfig
	host, err := GetHost(node.HostID.String())
	if err != nil {
		logger.Log(0, "error retrieving host for node", node.ID.String(), err.Error())
	}

	extPeers, err := GetNetworkExtClients(node.Network)
	if err != nil {
		return peers, proxyPeerConf, err
	}
	for _, extPeer := range extPeers {
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

		peer = wgtypes.PeerConfig{
			PublicKey:         pubkey,
			ReplaceAllowedIPs: true,
			AllowedIPs:        allowedips,
		}
		extConf := models.PeerConf{
			IsExtClient: true,
			Address:     net.ParseIP(extPeer.Address),
		}
		proxyPeerConf[peer.PublicKey.String()] = extConf

		peers = append(peers, peer)
	}
	return peers, proxyPeerConf, nil

}

// GetAllowedIPs - calculates the wireguard allowedip field for a peer of a node based on the peer and node settings
func GetAllowedIPs(node, peer *models.Node, metrics *models.Metrics) []net.IPNet {
	var allowedips []net.IPNet
	allowedips = getNodeAllowedIPs(peer, node)

	// handle ingress gateway peers
	if peer.IsIngressGateway {
		extPeers, _, err := getExtPeers(peer)
		if err != nil {
			logger.Log(2, "could not retrieve ext peers for ", peer.ID.String(), err.Error())
		}
		for _, extPeer := range extPeers {
			allowedips = append(allowedips, extPeer.AllowedIPs...)
		}
		// if node is a failover node, add allowed ips from nodes it is handling
		if metrics != nil && peer.Failover && metrics.FailoverPeers != nil {
			// traverse through nodes that need handling
			logger.Log(3, "peer", peer.ID.String(), "was found to be failover for", node.ID.String(), "checking failover peers...")
			for k := range metrics.FailoverPeers {
				// if FailoverNode is me for this node, add allowedips
				if metrics.FailoverPeers[k] == peer.ID.String() {
					// get original node so we can traverse the allowed ips
					nodeToFailover, err := GetNodeByID(k)
					if err == nil {
						failoverNodeMetrics, err := GetMetrics(nodeToFailover.ID.String())
						if err == nil && failoverNodeMetrics != nil {
							if len(failoverNodeMetrics.NodeName) > 0 {
								allowedips = append(allowedips, getNodeAllowedIPs(&nodeToFailover, peer)...)
								logger.Log(0, "failing over node", nodeToFailover.ID.String(), nodeToFailover.PrimaryAddress(), "to failover node", peer.ID.String())
							}
						}
					}
				}
			}
		}
	}
	return allowedips
}

func getEgressIPs(node, peer *models.Node) []net.IPNet {
	host, err := GetHost(node.HostID.String())
	if err != nil {
		logger.Log(0, "error retrieving host for node", node.ID.String(), err.Error())
	}
	peerHost, err := GetHost(peer.HostID.String())
	if err != nil {
		logger.Log(0, "error retrieving host for peer", peer.ID.String(), err.Error())
	}

	//check for internet gateway
	internetGateway := false
	if slices.Contains(peer.EgressGatewayRanges, "0.0.0.0/0") || slices.Contains(peer.EgressGatewayRanges, "::/0") {
		internetGateway = true
	}
	allowedips := []net.IPNet{}
	for _, iprange := range peer.EgressGatewayRanges { // go through each cidr for egress gateway
		_, ipnet, err := net.ParseCIDR(iprange) // confirming it's valid cidr
		if err != nil {
			logger.Log(1, "could not parse gateway IP range. Not adding ", iprange)
			continue // if can't parse CIDR
		}
		// getting the public ip of node
		if ipnet.Contains(peerHost.EndpointIP) && !internetGateway { // ensuring egress gateway range does not contain endpoint of node
			logger.Log(2, "egress IP range of ", iprange, " overlaps with ", host.EndpointIP.String(), ", omitting")
			continue // skip adding egress range if overlaps with node's ip
		}
		// TODO: Could put in a lot of great logic to avoid conflicts / bad routes
		if ipnet.Contains(node.LocalAddress.IP) && !internetGateway { // ensuring egress gateway range does not contain public ip of node
			logger.Log(2, "egress IP range of ", iprange, " overlaps with ", node.LocalAddress.String(), ", omitting")
			continue // skip adding egress range if overlaps with node's local ip
		}
		if err != nil {
			logger.Log(1, "error encountered when setting egress range", err.Error())
		} else {
			allowedips = append(allowedips, *ipnet)
		}
	}
	return allowedips
}

func getNodeAllowedIPs(peer, node *models.Node) []net.IPNet {
	var allowedips = []net.IPNet{}
	if peer.Address.IP != nil {
		allowed := net.IPNet{
			IP:   peer.Address.IP,
			Mask: net.CIDRMask(32, 32),
		}
		allowedips = append(allowedips, allowed)
	}
	if peer.Address6.IP != nil {
		allowed := net.IPNet{
			IP:   peer.Address6.IP,
			Mask: net.CIDRMask(128, 128),
		}
		allowedips = append(allowedips, allowed)
	}
	// handle egress gateway peers
	if peer.IsEgressGateway {
		//hasGateway = true
		egressIPs := getEgressIPs(node, peer)
		allowedips = append(allowedips, egressIPs...)
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
