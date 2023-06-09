package logic

import (
	"context"
	"errors"
	"fmt"
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

var (
	// PeerUpdateCtx context to send to host peer updates
	PeerUpdateCtx context.Context
	// PeerUpdateStop - the cancel for PeerUpdateCtx
	PeerUpdateStop context.CancelFunc
)

// ResetPeerUpdateContext - kills any current peer updates and resets the context
func ResetPeerUpdateContext() {
	if PeerUpdateCtx != nil && PeerUpdateStop != nil {
		PeerUpdateStop() // tell any current peer updates to stop
	}

	PeerUpdateCtx, PeerUpdateStop = context.WithCancel(context.Background())
}

// GetPeerUpdateForHost - gets the consolidated peer update for the host from all networks
func GetPeerUpdateForHost(ctx context.Context, network string, host *models.Host, deletedNode *models.Node, deletedClients []models.ExtClient) (models.HostPeerUpdate, error) {
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
		HostPeerIDs:     make(models.HostPeerMap, 0),
		ServerVersion:   servercfg.GetVersion(),
		ServerAddrs:     []models.ServerAddr{},
		PeerIDs:         make(models.PeerMap, 0),
		Peers:           []wgtypes.PeerConfig{},
		NodePeers:       []wgtypes.PeerConfig{},
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
			select {
			case <-ctx.Done():
				logger.Log(2, "cancelled peer update for host", host.Name, host.ID.String())
				return models.HostPeerUpdate{}, fmt.Errorf("peer update cancelled")
			default:
				peer := peer
				if peer.ID.String() == node.ID.String() {
					logger.Log(2, "peer update, skipping self")
					//skip yourself
					continue
				}
				if peer.IsRelayed {
					// skip relayed peers; will be included in relay peer
					continue
				}
				var peerConfig wgtypes.PeerConfig
				peerHost, err := GetHost(peer.HostID.String())
				if err != nil {
					logger.Log(1, "no peer host", peer.HostID.String(), err.Error())
					return models.HostPeerUpdate{}, err
				}

				peerConfig.PublicKey = peerHost.PublicKey
				peerConfig.PersistentKeepaliveInterval = &peer.PersistentKeepalive
				peerConfig.ReplaceAllowedIPs = true
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
					host, err := GetHost(peer.HostID.String())
					if err == nil {
						allowedips = append(allowedips, getEgressIPs(
							&models.Client{
								Host: *host,
								Node: peer,
							})...)
					}
				}
				if peer.Action != models.NODE_DELETE &&
					!peer.PendingDelete &&
					peer.Connected &&
					nodeacls.AreNodesAllowed(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()), nodeacls.NodeID(peer.ID.String())) &&
					(deletedNode == nil || (deletedNode != nil && peer.ID.String() != deletedNode.ID.String())) {
					peerConfig.AllowedIPs = allowedips // only append allowed IPs if valid connection
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
					hostPeerUpdate.HostNetworkInfo[peerHost.PublicKey.String()] = models.HostNetworkInfo{
						Interfaces: peerHost.Interfaces,
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
					hostPeerUpdate.HostNetworkInfo[peerHost.PublicKey.String()] = models.HostNetworkInfo{
						Interfaces: peerHost.Interfaces,
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
		}
		var extPeers []wgtypes.PeerConfig
		var extPeerIDAndAddrs []models.IDandAddr
		if node.IsIngressGateway {
			extPeers, extPeerIDAndAddrs, err = GetExtPeers(&node)
			if err == nil {
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
					if node.Network == network {
						hostPeerUpdate.PeerIDs[extPeerIdAndAddr.ID] = extPeerIdAndAddr
						hostPeerUpdate.NodePeers = append(hostPeerUpdate.NodePeers, extPeers...)
					}
				}
			} else if !database.IsEmptyRecord(err) {
				logger.Log(1, "error retrieving external clients:", err.Error())
			}
		}
	}
	// == post peer calculations ==
	// indicate removal if no allowed IPs were calculated
	for i := range hostPeerUpdate.Peers {
		peer := hostPeerUpdate.Peers[i]
		if len(peer.AllowedIPs) == 0 {
			peer.Remove = true
		}
		hostPeerUpdate.Peers[i] = peer
	}

	for i := range hostPeerUpdate.NodePeers {
		peer := hostPeerUpdate.NodePeers[i]
		if len(peer.AllowedIPs) == 0 {
			peer.Remove = true
		}
		hostPeerUpdate.NodePeers[i] = peer
	}

	if len(deletedClients) > 0 {
		for i := range deletedClients {
			deletedClient := deletedClients[i]
			key, err := wgtypes.ParseKey(deletedClient.PublicKey)
			if err == nil {
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, wgtypes.PeerConfig{
					PublicKey: key,
					Remove:    true,
				})
			}
		}
	}

	return hostPeerUpdate, nil
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
		extPeers, _, err := GetExtPeers(peer)
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

// getEgressIPs - gets the egress IPs for a client
func getEgressIPs(client *models.Client) []net.IPNet {

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
		host, err := GetHost(peer.HostID.String())
		if err == nil {
			egressIPs := getEgressIPs(
				&models.Client{
					Host: *host,
					Node: *peer,
				})
			allowedips = append(allowedips, egressIPs...)
		}
	}
	if peer.IsRelay {
		for _, relayed := range peer.RelayedNodes {
			allowed := getRelayedAddresses(relayed)
			allowedips = append(allowedips, allowed...)
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

func GetPeerUpdate(host *models.Host) []wgtypes.PeerConfig {
	peerUpdate := []wgtypes.PeerConfig{}
	for _, nodeStr := range host.Nodes {
		node, err := GetNodeByID(nodeStr)
		if err != nil {
			continue
		}
		client := models.Client{Host: *host, Node: node}
		peers, err := GetNetworkClients(node.Network)
		if err != nil {
			continue
		}
		if node.IsRelayed {
			peerUpdate = append(peerUpdate, peerUpdateForRelayed(&client, peers)...)
			continue
		}
		if node.IsRelay {
			peerUpdate = append(peerUpdate, peerUpdateForRelay(&client, peers)...)
			continue
		}
		for _, peer := range peers {
			if peer.Host.ID == client.Host.ID {
				continue
			}
			// if peer is relayed by some other node, remove it from the peer list,  it
			// will be added to allowedips of relay peer
			if peer.Node.IsRelayed {
				update := wgtypes.PeerConfig{
					PublicKey: peer.Host.PublicKey,
					Remove:    true,
				}
				peerUpdate = append(peerUpdate, update)
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
			// if peer is a relay that relays us, don't do anything
			if peer.Node.IsRelay && client.Node.RelayedBy == peer.Node.ID.String() {
				continue
			} else {
				update.AllowedIPs = append(update.AllowedIPs, getRelayAllowedIPs(&peer)...)
			}
			//normal peer
			if nodeacls.AreNodesAllowed(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()), nodeacls.NodeID(peer.Node.ID.String())) {
				update.AllowedIPs = append(update.AllowedIPs, AddAllowedIPs(&peer)...)
				peerUpdate = append(peerUpdate, update)
			} else {
				update.Remove = true
				peerUpdate = append(peerUpdate, update)
			}
		}
	}
	return peerUpdate
}

func AddAllowedIPs(peer *models.Client) []net.IPNet {
	allowedIPs := []net.IPNet{}
	if peer.Node.Address.IP != nil {
		peer.Node.Address.Mask = net.CIDRMask(32, 32)
		allowedIPs = append(allowedIPs, peer.Node.Address)
	}
	if peer.Node.Address6.IP != nil {
		peer.Node.Address6.Mask = net.CIDRMask(128, 128)
		allowedIPs = append(allowedIPs, peer.Node.Address6)
	}
	if peer.Node.IsEgressGateway {
		allowedIPs = append(allowedIPs, getEgressIPs(peer)...)
	}
	if peer.Node.IsIngressGateway {
		allowedIPs = append(allowedIPs, getIngressIPs(peer)...)
	}
	return allowedIPs
}

// getRelayAllowedIPs returns the list of allowedips for a peer that is a relay
func getRelayAllowedIPs(peer *models.Client) []net.IPNet {
	var relayIPs []net.IPNet
	if !peer.Node.IsRelay {
		logger.Log(0, "getRelayAllowedIPs called for a non-relay node", peer.Host.Name)
		return relayIPs
	}
	//if !client.Node.IsRelayed || client.Node.RelayedBy != peer.Node.ID.String() {
	//logger.Log(0, "getRelayAllowedIPs called for non-relayed node", client.Host.Name, peer.Host.Name)
	//return relayIPs
	//}
	for _, relayed := range peer.Node.RelayedNodes {
		relayedNode, err := GetNodeByID(relayed)
		if err != nil {
			logger.Log(0, "retrieve relayed node", err.Error())
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
		host, err := GetHost(relayedNode.HostID.String())
		if err == nil {
			if relayedNode.IsRelay {
				relayIPs = append(relayIPs, getRelayAllowedIPs(
					&models.Client{
						Host: *host,
						Node: relayedNode,
					})...)
			}
			if relayedNode.IsEgressGateway {
				relayIPs = append(relayIPs, getEgressIPs(
					&models.Client{
						Host: *host,
						Node: relayedNode,
					})...)
			}
			if relayedNode.IsIngressGateway {
				relayIPs = append(relayIPs, getIngressIPs(
					&models.Client{
						Host: *host,
						Node: relayedNode,
					})...)
			}
		}
	}
	return relayIPs
}

// getIngressIPs returns the additional allowedips (ext client addresses) that need
// to be included for an ingress gateway peer
// TODO:  add ExtraAllowedIPs
func getIngressIPs(peer *models.Client) []net.IPNet {
	var ingressIPs []net.IPNet
	extclients, err := GetNetworkExtClients(peer.Node.Network)
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

// GetPeerUpdateForRelay - returns the peer update for a relay node
func GetPeerUpdateForRelay(client *models.Client, peers []models.Client) []wgtypes.PeerConfig {
	peerConfig := []wgtypes.PeerConfig{}
	if !client.Node.IsRelay {
		return []wgtypes.PeerConfig{}
	}
	for _, peer := range peers {
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
		update.AllowedIPs = append(update.AllowedIPs, AddAllowedIPs(&peer)...)
		peerConfig = append(peerConfig, update)
	}
	return peerConfig
}
