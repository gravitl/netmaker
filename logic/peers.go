package logic

import (
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
	"golang.org/x/exp/slog"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	// ResetFailOver - function to reset failOvered peers on this node
	ResetFailOver = func(failOverNode *models.Node) error {
		return nil
	}
	// ResetFailedOverPeer - removes failed over node from network peers
	ResetFailedOverPeer = func(failedOverNode *models.Node) error {
		return nil
	}
	// FailOverExists - check if failover node existed or not
	FailOverExists = func(network string) (failOverNode models.Node, exists bool) {
		return failOverNode, exists
	}
	// GetFailOverPeerIps - gets failover peerips
	GetFailOverPeerIps = func(peer, node *models.Node) []net.IPNet {
		return []net.IPNet{}
	}
	// CreateFailOver - creates failover in a network
	CreateFailOver = func(node models.Node) error {
		return nil
	}

	// SetDefaulGw
	SetDefaultGw = func(node models.Node, peerUpdate models.HostPeerUpdate) models.HostPeerUpdate {
		return peerUpdate
	}
	SetDefaultGwForRelayedUpdate = func(relayed, relay models.Node, peerUpdate models.HostPeerUpdate) models.HostPeerUpdate {
		return peerUpdate
	}
	// UnsetInternetGw
	UnsetInternetGw = func(node *models.Node) {
		node.IsInternetGateway = false
	}
	// SetInternetGw
	SetInternetGw = func(node *models.Node, req models.InetNodeReq) {
		node.IsInternetGateway = true
	}
	// GetAllowedIpForInetNodeClient
	GetAllowedIpForInetNodeClient = func(node, peer *models.Node) []net.IPNet {
		return []net.IPNet{}
	}
)

// GetHostPeerInfo - fetches required peer info per network
func GetHostPeerInfo(host *models.Host) (models.HostPeerInfo, error) {
	peerInfo := models.HostPeerInfo{
		NetworkPeerIDs: make(map[models.NetworkID]models.PeerMap),
	}
	allNodes, err := GetAllNodes()
	if err != nil {
		return peerInfo, err
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
		networkPeersInfo := make(models.PeerMap)
		defaultDevicePolicy, _ := GetDefaultPolicy(models.NetworkID(node.Network), models.DevicePolicy)

		currentPeers := GetNetworkNodesMemory(allNodes, node.Network)
		for _, peer := range currentPeers {
			peer := peer
			if peer.ID.String() == node.ID.String() {
				logger.Log(2, "peer update, skipping self")
				// skip yourself
				continue
			}

			peerHost, err := GetHost(peer.HostID.String())
			if err != nil {
				logger.Log(1, "no peer host", peer.HostID.String(), err.Error())
				continue
			}

			var allowedToComm bool
			if defaultDevicePolicy.Enabled {
				allowedToComm = true
			} else {
				allowedToComm = IsPeerAllowed(node, peer, false)
			}
			if peer.Action != models.NODE_DELETE &&
				!peer.PendingDelete &&
				peer.Connected &&
				nodeacls.AreNodesAllowed(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()), nodeacls.NodeID(peer.ID.String())) &&
				(defaultDevicePolicy.Enabled || allowedToComm) {

				networkPeersInfo[peerHost.PublicKey.String()] = models.IDandAddr{
					ID:         peer.ID.String(),
					HostID:     peerHost.ID.String(),
					Address:    peer.PrimaryAddress(),
					Name:       peerHost.Name,
					Network:    peer.Network,
					ListenPort: peerHost.ListenPort,
				}

			}
		}
		var extPeerIDAndAddrs []models.IDandAddr
		if node.IsIngressGateway {
			_, extPeerIDAndAddrs, _, err = GetExtPeers(&node, &node)
			if err == nil {
				for _, extPeerIdAndAddr := range extPeerIDAndAddrs {
					networkPeersInfo[extPeerIdAndAddr.ID] = extPeerIdAndAddr
				}
			}
		}
		peerInfo.NetworkPeerIDs[models.NetworkID(node.Network)] = networkPeersInfo
	}
	return peerInfo, nil
}

// GetPeerUpdateForHost - gets the consolidated peer update for the host from all networks
func GetPeerUpdateForHost(network string, host *models.Host, allNodes []models.Node,
	deletedNode *models.Node, deletedClients []models.ExtClient) (models.HostPeerUpdate, error) {
	if host == nil {
		return models.HostPeerUpdate{}, errors.New("host is nil")
	}

	// track which nodes are deleted
	// after peer calculation, if peer not in list, add delete config of peer
	hostPeerUpdate := models.HostPeerUpdate{
		Host:          *host,
		Server:        servercfg.GetServer(),
		ServerVersion: servercfg.GetVersion(),
		ServerAddrs:   []models.ServerAddr{},
		FwUpdate: models.FwUpdate{
			AllowAll:    true,
			EgressInfo:  make(map[string]models.EgressInfo),
			IngressInfo: make(map[string]models.IngressInfo),
			AclRules:    make(map[string]models.AclRule),
		},
		PeerIDs:         make(models.PeerMap, 0),
		Peers:           []wgtypes.PeerConfig{},
		NodePeers:       []wgtypes.PeerConfig{},
		HostNetworkInfo: models.HostInfoMap{},
		ServerConfig:    servercfg.ServerInfo,
	}
	defer func() {
		if !hostPeerUpdate.FwUpdate.AllowAll {
			aclRule := models.AclRule{
				ID:              "allowed-network-rules",
				AllowedProtocol: models.ALL,
				Direction:       models.TrafficDirectionBi,
				Allowed:         true,
			}
			for _, allowedNet := range hostPeerUpdate.FwUpdate.AllowedNetworks {
				if allowedNet.IP.To4() != nil {
					aclRule.IPList = append(aclRule.IPList, allowedNet)
				} else {
					aclRule.IP6List = append(aclRule.IP6List, allowedNet)
				}
			}
			hostPeerUpdate.FwUpdate.AclRules["allowed-network-rules"] = aclRule
		}
	}()

	slog.Debug("peer update for host", "hostId", host.ID.String())
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
		if host.OS == models.OS_Types.IoT {
			hostPeerUpdate.NodeAddrs = append(hostPeerUpdate.NodeAddrs, node.PrimaryAddressIPNet())
			if node.IsRelayed {
				relayNode, err := GetNodeByID(node.RelayedBy)
				if err != nil {
					continue
				}
				relayHost, err := GetHost(relayNode.HostID.String())
				if err != nil {
					continue
				}
				relayPeer := wgtypes.PeerConfig{
					PublicKey:                   relayHost.PublicKey,
					PersistentKeepaliveInterval: &relayHost.PersistentKeepalive,
					ReplaceAllowedIPs:           true,
					AllowedIPs:                  GetAllowedIPs(&node, &relayNode, nil),
				}
				uselocal := false
				if host.EndpointIP.String() == relayHost.EndpointIP.String() {
					// peer is on same network
					// set to localaddress
					uselocal = true
					if node.LocalAddress.IP == nil {
						// use public endpint
						uselocal = false
					}
					if node.LocalAddress.String() == relayNode.LocalAddress.String() {
						uselocal = false
					}
				}
				relayPeer.Endpoint = &net.UDPAddr{
					IP:   relayHost.EndpointIP,
					Port: GetPeerListenPort(relayHost),
				}

				if uselocal {
					relayPeer.Endpoint.IP = relayNode.LocalAddress.IP
					relayPeer.Endpoint.Port = relayHost.ListenPort
				}

				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, relayPeer)
			} else if deletedNode != nil && deletedNode.IsRelay {
				relayHost, err := GetHost(deletedNode.HostID.String())
				if err != nil {
					continue
				}
				relayPeer := wgtypes.PeerConfig{
					PublicKey: relayHost.PublicKey,
					Remove:    true,
				}
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, relayPeer)
			}
			continue
		}
		hostPeerUpdate = SetDefaultGw(node, hostPeerUpdate)
		if !hostPeerUpdate.IsInternetGw {
			hostPeerUpdate.IsInternetGw = IsInternetGw(node)
		}
		defaultUserPolicy, _ := GetDefaultPolicy(models.NetworkID(node.Network), models.UserPolicy)
		defaultDevicePolicy, _ := GetDefaultPolicy(models.NetworkID(node.Network), models.DevicePolicy)

		if defaultDevicePolicy.Enabled && defaultUserPolicy.Enabled {
			if node.NetworkRange.IP != nil {
				hostPeerUpdate.FwUpdate.AllowedNetworks = append(hostPeerUpdate.FwUpdate.AllowedNetworks, node.NetworkRange)
			}
			if node.NetworkRange6.IP != nil {
				hostPeerUpdate.FwUpdate.AllowedNetworks = append(hostPeerUpdate.FwUpdate.AllowedNetworks, node.NetworkRange6)
			}

		} else {
			hostPeerUpdate.FwUpdate.AllowAll = false
			rules := GetAclRulesForNode(&node)
			if len(hostPeerUpdate.FwUpdate.AclRules) == 0 {
				hostPeerUpdate.FwUpdate.AclRules = rules
			} else {
				for aclID, rule := range rules {
					hostPeerUpdate.FwUpdate.AclRules[aclID] = rule
				}
			}
		}
		networkSettings, err := GetNetwork(node.Network)
		if err != nil {
			continue
		}
		hostPeerUpdate.NameServers = append(hostPeerUpdate.NameServers, networkSettings.NameServers...)
		currentPeers := GetNetworkNodesMemory(allNodes, node.Network)
		for _, peer := range currentPeers {
			peer := peer
			if peer.ID.String() == node.ID.String() {
				logger.Log(2, "peer update, skipping self")
				// skip yourself
				continue
			}

			peerHost, err := GetHost(peer.HostID.String())
			if err != nil {
				logger.Log(1, "no peer host", peer.HostID.String(), err.Error())
				continue
			}
			peerConfig := wgtypes.PeerConfig{
				PublicKey:                   peerHost.PublicKey,
				PersistentKeepaliveInterval: &peerHost.PersistentKeepalive,
				ReplaceAllowedIPs:           true,
			}
			if peer.IsEgressGateway {
				hostPeerUpdate.EgressRoutes = append(hostPeerUpdate.EgressRoutes, models.EgressNetworkRoutes{
					EgressGwAddr:           peer.Address,
					EgressGwAddr6:          peer.Address6,
					NodeAddr:               node.Address,
					NodeAddr6:              node.Address6,
					EgressRanges:           peer.EgressGatewayRanges,
					EgressRangesWithMetric: peer.EgressGatewayRequest.RangesWithMetric,
				})
			}
			if peer.IsIngressGateway {
				hostPeerUpdate.EgressRoutes = append(hostPeerUpdate.EgressRoutes, getExtpeersExtraRoutes(node)...)
			}
			_, isFailOverPeer := node.FailOverPeers[peer.ID.String()]
			if (node.IsRelayed && node.RelayedBy != peer.ID.String()) ||
				(peer.IsRelayed && peer.RelayedBy != node.ID.String()) || isFailOverPeer {
				// if node is relayed and peer is not the relay, set remove to true
				if _, ok := peerIndexMap[peerHost.PublicKey.String()]; ok {
					continue
				}
				peerConfig.Remove = true
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, peerConfig)
				peerIndexMap[peerHost.PublicKey.String()] = len(hostPeerUpdate.Peers) - 1
				continue
			}
			if node.IsRelayed && node.RelayedBy == peer.ID.String() {
				hostPeerUpdate = SetDefaultGwForRelayedUpdate(node, peer, hostPeerUpdate)
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

			//1. check currHost has ipv4 endpoint and peerhost has ipv4 then set ipv4 endpoint for peer
			// 2. check currHost has ipv6 endpoint and peerhost has ipv6 then set ipv6 endpoint for peer

			//if host is ipv4 only or ipv4+ipv6, set the peer endpoint to ipv4 address, if host is ipv6 only, set the peer endpoint to ipv6 address
			var peerEndpoint net.IP
			if host.EndpointIP != nil && peerHost.EndpointIP != nil {
				peerEndpoint = peerHost.EndpointIP
			} else if host.EndpointIPv6 != nil && peerHost.EndpointIPv6 != nil {
				peerEndpoint = peerHost.EndpointIPv6
			}
			if host.EndpointIP == nil && peerEndpoint == nil {
				if peerHost.EndpointIP != nil {
					peerEndpoint = peerHost.EndpointIP
				}
			}
			if host.EndpointIPv6 == nil && peerEndpoint == nil {
				if peerHost.EndpointIPv6 != nil {
					peerEndpoint = peerHost.EndpointIPv6
				}
			}
			if node.IsRelay && peer.RelayedBy == node.ID.String() && !peer.IsStatic {
				// don't set endpoint on relayed peer
				peerEndpoint = nil
			}
			if isFailOverPeer && peer.FailedOverBy == node.ID && !peer.IsStatic {
				peerEndpoint = nil
			}

			peerConfig.Endpoint = &net.UDPAddr{
				IP:   peerEndpoint,
				Port: GetPeerListenPort(peerHost),
			}

			if uselocal {
				peerConfig.Endpoint.IP = peer.LocalAddress.IP
				peerConfig.Endpoint.Port = peerHost.ListenPort
			}
			var allowedToComm bool
			if defaultDevicePolicy.Enabled {
				allowedToComm = true
			} else {
				allowedToComm = IsPeerAllowed(node, peer, false)
			}
			if peer.Action != models.NODE_DELETE &&
				!peer.PendingDelete &&
				peer.Connected &&
				nodeacls.AreNodesAllowed(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()), nodeacls.NodeID(peer.ID.String())) &&
				(defaultDevicePolicy.Enabled || allowedToComm) &&
				(deletedNode == nil || (deletedNode != nil && peer.ID.String() != deletedNode.ID.String())) {
				peerConfig.AllowedIPs = GetAllowedIPs(&node, &peer, nil) // only append allowed IPs if valid connection
			}

			var nodePeer wgtypes.PeerConfig
			if _, ok := peerIndexMap[peerHost.PublicKey.String()]; !ok {
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, peerConfig)
				peerIndexMap[peerHost.PublicKey.String()] = len(hostPeerUpdate.Peers) - 1
				hostPeerUpdate.HostNetworkInfo[peerHost.PublicKey.String()] = models.HostNetworkInfo{
					Interfaces:   peerHost.Interfaces,
					ListenPort:   peerHost.ListenPort,
					IsStaticPort: peerHost.IsStaticPort,
					IsStatic:     peerHost.IsStatic,
				}
				nodePeer = peerConfig
			} else {
				peerAllowedIPs := hostPeerUpdate.Peers[peerIndexMap[peerHost.PublicKey.String()]].AllowedIPs
				peerAllowedIPs = append(peerAllowedIPs, peerConfig.AllowedIPs...)
				hostPeerUpdate.Peers[peerIndexMap[peerHost.PublicKey.String()]].AllowedIPs = peerAllowedIPs
				hostPeerUpdate.Peers[peerIndexMap[peerHost.PublicKey.String()]].Remove = false
				hostPeerUpdate.Peers[peerIndexMap[peerHost.PublicKey.String()]].Endpoint = peerConfig.Endpoint
				hostPeerUpdate.HostNetworkInfo[peerHost.PublicKey.String()] = models.HostNetworkInfo{
					Interfaces:   peerHost.Interfaces,
					ListenPort:   peerHost.ListenPort,
					IsStaticPort: peerHost.IsStaticPort,
					IsStatic:     peerHost.IsStatic,
				}
				nodePeer = hostPeerUpdate.Peers[peerIndexMap[peerHost.PublicKey.String()]]
			}

			if node.Network == network && !peerConfig.Remove && len(peerConfig.AllowedIPs) > 0 { // add to peers map for metrics
				hostPeerUpdate.PeerIDs[peerHost.PublicKey.String()] = models.IDandAddr{
					ID:         peer.ID.String(),
					HostID:     peerHost.ID.String(),
					Address:    peer.PrimaryAddress(),
					Name:       peerHost.Name,
					Network:    peer.Network,
					ListenPort: peerHost.ListenPort,
				}
				hostPeerUpdate.NodePeers = append(hostPeerUpdate.NodePeers, nodePeer)
			}
		}
		var extPeers []wgtypes.PeerConfig
		var extPeerIDAndAddrs []models.IDandAddr
		var egressRoutes []models.EgressNetworkRoutes
		if node.IsIngressGateway {
			hostPeerUpdate.FwUpdate.IsIngressGw = true
			extPeers, extPeerIDAndAddrs, egressRoutes, err = GetExtPeers(&node, &node)
			if err == nil {
				if !defaultDevicePolicy.Enabled || !defaultUserPolicy.Enabled {
					ingFwUpdate := models.IngressInfo{
						IngressID:     node.ID.String(),
						Network:       node.NetworkRange,
						Network6:      node.NetworkRange6,
						AllowAll:      defaultDevicePolicy.Enabled && defaultUserPolicy.Default,
						StaticNodeIps: GetStaticNodeIps(node),
						Rules:         GetFwRulesOnIngressGateway(node),
					}
					ingFwUpdate.EgressRanges, ingFwUpdate.EgressRanges6 = getExtpeerEgressRanges(node)
					hostPeerUpdate.FwUpdate.IngressInfo[node.ID.String()] = ingFwUpdate
				}
				hostPeerUpdate.EgressRoutes = append(hostPeerUpdate.EgressRoutes, egressRoutes...)
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, extPeers...)
				for _, extPeerIdAndAddr := range extPeerIDAndAddrs {
					extPeerIdAndAddr := extPeerIdAndAddr
					if node.Network == network {
						hostPeerUpdate.PeerIDs[extPeerIdAndAddr.ID] = extPeerIdAndAddr
						hostPeerUpdate.NodePeers = append(hostPeerUpdate.NodePeers, extPeers...)
					}
				}
			} else if !database.IsEmptyRecord(err) {
				logger.Log(1, "error retrieving external clients:", err.Error())
			}
		}
		if node.IsEgressGateway && node.EgressGatewayRequest.NatEnabled == "yes" && len(node.EgressGatewayRequest.Ranges) > 0 {
			hostPeerUpdate.FwUpdate.IsEgressGw = true
			hostPeerUpdate.FwUpdate.EgressInfo[node.ID.String()] = models.EgressInfo{
				EgressID: node.ID.String(),
				Network:  node.PrimaryNetworkRange(),
				EgressGwAddr: net.IPNet{
					IP:   net.ParseIP(node.PrimaryAddress()),
					Mask: getCIDRMaskFromAddr(node.PrimaryAddress()),
				},
				Network6: node.NetworkRange6,
				EgressGwAddr6: net.IPNet{
					IP:   node.Address6.IP,
					Mask: getCIDRMaskFromAddr(node.Address6.IP.String()),
				},
				EgressGWCfg: node.EgressGatewayRequest,
			}

		}
		if IsInternetGw(node) {
			hostPeerUpdate.FwUpdate.IsEgressGw = true
			egressrange := []string{"0.0.0.0/0"}
			if node.Address6.IP != nil {
				egressrange = append(egressrange, "::/0")
			}
			hostPeerUpdate.FwUpdate.EgressInfo[fmt.Sprintf("%s-%s", node.ID.String(), "inet")] = models.EgressInfo{
				EgressID: fmt.Sprintf("%s-%s", node.ID.String(), "inet"),
				Network:  node.PrimaryAddressIPNet(),
				EgressGwAddr: net.IPNet{
					IP:   net.ParseIP(node.PrimaryAddress()),
					Mask: getCIDRMaskFromAddr(node.PrimaryAddress()),
				},
				Network6: node.NetworkRange6,
				EgressGwAddr6: net.IPNet{
					IP:   node.Address6.IP,
					Mask: getCIDRMaskFromAddr(node.Address6.IP.String()),
				},
				EgressGWCfg: models.EgressGatewayRequest{
					NodeID:     fmt.Sprintf("%s-%s", node.ID.String(), "inet"),
					NetID:      node.Network,
					NatEnabled: "yes",
					Ranges:     egressrange,
				},
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
	if deletedNode != nil && host.OS != models.OS_Types.IoT {
		peerHost, err := GetHost(deletedNode.HostID.String())
		if err == nil && host.ID != peerHost.ID {
			if _, ok := peerIndexMap[peerHost.PublicKey.String()]; !ok {
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, wgtypes.PeerConfig{
					PublicKey: peerHost.PublicKey,
					Remove:    true,
				})
			}
		}

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

// GetPeerListenPort - given a host, retrieve it's appropriate listening port
func GetPeerListenPort(host *models.Host) int {
	peerPort := host.ListenPort
	if !host.IsStaticPort && host.WgPublicListenPort != 0 {
		peerPort = host.WgPublicListenPort
	}
	return peerPort
}

// GetAllowedIPs - calculates the wireguard allowedip field for a peer of a node based on the peer and node settings
func GetAllowedIPs(node, peer *models.Node, metrics *models.Metrics) []net.IPNet {
	var allowedips []net.IPNet
	allowedips = getNodeAllowedIPs(peer, node)
	if peer.IsInternetGateway && node.InternetGwID == peer.ID.String() {
		allowedips = append(allowedips, GetAllowedIpForInetNodeClient(node, peer)...)
		return allowedips
	}
	if node.IsRelayed && node.RelayedBy == peer.ID.String() {
		allowedips = append(allowedips, GetAllowedIpsForRelayed(node, peer)...)
		if peer.InternetGwID != "" {
			return allowedips
		}
	}

	// handle ingress gateway peers
	if peer.IsIngressGateway {
		extPeers, _, _, err := GetExtPeers(peer, node)
		if err != nil {
			logger.Log(2, "could not retrieve ext peers for ", peer.ID.String(), err.Error())
		}
		for _, extPeer := range extPeers {

			allowedips = append(allowedips, extPeer.AllowedIPs...)
		}
	}

	return allowedips
}

func GetEgressIPs(peer *models.Node) []net.IPNet {

	peerHost, err := GetHost(peer.HostID.String())
	if err != nil {
		logger.Log(0, "error retrieving host for peer", peer.ID.String(), "host id", peer.HostID.String(), err.Error())
	}

	// check for internet gateway
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
			logger.Log(2, "egress IP range of ", iprange, " overlaps with ", peerHost.EndpointIP.String(), ", omitting")
			continue // skip adding egress range if overlaps with node's ip
		}
		// TODO: Could put in a lot of great logic to avoid conflicts / bad routes
		if ipnet.Contains(peer.LocalAddress.IP) && !internetGateway { // ensuring egress gateway range does not contain public ip of node
			logger.Log(2, "egress IP range of ", iprange, " overlaps with ", peer.LocalAddress.String(), ", omitting")
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
		// hasGateway = true
		egressIPs := GetEgressIPs(peer)
		allowedips = append(allowedips, egressIPs...)
	}
	if peer.IsRelay {
		allowedips = append(allowedips, RelayedAllowedIPs(peer, node)...)
	}
	if peer.IsFailOver {
		allowedips = append(allowedips, GetFailOverPeerIps(peer, node)...)
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
