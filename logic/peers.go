package logic

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slices"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// GetPeersforProxy calculates the peers for a proxy
// TODO ==========================
// TODO ==========================
// TODO ==========================
// TODO ==========================
// TODO ==========================
// revisit this logic with new host/node models.
func GetPeersForProxy(node *models.Node, onlyPeers bool) (models.ProxyManagerPayload, error) {
	proxyPayload := models.ProxyManagerPayload{}
	var peers []wgtypes.PeerConfig
	peerConfMap := make(map[string]models.PeerConf)
	var err error
	currentPeers, err := GetNetworkNodes(node.Network)
	if err != nil {
		return proxyPayload, err
	}
	if !onlyPeers {
		if node.IsRelayed {
			relayNode := FindRelay(node)
			relayHost, err := GetHost(relayNode.HostID.String())
			if err != nil {
				return proxyPayload, err
			}
			if relayNode != nil {
				host, err := GetHost(relayNode.HostID.String())
				if err != nil {
					logger.Log(0, "error retrieving host for relay node", relayNode.HostID.String(), err.Error())
				}
				relayEndpoint, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", relayHost.EndpointIP, host.ListenPort))
				if err != nil {
					logger.Log(1, "failed to resolve relay node endpoint: ", err.Error())
				}
				proxyPayload.IsRelayed = true
				proxyPayload.RelayedTo = relayEndpoint
			} else {
				logger.Log(0, "couldn't find relay node for:  ", node.ID.String())
			}

		}
		if node.IsRelay {
			host, err := GetHost(node.HostID.String())
			if err != nil {
				logger.Log(0, "error retrieving host for relay node", node.ID.String(), err.Error())
			}
			relayedNodes, err := GetRelayedNodes(node)
			if err != nil {
				logger.Log(1, "failed to relayed nodes: ", node.ID.String(), err.Error())
				proxyPayload.IsRelay = false
			} else {
				relayPeersMap := make(map[string]models.RelayedConf)
				for _, relayedNode := range relayedNodes {
					relayedNode := relayedNode
					payload, err := GetPeersForProxy(&relayedNode, true)
					if err == nil {
						relayedHost, err := GetHost(relayedNode.HostID.String())
						if err != nil {
							logger.Log(0, "error retrieving host for relayNode", relayedNode.ID.String(), err.Error())
						}
						relayedEndpoint, udpErr := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", relayedHost.EndpointIP, host.ListenPort))
						if udpErr == nil {
							relayPeersMap[host.PublicKey.String()] = models.RelayedConf{
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
		}

	}

	for _, peer := range currentPeers {
		if peer.ID == node.ID {
			//skip yourself
			continue
		}
		host, err := GetHost(peer.HostID.String())
		if err != nil {
			continue
		}
		proxyStatus := host.ProxyEnabled
		listenPort := host.ListenPort
		if proxyStatus {
			listenPort = host.ProxyListenPort
			if listenPort == 0 {
				listenPort = models.NmProxyPort
			}
		} else if listenPort == 0 {
			listenPort = host.ListenPort

		}

		endpoint, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host.EndpointIP, listenPort))
		if err != nil {
			logger.Log(1, "failed to resolve udp addr for node: ", peer.ID.String(), host.EndpointIP.String(), err.Error())
			continue
		}
		allowedips := GetAllowedIPs(node, &peer, nil)
		var keepalive time.Duration
		if node.PersistentKeepalive != 0 {
			// set_keepalive
			keepalive = node.PersistentKeepalive
		}
		peers = append(peers, wgtypes.PeerConfig{
			PublicKey:                   host.PublicKey,
			Endpoint:                    endpoint,
			AllowedIPs:                  allowedips,
			PersistentKeepaliveInterval: &keepalive,
			ReplaceAllowedIPs:           true,
		})
		peerConfMap[host.PublicKey.String()] = models.PeerConf{
			Address:          net.ParseIP(peer.PrimaryAddress()),
			Proxy:            proxyStatus,
			PublicListenPort: int32(listenPort),
		}

		if !onlyPeers && peer.IsRelayed {
			relayNode := FindRelay(&peer)
			if relayNode != nil {
				relayHost, err := GetHost(relayNode.HostID.String())
				if err != nil {
					logger.Log(0, "error retrieving host for relayNode", relayNode.ID.String(), err.Error())
					continue
				}
				relayTo, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", relayHost.EndpointIP, relayHost.ListenPort))
				if err == nil {
					peerConfMap[host.PublicKey.String()] = models.PeerConf{

						IsRelayed:        true,
						RelayedTo:        relayTo,
						Address:          net.ParseIP(peer.PrimaryAddress()),
						Proxy:            proxyStatus,
						PublicListenPort: int32(listenPort),
					}
				}

			}

		}
	}
	if node.IsIngressGateway {
		var extPeers []wgtypes.PeerConfig
		extPeers, peerConfMap, err = getExtPeersForProxy(node, peerConfMap)
		if err == nil {
			peers = append(peers, extPeers...)

		} else if !database.IsEmptyRecord(err) {
			logger.Log(1, "error retrieving external clients:", err.Error())
		}
	}

	proxyPayload.IsIngress = node.IsIngressGateway
	addr := node.Address
	if addr.String() == "" {
		addr = node.Address6
	}
	proxyPayload.Peers = peers
	proxyPayload.PeerMap = peerConfMap
	//proxyPayload.Network = node.Network
	//proxyPayload.InterfaceName = node.Interface
	//hardcode or read from host ??
	proxyPayload.InterfaceName = models.WIREGUARD_INTERFACE

	return proxyPayload, nil
}

// GetProxyUpdateForHost - gets the proxy update for host
func GetProxyUpdateForHost(host *models.Host) (models.ProxyManagerPayload, error) {
	proxyPayload := models.ProxyManagerPayload{
		Action: models.ProxyUpdate,
	}
	peerConfMap := make(map[string]models.PeerConf)
	if host.IsRelayed {
		relayHost, err := GetHost(host.RelayedBy)
		if err == nil {
			relayEndpoint, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", relayHost.EndpointIP, getPeerListenPort(relayHost)))
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
			payload, err := GetPeerUpdateForHost(&relayedHost)
			if err == nil {
				relayedEndpoint, udpErr := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", relayedHost.EndpointIP, getPeerListenPort(&relayedHost)))
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
					PublicListenPort: int32(getPeerListenPort(peerHost)),
				}
			}

			if peerHost.IsRelayed && peerHost.RelayedBy != host.ID.String() {
				relayHost, err := GetHost(peerHost.RelayedBy)
				if err == nil {
					relayTo, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", relayHost.EndpointIP, getPeerListenPort(peerHost)))
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
func GetPeerUpdateForHost(host *models.Host) (models.HostPeerUpdate, error) {
	if host == nil {
		return models.HostPeerUpdate{}, errors.New("host is nil")
	}
	hostPeerUpdate := models.HostPeerUpdate{
		Host:          *host,
		Network:       make(map[string]models.NetworkInfo),
		PeerIDs:       make(models.HostPeerMap),
		ServerVersion: servercfg.GetVersion(),
		ServerAddrs:   []models.ServerAddr{},
	}
	logger.Log(1, "peer update for host ", host.ID.String())
	peerIndexMap := make(map[string]int)
	for _, nodeID := range host.Nodes {
		node, err := GetNodeByID(nodeID)
		if err != nil {
			continue
		}
		if !node.Connected || node.Action == models.NODE_DELETE || node.PendingDelete {
			continue
		}
		hostPeerUpdate.Network[node.Network] = models.NetworkInfo{
			DNS: getPeerDNS(node.Network),
		}
		currentPeers, err := GetNetworkNodes(node.Network)
		if err != nil {
			log.Println("no network nodes")
			return models.HostPeerUpdate{}, err
		}
		for _, peer := range currentPeers {
			if peer.ID == node.ID {
				logger.Log(2, "peer update, skipping self")
				//skip yourself

				continue
			}
			var peerConfig wgtypes.PeerConfig
			peerHost, err := GetHost(peer.HostID.String())
			if err != nil {
				logger.Log(1, "no peer host", peer.HostID.String(), err.Error())
				return models.HostPeerUpdate{}, err
			}

			if !peer.Connected || peer.Action == models.NODE_DELETE || peer.PendingDelete {
				logger.Log(2, "peer update, skipping unconnected node", peer.ID.String())
				//skip unconnected nodes
				continue
			}
			if !nodeacls.AreNodesAllowed(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()), nodeacls.NodeID(peer.ID.String())) {
				log.Println("peer update, skipping node for acl")
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
				Port: getPeerListenPort(peerHost),
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

			if _, ok := hostPeerUpdate.PeerIDs[peerHost.PublicKey.String()]; !ok {
				hostPeerUpdate.PeerIDs[peerHost.PublicKey.String()] = make(map[string]models.IDandAddr)
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, peerConfig)
				peerIndexMap[peerHost.PublicKey.String()] = len(hostPeerUpdate.Peers) - 1
				hostPeerUpdate.PeerIDs[peerHost.PublicKey.String()][peer.ID.String()] = models.IDandAddr{
					ID:      peer.ID.String(),
					Address: peer.PrimaryAddress(),
					Name:    peerHost.Name,
					Network: peer.Network,
				}
			} else {
				peerAllowedIPs := hostPeerUpdate.Peers[peerIndexMap[peerHost.PublicKey.String()]].AllowedIPs
				peerAllowedIPs = append(peerAllowedIPs, allowedips...)
				hostPeerUpdate.Peers[peerIndexMap[peerHost.PublicKey.String()]].AllowedIPs = peerAllowedIPs
				hostPeerUpdate.PeerIDs[peerHost.PublicKey.String()][peer.ID.String()] = models.IDandAddr{
					ID:      peer.ID.String(),
					Address: peer.PrimaryAddress(),
					Name:    peerHost.Name,
					Network: peer.Network,
				}
			}

		}
		if node.IsIngressGateway {
			extPeers, extPeerIDAndAddrs, err := getExtPeers(&node)
			if err == nil {
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, extPeers...)
				for _, extPeerIdAndAddr := range extPeerIDAndAddrs {
					hostPeerUpdate.PeerIDs[extPeerIdAndAddr.ID] = make(map[string]models.IDandAddr)
					hostPeerUpdate.PeerIDs[extPeerIdAndAddr.ID][extPeerIdAndAddr.ID] = models.IDandAddr{
						ID:      extPeerIdAndAddr.ID,
						Address: extPeerIdAndAddr.Address,
						Name:    extPeerIdAndAddr.Name,
						Network: node.Network,
					}
				}

			} else if !database.IsEmptyRecord(err) {
				logger.Log(1, "error retrieving external clients:", err.Error())
			}
		}
	}

	return hostPeerUpdate, nil
}

func getPeerListenPort(host *models.Host) int {
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

// GetPeerUpdate - gets a wireguard peer config for each peer of a node
func GetPeerUpdate(node *models.Node, host *models.Host) (models.PeerUpdate, error) {
	log.Println("peer update for node ", node.ID)
	peerUpdate := models.PeerUpdate{
		Network:       node.Network,
		ServerVersion: ncutils.Version,
		DNS:           getPeerDNS(node.Network),
		PeerIDs:       make(models.PeerMap),
	}
	currentPeers, err := GetNetworkNodes(node.Network)
	if err != nil {
		log.Println("no network nodes")
		return models.PeerUpdate{}, err
	}
	for _, peer := range currentPeers {
		var peerConfig wgtypes.PeerConfig
		peerHost, err := GetHost(peer.HostID.String())
		if err != nil {
			log.Println("no peer host", err)
			return models.PeerUpdate{}, err
		}
		if peer.ID == node.ID {
			log.Println("peer update, skipping self")
			//skip yourself

			continue
		}
		if !peer.Connected {
			log.Println("peer update, skipping unconnected node")
			//skip unconnected nodes
			continue
		}
		if !nodeacls.AreNodesAllowed(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()), nodeacls.NodeID(peer.ID.String())) {
			log.Println("peer update, skipping node for acl")
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
			Port: peerHost.ListenPort,
		}
		if peerHost.ProxyEnabled {
			peerConfig.Endpoint.Port = getPeerListenPort(peerHost)
		}
		if uselocal {
			peerConfig.Endpoint.IP = peer.LocalAddress.IP
		}
		allowedips := getNodeAllowedIPs(&peer, node)
		if peer.IsIngressGateway {
			for _, entry := range peer.IngressGatewayRange {
				_, cidr, err := net.ParseCIDR(string(entry))
				if err == nil {
					allowedips = append(allowedips, *cidr)
				}
			}
		}
		if peer.IsEgressGateway {
			allowedips = append(allowedips, getEgressIPs(node, &peer)...)
		}

		peerUpdate.PeerIDs[peerHost.PublicKey.String()] = models.IDandAddr{
			ID:      peer.ID.String(),
			Address: peer.PrimaryAddress(),
			Name:    peerHost.Name,
			Network: peer.Network,
		}
		peerConfig.AllowedIPs = allowedips
		peerUpdate.Peers = append(peerUpdate.Peers, peerConfig)
	}
	return peerUpdate, nil
}

// func getRelayAllowedIPs(node, peer *models.Node) []net.IPNet {
// 	var allowedips []net.IPNet
// 	var allowedip net.IPNet
// 	for _, addr := range peer.RelayAddrs {
// 		if node.Address.IP.String() == addr {
// 			continue
// 		}
// 		if node.Address6.IP.String() == addr {
// 			continue
// 		}
// 		allowedip.IP = net.ParseIP(addr)
// 		allowedips = append(allowedips, allowedip)
// 	}
// 	return allowedips
// }

// GetPeerUpdateLegacy - gets a wireguard peer config for each peer of a node
func GetPeerUpdateLegacy(node *models.Node) (models.PeerUpdate, error) {
	var peerUpdate models.PeerUpdate
	var peers []wgtypes.PeerConfig
	var serverNodeAddresses = []models.ServerAddr{}
	var peerMap = make(models.PeerMap)
	var metrics *models.Metrics
	if servercfg.Is_EE {
		metrics, _ = GetMetrics(node.ID.String())
	}
	if metrics == nil {
		metrics = &models.Metrics{}
	}
	if metrics.FailoverPeers == nil {
		metrics.FailoverPeers = make(map[string]string)
	}
	// udppeers = the peers parsed from the local interface
	// gives us correct port to reach
	udppeers, errN := database.GetPeers(node.Network)
	if errN != nil {
		logger.Log(2, errN.Error())
	}

	currentPeers, err := GetNetworkNodes(node.Network)
	if err != nil {
		return models.PeerUpdate{}, err
	}
	host, err := GetHost(node.HostID.String())
	if err != nil {
		return peerUpdate, err
	}
	if node.IsRelayed && !host.ProxyEnabled {
		return GetPeerUpdateForRelayedNode(node, udppeers)
	}

	// #1 Set Keepalive values: set_keepalive
	// #2 Set local address: set_local - could be a LOT BETTER and fix some bugs with additional logic
	// #3 Set allowedips: set_allowedips
	for _, peer := range currentPeers {
		peerHost, err := GetHost(peer.HostID.String())
		if err != nil {
			logger.Log(0, "error retrieving host for peer", node.ID.String(), err.Error())
			return models.PeerUpdate{}, err
		}
		if peer.ID == node.ID {
			//skip yourself
			continue
		}
		if node.Connected {
			//skip unconnected nodes
			continue
		}

		// if the node is not a server, set the endpoint
		var setEndpoint = true

		if peer.IsRelayed {
			if !peerHost.ProxyEnabled && !(node.IsRelay && ncutils.StringSliceContains(node.RelayAddrs, peer.PrimaryAddress())) {
				//skip -- will be added to relay
				continue
			} else if node.IsRelay && ncutils.StringSliceContains(node.RelayAddrs, peer.PrimaryAddress()) {
				// dont set peer endpoint if it's relayed by node
				setEndpoint = false
			}
		}
		if !nodeacls.AreNodesAllowed(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()), nodeacls.NodeID(peer.ID.String())) {
			//skip if not permitted by acl
			continue
		}
		if len(metrics.FailoverPeers[peer.ID.String()]) > 0 && IsFailoverPresent(node.Network) {
			logger.Log(2, "peer", peer.ID.String(), peer.PrimaryAddress(), "was found to be in failover peers list for node", node.ID.String(), node.PrimaryAddress())
			continue
		}
		if err != nil {
			return models.PeerUpdate{}, err
		}
		host, err := GetHost(node.HostID.String())
		if err != nil {
			logger.Log(0, "error retrieving host for node", node.ID.String(), err.Error())
			return models.PeerUpdate{}, err
		}
		if host.EndpointIP.String() == peerHost.EndpointIP.String() {
			//peer is on same network
			// set_local
			if node.LocalAddress.String() != peer.LocalAddress.String() && peer.LocalAddress.IP != nil {
				peerHost.EndpointIP = peer.LocalAddress.IP
				if peerHost.ListenPort != 0 {
					peerHost.ListenPort = getPeerListenPort(peerHost)
				}
			} else {
				continue
			}
		}

		// set address if setEndpoint is true
		// otherwise, will get inserted as empty value
		var address *net.UDPAddr

		// Sets ListenPort to UDP Hole Punching Port assuming:
		// - UDP Hole Punching is enabled
		// - udppeers retrieval did not return an error
		// - the endpoint is valid
		if setEndpoint {

			var setUDPPort = false
			if CheckEndpoint(udppeers[peerHost.PublicKey.String()]) {
				endpointstring := udppeers[peerHost.PublicKey.String()]
				endpointarr := strings.Split(endpointstring, ":")
				if len(endpointarr) == 2 {
					port, err := strconv.Atoi(endpointarr[1])
					if err == nil {
						setUDPPort = true
						peerHost.ListenPort = port
					}
				}
			}
			// if udp hole punching is on, but udp hole punching did not set it, use the LocalListenPort instead
			// or, if port is for some reason zero use the LocalListenPort
			// but only do this if LocalListenPort is not zero
			if ((!setUDPPort) || peerHost.ListenPort == 0) && peerHost.ListenPort != 0 {
				peerHost.ListenPort = getPeerListenPort(peerHost)
			}

			endpoint := peerHost.EndpointIP.String() + ":" + strconv.FormatInt(int64(peerHost.ListenPort), 10)
			address, err = net.ResolveUDPAddr("udp", endpoint)
			if err != nil {
				return models.PeerUpdate{}, err
			}
		}
		allowedips := GetAllowedIPs(node, &peer, metrics)
		var keepalive time.Duration
		if node.PersistentKeepalive != 0 {
			// set_keepalive
			keepalive = node.PersistentKeepalive
		}
		var peerData = wgtypes.PeerConfig{
			PublicKey:                   peerHost.PublicKey,
			Endpoint:                    address,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  allowedips,
			PersistentKeepaliveInterval: &keepalive,
		}

		peers = append(peers, peerData)
		peerMap[peerHost.PublicKey.String()] = models.IDandAddr{
			Name:     peerHost.Name,
			ID:       peer.ID.String(),
			Address:  peer.PrimaryAddress(),
			IsServer: "no",
		}

	}
	if node.IsIngressGateway {
		extPeers, idsAndAddr, err := getExtPeers(node)
		if err == nil {
			peers = append(peers, extPeers...)
			for i := range idsAndAddr {
				peerMap[idsAndAddr[i].ID] = idsAndAddr[i]
			}
		} else if !database.IsEmptyRecord(err) {
			logger.Log(1, "error retrieving external clients:", err.Error())
		}
	}

	peerUpdate.Network = node.Network
	peerUpdate.ServerVersion = servercfg.Version
	sort.SliceStable(peers[:], func(i, j int) bool {
		return peers[i].PublicKey.String() < peers[j].PublicKey.String()
	})
	peerUpdate.Peers = peers
	peerUpdate.ServerAddrs = serverNodeAddresses
	peerUpdate.DNS = getPeerDNS(node.Network)
	peerUpdate.PeerIDs = peerMap
	return peerUpdate, nil
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
			extPeer.IngressGatewayID != node.ID.String() {
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
			extPeer.IngressGatewayID != node.ID.String() {
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

func getPeerDNS(network string) string {
	var dns string
	if nodes, err := GetNetworkNodes(network); err == nil {
		for i, node := range nodes {
			host, err := GetHost(node.HostID.String())
			if err != nil {
				logger.Log(0, "error retrieving host for node", node.ID.String(), err.Error())
				continue
			}
			dns = dns + fmt.Sprintf("%s %s.%s\n", nodes[i].Address, host.Name, nodes[i].Network)
		}
	}

	if customDNSEntries, err := GetCustomDNS(network); err == nil {
		for _, entry := range customDNSEntries {
			// TODO - filter entries based on ACLs / given peers vs nodes in network
			dns = dns + fmt.Sprintf("%s %s.%s\n", entry.Address, entry.Name, entry.Network)
		}
	}
	return dns
}

// GetPeerUpdateForRelayedNode - calculates peer update for a relayed node by getting the relay
// copying the relay node's allowed ips and making appropriate substitutions
func GetPeerUpdateForRelayedNode(node *models.Node, udppeers map[string]string) (models.PeerUpdate, error) {
	var peerUpdate models.PeerUpdate
	var peers []wgtypes.PeerConfig
	var serverNodeAddresses = []models.ServerAddr{}
	var allowedips []net.IPNet
	//find node that is relaying us
	relay := FindRelay(node)
	if relay == nil {
		return models.PeerUpdate{}, errors.New("not found")
	}

	//add relay to lists of allowed ip
	if relay.Address.IP != nil {
		relayIP := relay.Address
		allowedips = append(allowedips, relayIP)
	}
	if relay.Address6.IP != nil {
		relayIP6 := relay.Address6
		allowedips = append(allowedips, relayIP6)
	}
	//get PeerUpdate for relayed node
	relayHost, err := GetHost(relay.HostID.String())
	if err != nil {
		return models.PeerUpdate{}, err
	}
	relayPeerUpdate, err := GetPeerUpdate(relay, relayHost)
	if err != nil {
		return models.PeerUpdate{}, err
	}
	//add the relays allowed ips from all of the relay's peers
	for _, peer := range relayPeerUpdate.Peers {
		allowedips = append(allowedips, peer.AllowedIPs...)
	}
	//delete any ips not permitted by acl
	for i := len(allowedips) - 1; i >= 0; i-- {
		target, err := findNode(allowedips[i].IP.String())
		if err != nil {
			logger.Log(0, "failed to find node for ip", allowedips[i].IP.String(), err.Error())
			continue
		}
		if target == nil {
			logger.Log(0, "failed to find node for ip", allowedips[i].IP.String())
			continue
		}
		if !nodeacls.AreNodesAllowed(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()), nodeacls.NodeID(target.ID.String())) {
			logger.Log(0, "deleting node from relayednode per acl", node.ID.String(), target.ID.String())
			allowedips = append(allowedips[:i], allowedips[i+1:]...)
		}
	}
	//delete self from allowed ips
	for i := len(allowedips) - 1; i >= 0; i-- {
		if allowedips[i].IP.String() == node.Address.IP.String() || allowedips[i].IP.String() == node.Address6.IP.String() {
			allowedips = append(allowedips[:i], allowedips[i+1:]...)
		}
	}
	//delete egressrange from allowedip if we are egress gateway
	if node.IsEgressGateway {
		for i := len(allowedips) - 1; i >= 0; i-- {
			if StringSliceContains(node.EgressGatewayRanges, allowedips[i].String()) {
				allowedips = append(allowedips[:i], allowedips[i+1:]...)
			}
		}
	}
	//delete extclients from allowedip if we are ingress gateway
	if node.IsIngressGateway {
		for i := len(allowedips) - 1; i >= 0; i-- {
			if strings.Contains(node.IngressGatewayRange, allowedips[i].IP.String()) {
				allowedips = append(allowedips[:i], allowedips[i+1:]...)
			}
		}
	}
	//add egress range if relay is egress
	if relay.IsEgressGateway {
		var ip *net.IPNet
		for _, cidr := range relay.EgressGatewayRanges {
			_, ip, err = net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
		}
		allowedips = append(allowedips, *ip)
	}
	var setUDPPort = false
	var listenPort int
	if CheckEndpoint(udppeers[relayHost.PublicKey.String()]) {
		endpointstring := udppeers[relayHost.PublicKey.String()]
		endpointarr := strings.Split(endpointstring, ":")
		if len(endpointarr) == 2 {
			port, err := strconv.Atoi(endpointarr[1])
			if err == nil {
				setUDPPort = true
				listenPort = port
			}
		}
	}
	// if udp hole punching is on, but udp hole punching did not set it, use the LocalListenPort instead
	// or, if port is for some reason zero use the LocalListenPort
	// but only do this if LocalListenPort is not zero
	if ((!setUDPPort) || relayHost.ListenPort == 0) && relayHost.ListenPort != 0 {
		listenPort = relayHost.ListenPort
	}

	endpoint := relayHost.EndpointIP.String() + ":" + strconv.FormatInt(int64(listenPort), 10)
	address, err := net.ResolveUDPAddr("udp", endpoint)
	if err != nil {
		return models.PeerUpdate{}, err
	}
	var keepalive time.Duration
	if node.PersistentKeepalive != 0 {
		// set_keepalive
		keepalive = node.PersistentKeepalive
	}
	var peerData = wgtypes.PeerConfig{
		PublicKey:                   relayHost.PublicKey,
		Endpoint:                    address,
		ReplaceAllowedIPs:           true,
		AllowedIPs:                  allowedips,
		PersistentKeepaliveInterval: &keepalive,
	}
	peers = append(peers, peerData)
	//if ingress add extclients
	if node.IsIngressGateway {
		extPeers, _, err := getExtPeers(node)
		if err == nil {
			peers = append(peers, extPeers...)
		} else {
			logger.Log(2, "could not retrieve ext peers for ", node.ID.String(), err.Error())
		}
	}
	peerUpdate.Network = node.Network
	peerUpdate.ServerVersion = servercfg.Version
	sort.SliceStable(peers[:], func(i, j int) bool {
		return peers[i].PublicKey.String() < peers[j].PublicKey.String()
	})
	peerUpdate.Peers = peers
	peerUpdate.ServerAddrs = serverNodeAddresses
	peerUpdate.DNS = getPeerDNS(node.Network)
	return peerUpdate, nil
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
		nodeEndpointArr := strings.Split(peerHost.EndpointIP.String(), ":")      // getting the public ip of node
		if ipnet.Contains(net.ParseIP(nodeEndpointArr[0])) && !internetGateway { // ensuring egress gateway range does not contain endpoint of node
			logger.Log(2, "egress IP range of ", iprange, " overlaps with ", host.EndpointIP.String(), ", omitting")
			continue // skip adding egress range if overlaps with node's ip
		}
		// TODO: Could put in a lot of great logic to avoid conflicts / bad routes
		if ipnet.Contains(net.ParseIP(node.LocalAddress.String())) && !internetGateway { // ensuring egress gateway range does not contain public ip of node
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
