package logic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

var (
	// ResetAutoRelay - function to reset autorelayed peers on this node
	ResetAutoRelay = func(ctx context.Context, autoRelayNode *models.Node) error {
		return nil
	}
	// ResetAutoRelayedPeer - removes relayed peers for node
	ResetAutoRelayedPeer = func(ctx context.Context, failedOverNode *models.Node) error {
		return nil
	}
	// GetAutoRelayPeerIps - gets autorelay peerips
	GetAutoRelayPeerIps = func(ctx context.Context, peer, node *models.Node) []net.IPNet {
		return []net.IPNet{}
	}
	// SetAutoRelay - sets autorelay flag on the node
	SetAutoRelay = func(node *models.Node) {
		node.IsAutoRelay = false
	}
)

var (
	hostPeerInfoCache   map[string]map[string]models.HostPeerInfo
	hostPeerInfoCacheMu sync.RWMutex
)

var (
	hostPeerUpdateCache   map[string]map[string]models.HostPeerUpdate
	hostPeerUpdateCacheMu sync.RWMutex
)

// InvalidateHostPeerCaches clears both hostPeerInfoCache and
// hostPeerUpdateCache so they are rebuilt on next access or refresh.
func InvalidateHostPeerCaches(ctx context.Context) {
	tenantID := scope.ID(ctx)

	hostPeerInfoCacheMu.Lock()
	delete(hostPeerInfoCache, tenantID)
	hostPeerInfoCacheMu.Unlock()

	hostPeerUpdateCacheMu.Lock()
	delete(hostPeerUpdateCache, tenantID)
	hostPeerUpdateCacheMu.Unlock()
}

// StoreHostPeerUpdate - caches a computed HostPeerUpdate for a host.
// Called as a side-effect of PublishSingleHostPeerUpdate during broadcast.
func StoreHostPeerUpdate(ctx context.Context, hostID string, peerUpdate models.HostPeerUpdate) {
	tenantID := scope.ID(ctx)
	hostPeerUpdateCacheMu.Lock()
	if hostPeerUpdateCache == nil {
		hostPeerUpdateCache = make(map[string]map[string]models.HostPeerUpdate)
	}
	if hostPeerUpdateCache[tenantID] == nil {
		hostPeerUpdateCache[tenantID] = make(map[string]models.HostPeerUpdate)
	}
	hostPeerUpdateCache[tenantID][hostID] = peerUpdate
	hostPeerUpdateCacheMu.Unlock()
}

// GetCachedHostPeerUpdate - returns a cached HostPeerUpdate if available.
func GetCachedHostPeerUpdate(ctx context.Context, hostID string) (models.HostPeerUpdate, bool) {
	tenantID := scope.ID(ctx)
	hostPeerUpdateCacheMu.RLock()
	defer hostPeerUpdateCacheMu.RUnlock()
	tenantCache, ok := hostPeerUpdateCache[tenantID]
	if !ok {
		return models.HostPeerUpdate{}, false
	}
	hpu, ok := tenantCache[hostID]
	return hpu, ok
}

// GetHostPeerInfo - returns cached peer info for a host.
// Falls back to on-demand computation if the cache is not yet populated.
func GetHostPeerInfo(ctx context.Context, host *schema.Host) (models.HostPeerInfo, error) {
	tenantID := scope.ID(ctx)
	hostID := host.ID.String()
	hostPeerInfoCacheMu.RLock()
	if tenantCache, ok := hostPeerInfoCache[tenantID]; ok {
		if info, ok := tenantCache[hostID]; ok {
			hostPeerInfoCacheMu.RUnlock()
			return info, nil
		}
	}
	hostPeerInfoCacheMu.RUnlock()
	return computeHostPeerInfo(ctx, host, nil, models.ServerConfig{})
}

// RefreshHostPeerInfoCache - batch pre-computes peer info for all hosts
// and stores the results in the cache. Returns the fetched hosts and
// nodes so callers can reuse them without redundant DB queries.
func RefreshHostPeerInfoCache(ctx context.Context) ([]schema.Host, []models.Node) {
	hosts, err := (&schema.Host{}).ListAll(ctx)
	if err != nil {
		slog.Error("failed to refresh host peer info cache", "error", err)
		return nil, nil
	}
	allNodes, err := GetAllNodes(ctx)
	if err != nil {
		slog.Error("failed to refresh host peer info cache", "error", err)
		return nil, nil
	}
	serverInfo := GetServerInfo(ctx)

	newCache := make(map[string]map[string]models.HostPeerInfo)
	for i := range hosts {
		info, err := computeHostPeerInfo(ctx, &hosts[i], allNodes, serverInfo)
		if err != nil {
			continue
		}
		tenantID := hosts[i].TenantID
		if newCache[tenantID] == nil {
			newCache[tenantID] = make(map[string]models.HostPeerInfo)
		}
		newCache[tenantID][hosts[i].ID.String()] = info
	}

	hostPeerInfoCacheMu.Lock()
	hostPeerInfoCache = newCache
	hostPeerInfoCacheMu.Unlock()
	return hosts, allNodes
}

// computeHostPeerInfo - computes peer info for a single host.
// If allNodes is nil or serverInfo is zero-value, fetches them fresh.
func computeHostPeerInfo(ctx context.Context, host *schema.Host, allNodes []models.Node, serverInfo models.ServerConfig) (models.HostPeerInfo, error) {
	peerInfo := models.HostPeerInfo{
		NetworkPeerIDs: make(map[schema.NetworkID]models.PeerMap),
	}
	var err error
	if allNodes == nil {
		allNodes, err = GetAllNodes(ctx)
		if err != nil {
			return peerInfo, err
		}
	}
	if serverInfo.Server == "" {
		serverInfo = GetServerInfo(ctx)
	}
	for _, nodeID := range host.Nodes {
		nodeID := nodeID
		node, err := GetNodeByID(nodeID)
		if err != nil {
			continue
		}

		if !node.Connected || node.PendingDelete || node.Action == schema.NODE_DELETE {
			continue
		}
		networkPeersInfo := make(models.PeerMap)
		defaultDevicePolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(node.Network), models.DevicePolicy)

		currentPeers := GetNetworkNodesMemory(allNodes, node.Network)
		for _, peer := range currentPeers {
			peer := peer
			if peer.ID.String() == node.ID.String() {
				continue
			}

			peerHost := &schema.Host{
				ID: peer.HostID,
			}
			err := peerHost.Get(ctx)
			if err != nil {
				logger.Log(4, "no peer host", peer.HostID.String(), err.Error())
				continue
			}

			var allowedToComm bool
			if defaultDevicePolicy.Enabled {
				allowedToComm = true
			} else {
				allowedToComm = IsPeerAllowed(ctx, node, peer, false)
			}
			if peer.Action != schema.NODE_DELETE &&
				!peer.PendingDelete &&
				peer.Connected &&
				(allowedToComm) {

				networkPeersInfo[peerHost.PublicKey.String()] = models.IDandAddr{
					ID:         peer.ID.String(),
					HostID:     peerHost.ID.String(),
					Address:    peer.PrimaryAddress(),
					Address4:   peer.Address.IP.String(),
					Address6:   peer.Address6.IP.String(),
					Name:       peerHost.Name,
					Network:    peer.Network,
					ListenPort: peerHost.ListenPort,
				}
			}
		}
		var extPeerIDAndAddrs []models.IDandAddr
		if node.IsIngressGateway {
			_, extPeerIDAndAddrs, _, err = GetExtPeers(ctx, &node, &node, make(map[string]models.PeerIdentity))
			if err == nil {
				for _, extPeerIdAndAddr := range extPeerIDAndAddrs {
					networkPeersInfo[extPeerIdAndAddr.ID] = extPeerIdAndAddr
				}
			}
		}

		peerInfo.NetworkPeerIDs[schema.NetworkID(node.Network)] = networkPeersInfo
	}
	return peerInfo, nil
}

// GetPeerUpdateForHost - gets the consolidated peer update for the host from all networks
func GetPeerUpdateForHost(ctx context.Context, network string, host *schema.Host, allNodes []models.Node, deletedHost *schema.Host, deletedNode *models.Node, deletedClients []models.ExtClient) (hostPeerUpdate models.HostPeerUpdate, err error) {
	if host == nil {
		return models.HostPeerUpdate{}, errors.New("host is nil")
	}

	// track which nodes are deleted
	// after peer calculation, if peer not in list, add delete config of peer
	hostPeerUpdate = models.HostPeerUpdate{
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
		PeerIDs:            make(models.PeerMap, 0),
		Peers:              []wgtypes.PeerConfig{},
		NodePeers:          []wgtypes.PeerConfig{},
		HostNetworkInfo:    models.HostInfoMap{},
		ServerConfig:       GetServerInfo(ctx),
		DnsNameservers:     GetNameserversForHost(ctx, host),
		AutoRelayNodes:     make(map[schema.NetworkID][]models.Node),
		GwNodes:            make(map[schema.NetworkID][]models.Node),
		AddressIdentityMap: make(map[string]models.PeerIdentity),
	}
	defer func() {
		hostPeerUpdate.EgressRoutes = deduplicateEgressRoutes(hostPeerUpdate.EgressRoutes)
	}()
	if host.DNS == "no" {
		hostPeerUpdate.ManageDNS = false
	}

	if !GetFeatureFlags().EnableFlowLogs || !GetServerSettings(ctx).EnableFlowLogs {
		host.EnableFlowLogs = false
	}

	defer func() {
		if !hostPeerUpdate.FwUpdate.AllowAll {
			if len(hostPeerUpdate.FwUpdate.AllowedNetworks) > 0 {
				hostPeerUpdate.FwUpdate.EgressInfo["allowed-network-rules"] = models.EgressInfo{
					EgressID:      "allowed-network-rules",
					EgressFwRules: make(map[string]models.AclRule),
				}
			}
			for _, aclRule := range hostPeerUpdate.FwUpdate.AllowedNetworks {
				hostPeerUpdate.FwUpdate.AclRules[aclRule.ID] = aclRule
				hostPeerUpdate.FwUpdate.EgressInfo["allowed-network-rules"].EgressFwRules[aclRule.ID] = aclRule
			}

		}
	}()

	slog.Debug("peer update for host", "hostId", host.ID.String())
	peerIndexMap := make(map[string]int)
	for _, nodeID := range host.Nodes {
		networkAllowAll := true
		nodeID := nodeID
		if nodeID == uuid.Nil.String() {
			continue
		}
		node, err := GetNodeByID(nodeID)
		if err != nil {
			continue
		}

		if !node.Connected || node.PendingDelete || node.Action == schema.NODE_DELETE {
			if deletedNode == nil || deletedNode.ID != node.ID {
				continue
			}
		}
		if host.EnableFlowLogs {
			if node.Address.IP != nil {
				hostPeerUpdate.AddressIdentityMap[node.Address.IP.String()+"/32"] = models.PeerIdentity{
					ID:   node.ID.String(),
					Type: models.PeerType_Node,
					Name: host.Name,
				}
			}

			if node.Address6.IP != nil {
				hostPeerUpdate.AddressIdentityMap[node.Address6.IP.String()+"/128"] = models.PeerIdentity{
					ID:   node.ID.String(),
					Type: models.PeerType_Node,
					Name: host.Name,
				}
			}
		}

		hostPeerUpdate.Nodes = append(hostPeerUpdate.Nodes, node)
		acls, _ := ListAclsByNetwork(ctx, schema.NetworkID(node.Network))
		eli, _ := (&schema.Egress{Network: node.Network}).ListByNetwork(ctx)
		GetNodeEgressInfo(&node, eli, acls)
		ResolveInternetExitRoutingNode(&node)
		egsWithDomain := ListAllByRoutingNodeWithDomain(eli, node.ID.String())
		if len(egsWithDomain) > 0 {
			hostPeerUpdate.EgressWithDomains = append(hostPeerUpdate.EgressWithDomains, egsWithDomain...)
		}
		hostPeerUpdate = SetDefaultGw(node, hostPeerUpdate)
		if !hostPeerUpdate.IsInternetGw {
			hostPeerUpdate.IsInternetGw = IsInternetGw(node) || NodeIsInternetEgressRouter(node.ID.String(), node.Network)
		}
		hostPeerUpdate.DnsNameservers = append(hostPeerUpdate.DnsNameservers, GetEgressDomainNSForNode(ctx, &node)...)
		defaultUserPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(node.Network), models.UserPolicy)
		defaultDevicePolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(node.Network), models.DevicePolicy)
		if (defaultDevicePolicy.Enabled && defaultUserPolicy.Enabled) ||
			(!CheckIfAnyPolicyisUniDirectional(node, acls) &&
				!(node.EgressDetails.IsEgressGateway && len(node.EgressDetails.EgressGatewayRanges) > 0)) {
			aclRule := models.AclRule{
				ID:              fmt.Sprintf("%s-allowed-network-rules", node.ID.String()),
				AllowedProtocol: models.ALL,
				Direction:       models.TrafficDirectionBi,
				Allowed:         true,
				IPList:          []net.IPNet{node.NetworkRange},
				IP6List:         []net.IPNet{node.NetworkRange6},
			}
			if !(defaultDevicePolicy.Enabled && defaultUserPolicy.Enabled) {
				aclRule.Dst = []net.IPNet{node.NetworkRange}
				aclRule.Dst6 = []net.IPNet{node.NetworkRange6}
			}
			hostPeerUpdate.FwUpdate.AllowedNetworks = append(hostPeerUpdate.FwUpdate.AllowedNetworks, aclRule)
		} else {
			networkAllowAll = false
			hostPeerUpdate.FwUpdate.AllowAll = false
			rules := GetAclRulesForNode(ctx, &node)
			if len(hostPeerUpdate.FwUpdate.AclRules) == 0 {
				hostPeerUpdate.FwUpdate.AclRules = rules
			} else {
				for aclID, rule := range rules {
					hostPeerUpdate.FwUpdate.AclRules[aclID] = rule
				}
			}
		}
		currentPeers := GetNetworkNodesMemory(allNodes, node.Network)
		for _, peer := range currentPeers {
			if peer.ID.String() == node.ID.String() {
				// skip yourself
				continue
			}

			peerHost := &schema.Host{
				ID: peer.HostID,
			}
			err := peerHost.Get(ctx)
			if err != nil {
				logger.Log(4, "no peer host", peer.HostID.String(), err.Error())
				continue
			}
			peerConfig := wgtypes.PeerConfig{
				PublicKey:                   peerHost.PublicKey.Key,
				PersistentKeepaliveInterval: &peerHost.PersistentKeepalive,
				ReplaceAllowedIPs:           true,
			}
			GetNodeEgressInfo(&peer, eli, acls)
			if peer.EgressDetails.IsEgressGateway {
				AddEgressInfoToPeerByAccess(&node, &peer, eli, acls, defaultDevicePolicy.Enabled)
			}
			if node.Mutex != nil {
				node.Mutex.Lock()
			}

			peerAutoRelayID, isAutoRelayPeer := node.AutoRelayedPeers[peer.ID.String()]
			if node.Mutex != nil {
				node.Mutex.Unlock()
			}

			if peer.EgressDetails.IsEgressGateway {
				peerKey := peerHost.PublicKey.String()
				if isAutoRelayPeer && peerAutoRelayID != node.ID.String() {
					// get relay host
					autoRelayNode, err := GetNodeByID(peerAutoRelayID)
					if err == nil {
						relayHost := &schema.Host{
							ID: autoRelayNode.HostID,
						}
						err = relayHost.Get(ctx)
						if err == nil {
							peerKey = relayHost.PublicKey.String()
						}
					}
				}
				if peer.IsRelayed && (peer.RelayedBy != node.ID.String()) {
					// get relay host
					relayNode, err := GetNodeByID(peer.RelayedBy)
					if err == nil {
						relayHost := &schema.Host{
							ID: relayNode.HostID,
						}
						err := relayHost.Get(ctx)
						if err == nil {
							peerKey = relayHost.PublicKey.String()
						}
					}
				}

				hostPeerUpdate.EgressRoutes = append(hostPeerUpdate.EgressRoutes, models.EgressNetworkRoutes{
					PeerKey:                peerKey,
					EgressGwAddr:           peer.Address,
					EgressGwAddr6:          peer.Address6,
					NodeAddr:               node.Address,
					NodeAddr6:              node.Address6,
					EgressRanges:           filterConflictingEgressRoutes(node, peer),
					EgressRangesWithMetric: filterConflictingEgressRoutesWithMetric(node, peer),
					Network:                peer.Network,
				})
			}
			if peer.IsIngressGateway {
				hostPeerUpdate.EgressRoutes = append(hostPeerUpdate.EgressRoutes, getExtpeersExtraRoutes(ctx, node)...)
			}
			var allowedToComm bool
			if defaultDevicePolicy.Enabled {
				allowedToComm = true
			} else {
				allowedToComm = IsPeerAllowed(ctx, node, peer, false)
			}

			if (node.IsRelayed && node.RelayedBy != peer.ID.String()) ||
				(peer.IsRelayed && peer.RelayedBy != node.ID.String()) || isAutoRelayPeer {
				// Never remove the peer that is this node's internet exit. Exit
				// clients need that WireGuard peer (with 0.0.0.0/0) even when
				// RelayedBy is temporarily empty/stale after gateway teardown;
				// otherwise netclient IGW monitor fails with "peer not found".
				if usesPeerAsInternetExit(&node, &peer) {
					// fall through to normal peer config
				} else {
					// if node is relayed and peer is not the relay, set remove to true
					if _, ok := peerIndexMap[peerHost.PublicKey.String()]; ok {
						continue
					}
					peerConfig.Remove = true
					hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, peerConfig)
					peerIndexMap[peerHost.PublicKey.String()] = len(hostPeerUpdate.Peers) - 1
					continue
				}
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
			if node.IsRelay && peer.RelayedBy == node.ID.String() && InternetExitRoutingNodeID(&peer) == "" && !peer.IsStatic {
				// don't set endpoint on relayed peer
				peerEndpoint = nil
			}
			if isAutoRelayPeer && peerAutoRelayID == node.ID.String() && !peer.IsStatic {
				peerEndpoint = nil
			}

			peerConfig.Endpoint = &net.UDPAddr{
				IP:   peerEndpoint,
				Port: GetPeerListenPort(peerHost),
			}

			if uselocal {
				peerConfig.Endpoint.Port = peerHost.ListenPort
			}

			if peer.Action != schema.NODE_DELETE &&
				!peer.PendingDelete &&
				peer.Connected &&
				(allowedToComm) &&
				(deletedNode == nil || (peer.ID.String() != deletedNode.ID.String())) {
				peerConfig.AllowedIPs = GetAllowedIPs(ctx, &node, &peer, nil) // only append allowed IPs if valid connection
				if peer.IsAutoRelay {
					hostPeerUpdate.AutoRelayNodes[schema.NetworkID(peer.Network)] = append(hostPeerUpdate.AutoRelayNodes[schema.NetworkID(peer.Network)],
						peer)
				}
				if node.AutoAssignGateway && peer.IsGw {
					hostPeerUpdate.GwNodes[schema.NetworkID(peer.Network)] = append(hostPeerUpdate.GwNodes[schema.NetworkID(peer.Network)],
						peer)
				}

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
			if host.EnableFlowLogs {
				if peer.Address.IP != nil {
					hostPeerUpdate.AddressIdentityMap[peer.Address.IP.String()+"/32"] = models.PeerIdentity{
						ID:   peer.ID.String(),
						Type: models.PeerType_Node,
						Name: peerHost.Name,
					}
				}
				if peer.Address6.IP != nil {
					hostPeerUpdate.AddressIdentityMap[peer.Address6.IP.String()+"/128"] = models.PeerIdentity{
						ID:   peer.ID.String(),
						Type: models.PeerType_Node,
						Name: peerHost.Name,
					}
				}
			}
		}
		var extPeers []wgtypes.PeerConfig
		var extPeerIDAndAddrs []models.IDandAddr
		var egressRoutes []models.EgressNetworkRoutes
		if node.IsIngressGateway {
			hostPeerUpdate.FwUpdate.IsIngressGw = true
			extPeers, extPeerIDAndAddrs, egressRoutes, err = GetExtPeers(ctx, &node, &node, hostPeerUpdate.AddressIdentityMap)
			if err == nil {
				if !defaultDevicePolicy.Enabled || !defaultUserPolicy.Enabled {
					ingFwUpdate := models.IngressInfo{
						IngressID:     node.ID.String(),
						Network:       node.NetworkRange,
						Network6:      node.NetworkRange6,
						StaticNodeIps: GetStaticNodeIps(ctx, node),
						Rules:         GetFwRulesOnIngressGateway(ctx, node),
					}
					ingFwUpdate.EgressRanges, ingFwUpdate.EgressRanges6 = getExtpeerEgressRanges(ctx, node)
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
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				logger.Log(1, "error retrieving external clients:", err.Error())
			}
		}
		if node.EgressDetails.IsEgressGateway && len(node.EgressDetails.EgressGatewayRequest.Ranges) > 0 {
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
				EgressGWCfg:   node.EgressDetails.EgressGatewayRequest,
				EgressFwRules: make(map[string]models.AclRule),
			}
			if host.EnableFlowLogs {
				for _, egressRange := range node.EgressDetails.EgressGatewayRequest.RangesWithMetric {
					if egressRange.EgressID != "" {
						hostPeerUpdate.AddressIdentityMap[egressRange.Network] = models.PeerIdentity{
							ID:   egressRange.EgressID,
							Type: models.PeerType_EgressRoute,
							Name: egressRange.EgressName,
						}
					}
				}
			}
		}
		if node.EgressDetails.IsEgressGateway {
			if !networkAllowAll {
				egressInfo := hostPeerUpdate.FwUpdate.EgressInfo[node.ID.String()]
				if egressInfo.EgressFwRules == nil {
					egressInfo.EgressFwRules = make(map[string]models.AclRule)
				}
				egressInfo.EgressFwRules = GetEgressRulesForNode(ctx, node)
				hostPeerUpdate.FwUpdate.EgressInfo[node.ID.String()] = egressInfo
			} else if defaultDevicePolicy.Enabled && defaultUserPolicy.Enabled {
				if r, ok := GetEgressDefaultAllowAllFwRule(node); ok {
					egressInfo := hostPeerUpdate.FwUpdate.EgressInfo[node.ID.String()]
					if egressInfo.EgressFwRules == nil {
						egressInfo.EgressFwRules = make(map[string]models.AclRule)
					}
					egressInfo.EgressFwRules[r.ID] = r
					hostPeerUpdate.FwUpdate.EgressInfo[node.ID.String()] = egressInfo
				}
			}

		}

		// Publish internet egress firewall config keyed by real egress IDs when present;
		// fall back to legacy synthetic {nodeID}-inet for node-level IGW flags.
		inetEgressesPublished := false
		for _, e := range eli {
			if !e.Status || !IsEgressInternetGateway(e) {
				continue
			}
			if _, ok := e.Nodes[node.ID.String()]; !ok {
				continue
			}
			inetEgressesPublished = true
			hostPeerUpdate.FwUpdate.IsEgressGw = true
			egressrange := ExpandEgressRouteRanges(e, exitHostHasEndpointIPv6(&node))
			rangeWithMetric := []models.EgressRangeMetric{}
			for _, rangeI := range egressrange {
				rangeWithMetric = append(rangeWithMetric, models.EgressRangeMetric{
					EgressID:    e.ID,
					EgressName:  e.Name,
					Network:     rangeI,
					RouteMetric: 256,
					Nat:         e.Nat,
					Mode:        e.Mode,
				})
			}
			inetEgressInfo := models.EgressInfo{
				EgressID: e.ID,
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
					NodeID:           node.ID.String(),
					NetID:            node.Network,
					NatEnabled:       "yes",
					Ranges:           egressrange,
					RangesWithMetric: rangeWithMetric,
				},
			}
			if !networkAllowAll {
				inetEgressInfo.EgressFwRules = GetAclRuleForInetGw(node)
			}
			hostPeerUpdate.FwUpdate.EgressInfo[e.ID] = inetEgressInfo
		}
		if !inetEgressesPublished && IsInternetGw(node) {
			hostPeerUpdate.FwUpdate.IsEgressGw = true
			egressrange := []string{"0.0.0.0/0"}
			if exitHostHasEndpointIPv6(&node) {
				egressrange = append(egressrange, "::/0")
			}
			rangeWithMetric := []models.EgressRangeMetric{}
			for _, rangeI := range egressrange {
				rangeWithMetric = append(rangeWithMetric, models.EgressRangeMetric{
					Network:     rangeI,
					RouteMetric: 256,
					Nat:         true,
					Mode:        schema.DirectNAT,
				})
			}
			inetEgressInfo := models.EgressInfo{
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
					NodeID:           fmt.Sprintf("%s-%s", node.ID.String(), "inet"),
					NetID:            node.Network,
					NatEnabled:       "yes",
					Ranges:           egressrange,
					RangesWithMetric: rangeWithMetric,
				},
			}
			if !networkAllowAll {
				inetEgressInfo.EgressFwRules = GetAclRuleForInetGw(node)
			}
			hostPeerUpdate.FwUpdate.EgressInfo[fmt.Sprintf("%s-%s", node.ID.String(), "inet")] = inetEgressInfo
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
		var deletedNodeHost *schema.Host
		var err error
		if deletedHost == nil {
			deletedNodeHost = &schema.Host{
				ID: deletedNode.HostID,
			}
			err = deletedNodeHost.Get(ctx)
		} else {
			deletedNodeHost = deletedHost
		}
		if err == nil && host.ID != deletedNodeHost.ID {
			if _, ok := peerIndexMap[deletedNodeHost.PublicKey.String()]; !ok {
				hostPeerUpdate.Peers = append(hostPeerUpdate.Peers, wgtypes.PeerConfig{
					PublicKey: deletedNodeHost.PublicKey.Key,
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
func GetPeerListenPort(host *schema.Host) int {
	peerPort := host.ListenPort
	if !host.IsStaticPort && host.WgPublicListenPort != 0 {
		peerPort = host.WgPublicListenPort
	}
	return peerPort
}

func filterConflictingEgressRoutes(node, peer models.Node) []string {
	egressIPs := slices.Clone(peer.EgressDetails.EgressGatewayRanges)
	if !usesPeerAsInternetExit(&node, &peer) {
		egressIPs = withoutDefaultRouteStrings(egressIPs)
	}
	if node.EgressDetails.IsEgressGateway {
		// filter conflicting addrs
		nodeEgressMap := make(map[string]struct{})
		for _, rangeI := range node.EgressDetails.EgressGatewayRanges {
			nodeEgressMap[rangeI] = struct{}{}
		}
		for i := len(egressIPs) - 1; i >= 0; i-- {
			if _, ok := nodeEgressMap[egressIPs[i]]; ok {
				egressIPs = append(egressIPs[:i], egressIPs[i+1:]...)
			}
		}
	}

	return UniqueStrings(egressIPs)
}

func filterConflictingEgressRoutesWithMetric(node, peer models.Node) []models.EgressRangeMetric {
	egressIPs := slices.Clone(peer.EgressDetails.EgressGatewayRequest.RangesWithMetric)
	if !usesPeerAsInternetExit(&node, &peer) {
		filtered := make([]models.EgressRangeMetric, 0, len(egressIPs))
		for _, r := range egressIPs {
			if r.Network == IPv4Network || r.Network == IPv6Network {
				continue
			}
			filtered = append(filtered, r)
		}
		egressIPs = filtered
	}
	if node.EgressDetails.IsEgressGateway {
		// filter conflicting addrs
		nodeEgressMap := make(map[string]struct{})
		for _, rangeI := range node.EgressDetails.EgressGatewayRanges {
			nodeEgressMap[rangeI] = struct{}{}
		}
		for i := len(egressIPs) - 1; i >= 0; i-- {
			// Use virtual network range for conflict detection when virtual NAT is enabled
			checkRange := egressIPs[i].Network
			if egressIPs[i].Nat && egressIPs[i].VirtualNetwork != "" {
				checkRange = egressIPs[i].VirtualNetwork
			}
			if _, ok := nodeEgressMap[checkRange]; ok {
				egressIPs = append(egressIPs[:i], egressIPs[i+1:]...)
			}
		}
	}

	return egressIPs
}

// withoutDefaultRouteStrings strips full-tunnel CIDR strings from egress range lists.
func withoutDefaultRouteStrings(ranges []string) []string {
	if len(ranges) == 0 {
		return ranges
	}
	out := make([]string, 0, len(ranges))
	for _, r := range ranges {
		if r == IPv4Network || r == IPv6Network {
			continue
		}
		out = append(out, r)
	}
	return out
}

// GetAllowedIPs - calculates the wireguard allowedip field for a peer of a node based on the peer and node settings
func GetAllowedIPs(ctx context.Context, node, peer *models.Node, metrics *models.Metrics) []net.IPNet {
	var allowedips []net.IPNet
	allowedips = getNodeAllowedIPs(ctx, peer, node)
	if usesPeerAsInternetExit(node, peer) {
		allowedips = append(allowedips, GetAllowedIpForInetNodeClient(node, peer)...)
		// Exit clients still need explicit overlay routes through the exit node for
		// mesh peers (including peers auto-relayed by this exit). 0.0.0.0/0 alone is
		// not sufficient when auto-relayed peers must accept return traffic / when
		// default-route handling is separate from overlay.
		if node.IsRelayed && node.RelayedBy == peer.ID.String() {
			allowedips = append(allowedips, withoutDefaultRoutes(GetAllowedIpsForRelayed(node, peer))...)
		}
		// handle ingress gateway peers
		if peer.IsIngressGateway {
			extPeers, _, _, err := GetExtPeers(peer, node, make(map[string]models.PeerIdentity))
			if err != nil {
				logger.Log(2, "could not retrieve ext peers for ", peer.ID.String(), err.Error())
			}
			for _, extPeer := range extPeers {
				allowedips = append(allowedips, withoutDefaultRoutes(extPeer.AllowedIPs)...)
			}
		}
		return allowedips
	}
	// Non-exit clients must never inherit default routes from egress/relay aggregation.
	allowedips = withoutDefaultRoutes(allowedips)
	if node.IsRelayed && node.RelayedBy == peer.ID.String() {
		allowedips = append(allowedips, withoutDefaultRoutes(GetAllowedIpsForRelayed(ctx, node, peer))...)
	}

	// handle ingress gateway peers
	if peer.IsIngressGateway {
		extPeers, _, _, err := GetExtPeers(ctx, peer, node, make(map[string]models.PeerIdentity))
		if err != nil {
			logger.Log(2, "could not retrieve ext peers for ", peer.ID.String(), err.Error())
		}
		for _, extPeer := range extPeers {
			allowedips = append(allowedips, withoutDefaultRoutes(extPeer.AllowedIPs)...)
		}
	}

	return allowedips
}

// usesPeerAsInternetExit reports whether node should route full internet through peer
// via selected internet egress or legacy InternetGwID assignment.
func usesPeerAsInternetExit(node, peer *models.Node) bool {
	if node == nil || peer == nil {
		return false
	}
	routingNodeID := InternetExitRoutingNodeID(node)
	return routingNodeID != "" && routingNodeID == peer.ID.String()
}

// isDefaultRoute reports whether ipnet is a full-tunnel default route.
func isDefaultRoute(ipnet net.IPNet) bool {
	s := ipnet.String()
	return s == IPv4Network || s == IPv6Network
}

// withoutDefaultRoutes strips 0.0.0.0/0 and ::/0. Full-tunnel routes must only be
// added via usesPeerAsInternetExit → GetAllowedIpForInetNodeClient.
func withoutDefaultRoutes(ips []net.IPNet) []net.IPNet {
	if len(ips) == 0 {
		return ips
	}
	out := make([]net.IPNet, 0, len(ips))
	for _, ip := range ips {
		if isDefaultRoute(ip) {
			continue
		}
		out = append(out, ip)
	}
	return out
}

func GetEgressIPs(peer *models.Node) []net.IPNet {
	peerHost := &schema.Host{}
	// Skip DB lookup when HostID is unset (e.g. unit tests); EndpointIP overlap
	// checks below simply no-op when peerHost is empty.
	if peer.HostID != uuid.Nil {
		peerHost.ID = peer.HostID
		err := peerHost.Get(db.WithContext(context.TODO()))
		if err != nil {
			logger.Log(0, "error retrieving host for peer", peer.ID.String(), "host id", peer.HostID.String(), err.Error())
		}
	}

	// check for internet gateway
	internetGateway := false
	if slices.Contains(peer.EgressDetails.EgressGatewayRanges, "0.0.0.0/0") || slices.Contains(peer.EgressDetails.EgressGatewayRanges, "::/0") {
		internetGateway = true
	}
	allowedips := []net.IPNet{}
	for _, iprange := range peer.EgressDetails.EgressGatewayRanges { // go through each cidr for egress gateway
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
		allowedips = append(allowedips, *ipnet)
	}
	return allowedips
}

func getNodeAllowedIPs(ctx context.Context, peer, node *models.Node) []net.IPNet {
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
	if peer.EgressDetails.IsEgressGateway {
		// hasGateway = true
		egressIPs := GetEgressIPs(peer)
		if node.EgressDetails.IsEgressGateway {
			// filter conflicting addrs
			nodeEgressMap := make(map[string]struct{})
			for _, rangeI := range node.EgressDetails.EgressGatewayRequest.RangesWithMetric {
				if rangeI.Nat {
					nodeEgressMap[rangeI.VirtualNetwork] = struct{}{}
				} else {
					nodeEgressMap[rangeI.Network] = struct{}{}
				}
			}
			for i := len(egressIPs) - 1; i >= 0; i-- {
				if _, ok := nodeEgressMap[egressIPs[i].String()]; ok {
					egressIPs = append(egressIPs[:i], egressIPs[i+1:]...)
				}
			}
		}
		// Default routes are opt-in via exit selection only (see GetAllowedIPs).
		if !usesPeerAsInternetExit(node, peer) {
			egressIPs = withoutDefaultRoutes(egressIPs)
		}
		allowedips = append(allowedips, egressIPs...)
	}
	// A relay advertises its relayed clients' overlay IPs (and non-default egress
	// ranges) to other peers. Default routes are stripped: full-tunnel is opt-in
	// via usesPeerAsInternetExit → GetAllowedIpForInetNodeClient only.
	if peer.IsRelay {
		allowedips = append(allowedips, withoutDefaultRoutes(RelayedAllowedIPs(peer, node))...)
		// RelayedAllowedIPs only walks RelayedNodes; also advertise exit clients that
		// appear only under InetNodeClientIDs / RelayedIGWClients.
		allowedips = append(allowedips, ExitClientOverlayIPsFromInetClients(peer, node.ID.String())...)
	} else if !usesPeerAsInternetExit(node, peer) {
		// A non-gateway internet exit node also relays the overlay traffic of the
		// peers using it as an exit node (they are kept in RelayedNodes for
		// stability). Advertise ONLY those clients' overlay addresses to other
		// peers so they remain reachable through the exit node.
		//
		// Deliberately do not use RelayedAllowedIPs here: it additionally appends
		// each relayed client's egress ranges (which may be broad, e.g. 0.0.0.0/0),
		// which can cover the exit node's public endpoint and create a WireGuard
		// routing loop, causing the exit node's endpoint to flap to an overlay
		// address. Only third-party peers need these routes; the exit node's own
		// clients already receive a default route to it.
		allowedips = append(allowedips, ExitClientOverlayIPs(ctx, peer, node.ID.String())...)
	}
	if peer.IsAutoRelay {
		allowedips = append(allowedips, withoutDefaultRoutes(GetAutoRelayPeerIps(ctx, peer, node))...)
	}
	return allowedips
}
func deduplicateEgressRoutes(routes []models.EgressNetworkRoutes) []models.EgressNetworkRoutes {
	mergedByKey := make(map[string]int, len(routes))
	result := make([]models.EgressNetworkRoutes, 0, len(routes))
	for _, r := range routes {
		key := r.PeerKey + "|" + r.Network
		if idx, exists := mergedByKey[key]; !exists {
			mergedByKey[key] = len(result)
			result = append(result, r)
		} else {
			result[idx].EgressRanges = UniqueStrings(append(result[idx].EgressRanges, r.EgressRanges...))
			result[idx].EgressRangesWithMetric = mergeUniqueEgressRangeMetrics(
				result[idx].EgressRangesWithMetric,
				r.EgressRangesWithMetric,
			)
		}
	}
	return result
}

func mergeUniqueEgressRangeMetrics(existing, incoming []models.EgressRangeMetric) []models.EgressRangeMetric {
	if len(incoming) == 0 {
		return existing
	}
	seen := make(map[models.EgressRangeMetric]struct{}, len(existing)+len(incoming))
	merged := make([]models.EgressRangeMetric, 0, len(existing)+len(incoming))
	for _, metric := range existing {
		if _, ok := seen[metric]; ok {
			continue
		}
		seen[metric] = struct{}{}
		merged = append(merged, metric)
	}
	for _, metric := range incoming {
		if _, ok := seen[metric]; ok {
			continue
		}
		seen[metric] = struct{}{}
		merged = append(merged, metric)
	}
	return merged
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
