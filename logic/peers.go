package logic

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// GetHubPeer - in HubAndSpoke networks, if not the hub, return the hub
/*
func GetHubPeer(networkName string) []models.Node {
	var hubpeer = make([]models.Node, 0)
	servernodes, err := GetNetworkNodes(networkName)
	if err != nil {
		return hubpeer
	}
	for i := range servernodes {
		if servernodes[i].IsHub == "yes" {
			return []models.Node{servernodes[i]}
		}
	}
	return hubpeer
}
*/

// GetNodePeers - fetches peers for a given node
func GetNodePeers(network *models.Network, nodeid string, excludeRelayed bool, isP2S bool) ([]models.Node, error) {
	var peers []models.Node
	var networkNodes, egressNetworkNodes, err = getNetworkEgressAndNodes(network.NetID)
	if err != nil {
		return peers, nil
	}

	udppeers, errN := database.GetPeers(network.NetID)
	if errN != nil {
		logger.Log(2, errN.Error())
	}

	currentNetworkACLs, aclErr := nodeacls.FetchAllACLs(nodeacls.NetworkID(network.NetID))
	if aclErr != nil {
		return peers, aclErr
	}

	for _, node := range networkNodes {
		if !currentNetworkACLs.IsAllowed(acls.AclID(nodeid), acls.AclID(node.ID)) {
			continue
		}

		var peer = models.Node{}
		if node.IsEgressGateway == "yes" { // handle egress stuff
			peer.EgressGatewayRanges = node.EgressGatewayRanges
			peer.IsEgressGateway = node.IsEgressGateway
		}

		peer.IsIngressGateway = node.IsIngressGateway
		allow := node.IsRelayed != "yes" || !excludeRelayed

		if node.Network == network.NetID && node.IsPending != "yes" && allow {
			peer = setPeerInfo(&node)
			if node.UDPHolePunch == "yes" && errN == nil && CheckEndpoint(udppeers[node.PublicKey]) {
				endpointstring := udppeers[node.PublicKey]
				endpointarr := strings.Split(endpointstring, ":")
				if len(endpointarr) == 2 {
					port, err := strconv.Atoi(endpointarr[1])
					if err == nil {
						// peer.Endpoint = endpointarr[0]
						peer.ListenPort = int32(port)
					}
				}
			}
			// if udp hole punching is on, but port is still set to default (e.g. 51821), use the LocalListenPort
			if node.UDPHolePunch == "yes" && node.IsStatic != "yes" && peer.ListenPort == node.ListenPort {
				peer.ListenPort = node.LocalListenPort
			}
			if node.IsRelay == "yes" {
				peer.AllowedIPs = append(peer.AllowedIPs, network.AddressRange)
				for _, egressNode := range egressNetworkNodes {
					if egressNode.IsRelayed == "yes" && StringSliceContains(node.RelayAddrs, egressNode.Address) {
						peer.AllowedIPs = append(peer.AllowedIPs, egressNode.EgressGatewayRanges...)
					}
				}
			}
			if peer.IsIngressGateway == "yes" { // handle ingress stuff
				if currentExtClients, err := GetExtPeersList(&node); err == nil {
					for i := range currentExtClients {
						if network.IsIPv4 == "yes" && currentExtClients[i].Address != "" {
							peer.AllowedIPs = append(peer.AllowedIPs, currentExtClients[i].Address)
						}
						if network.IsIPv6 == "yes" && currentExtClients[i].Address6 != "" {
							peer.AllowedIPs = append(peer.AllowedIPs, currentExtClients[i].Address6)
						}
					}
				}
			}

			if (!isP2S || peer.IsHub == "yes") && currentNetworkACLs.IsAllowed(acls.AclID(nodeid), acls.AclID(node.ID)) {
				peers = append(peers, peer)
			}
		}
	}

	return peers, err
}

// GetPeersList - gets the peers of a given network
func GetPeersList(refnode *models.Node) ([]models.Node, error) {
	var peers []models.Node
	var err error
	var isP2S bool
	var networkName = refnode.Network
	var excludeRelayed = refnode.IsRelay != "yes"
	var relayedNodeAddr string
	if refnode.IsRelayed == "yes" {
		relayedNodeAddr = refnode.Address
	}

	network, err := GetNetwork(networkName)
	if err != nil {
		return peers, err
	} else if network.IsPointToSite == "yes" && refnode.IsHub != "yes" {
		isP2S = true
	}
	if relayedNodeAddr == "" {
		peers, err = GetNodePeers(&network, refnode.ID, excludeRelayed, isP2S)
	} else {
		var relayNode models.Node
		relayNode, err = GetNodeRelay(networkName, relayedNodeAddr)
		if relayNode.Address != "" {
			var peerNode = setPeerInfo(&relayNode)
			network, err := GetNetwork(networkName)
			if err == nil {
				peerNode.AllowedIPs = append(peerNode.AllowedIPs, network.AddressRange)
				var _, egressNetworkNodes, err = getNetworkEgressAndNodes(networkName)
				if err == nil {
					for _, egress := range egressNetworkNodes {
						if egress.Address != relayedNodeAddr {
							peerNode.AllowedIPs = append(peerNode.AllowedIPs, egress.EgressGatewayRanges...)
						}
					}
				}
			} else {
				peerNode.AllowedIPs = append(peerNode.AllowedIPs, peerNode.RelayAddrs...)
			}
			nodepeers, err := GetNodePeers(&network, refnode.ID, false, isP2S)
			if err == nil && peerNode.UDPHolePunch == "yes" {
				for _, nodepeer := range nodepeers {
					if nodepeer.Address == peerNode.Address {
						// peerNode.Endpoint = nodepeer.Endpoint
						peerNode.ListenPort = nodepeer.ListenPort
					}
				}
			}
			if !isP2S || peerNode.IsHub == "yes" {
				peers = append(peers, peerNode)
			}
		}
	}
	return peers, err
}

// GetPeerUpdate - gets a wireguard peer config for each peer of a node
func GetPeerUpdate(node *models.Node) (models.PeerUpdate, error) {
	var peerUpdate models.PeerUpdate
	var peers []wgtypes.PeerConfig
	var serverNodeAddresses = []models.ServerAddr{}
	currentPeers, err := GetPeers(node)
	if err != nil {
		return models.PeerUpdate{}, err
	}
	// begin translating netclient logic
	/*


		Go through netclient code and put below



	*/
	// #1 Set Keepalive values: set_keepalive
	// #2 Set local address: set_local - could be a LOT BETTER and fix some bugs with additional logic
	// #3 Set allowedips: set_allowedips
	var dns string
	for _, peer := range currentPeers {
		if peer.ID == node.ID {
			//skip yourself
			continue
		}
		dns = dns + fmt.Sprintf("%s %s.%s\n", peer.Address, peer.Name, peer.Network)
		pubkey, err := wgtypes.ParseKey(peer.PublicKey)
		if err != nil {
			return models.PeerUpdate{}, err
		}
		if node.Endpoint == peer.Endpoint {
			//peer is on same network
			// set_local
			if node.LocalAddress != peer.LocalAddress && peer.LocalAddress != "" {
				peer.Endpoint = peer.LocalAddress
				if peer.LocalListenPort != 0 {
					peer.ListenPort = peer.LocalListenPort
				}
			} else {
				continue
			}
		}
		endpoint := peer.Endpoint + ":" + strconv.FormatInt(int64(peer.ListenPort), 10)
		address, err := net.ResolveUDPAddr("udp", endpoint)
		if err != nil {
			return models.PeerUpdate{}, err
		}
		// set_allowedips
		allowedips := GetAllowedIPs(node, &peer)
		var keepalive time.Duration
		if node.PersistentKeepalive != 0 {
			// set_keepalive
			keepalive, _ = time.ParseDuration(strconv.FormatInt(int64(node.PersistentKeepalive), 10) + "s")
		}
		var peerData = wgtypes.PeerConfig{
			PublicKey:                   pubkey,
			Endpoint:                    address,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  allowedips,
			PersistentKeepaliveInterval: &keepalive,
		}
		peers = append(peers, peerData)
		if peer.IsServer == "yes" {
			serverNodeAddresses = append(serverNodeAddresses, models.ServerAddr{IsLeader: IsLeader(&peer), Address: peer.Address})
		}
	}
	if node.IsIngressGateway == "yes" {
		extPeers, err := getExtPeers(node)
		if err == nil {
			peers = append(peers, extPeers...)
		} else {
			log.Println("ERROR RETRIEVING EXTERNAL PEERS", err)
		}
	}
	peerUpdate.Network = node.Network
	peerUpdate.Peers = peers
	peerUpdate.ServerAddrs = serverNodeAddresses
	/*


		End translation of netclient code


	*/
	if customDNSEntries, err := GetCustomDNS(peerUpdate.Network); err == nil {
		for _, entry := range customDNSEntries {
			// TODO - filter entries based on ACLs / given peers vs nodes in network
			dns = dns + fmt.Sprintf("%s %s.%s\n", entry.Address, entry.Name, entry.Network)
		}
	}
	peerUpdate.DNS = dns
	return peerUpdate, nil
}

func getExtPeers(node *models.Node) ([]wgtypes.PeerConfig, error) {
	var peers []wgtypes.PeerConfig
	extPeers, err := GetExtPeersList(node)
	if err != nil {
		return peers, err
	}
	for _, extPeer := range extPeers {
		pubkey, err := wgtypes.ParseKey(extPeer.PublicKey)
		if err != nil {
			logger.Log(1, "error parsing ext pub key:", err.Error())
			continue
		}

		if node.PublicKey == extPeer.PublicKey {
			continue
		}

		var peer wgtypes.PeerConfig
		var peeraddr = net.IPNet{
			IP:   net.ParseIP(extPeer.Address),
			Mask: net.CIDRMask(32, 32),
		}
		var allowedips []net.IPNet
		allowedips = append(allowedips, peeraddr)

		if extPeer.Address6 != "" {
			var addr6 = net.IPNet{
				IP:   net.ParseIP(extPeer.Address6),
				Mask: net.CIDRMask(128, 128),
			}
			allowedips = append(allowedips, addr6)
		}
		peer = wgtypes.PeerConfig{
			PublicKey:         pubkey,
			ReplaceAllowedIPs: true,
			AllowedIPs:        allowedips,
		}
		peers = append(peers, peer)
	}
	return peers, nil

}

// GetAllowedIPs - calculates the wireguard allowedip field for a peer of a node based on the peer and node settings
func GetAllowedIPs(node, peer *models.Node) []net.IPNet {
	var allowedips []net.IPNet
	var peeraddr = net.IPNet{
		IP:   net.ParseIP(peer.Address),
		Mask: net.CIDRMask(32, 32),
	}
	dualstack := false
	allowedips = append(allowedips, peeraddr)
	// handle manually set peers
	for _, allowedIp := range peer.AllowedIPs {
		if _, ipnet, err := net.ParseCIDR(allowedIp); err == nil {
			nodeEndpointArr := strings.Split(node.Endpoint, ":")
			if !ipnet.Contains(net.IP(nodeEndpointArr[0])) && ipnet.IP.String() != peer.Address { // don't need to add an allowed ip that already exists..
				allowedips = append(allowedips, *ipnet)
			}
		} else if appendip := net.ParseIP(allowedIp); appendip != nil && allowedIp != peer.Address {
			ipnet := net.IPNet{
				IP:   net.ParseIP(allowedIp),
				Mask: net.CIDRMask(32, 32),
			}
			allowedips = append(allowedips, ipnet)
		}
	}
	// handle egress gateway peers
	if peer.IsEgressGateway == "yes" {
		//hasGateway = true
		ranges := peer.EgressGatewayRanges
		for _, iprange := range ranges { // go through each cidr for egress gateway
			_, ipnet, err := net.ParseCIDR(iprange) // confirming it's valid cidr
			if err != nil {
				logger.Log(1, "could not parse gateway IP range. Not adding ", iprange)
				continue // if can't parse CIDR
			}
			nodeEndpointArr := strings.Split(peer.Endpoint, ":") // getting the public ip of node
			if ipnet.Contains(net.ParseIP(nodeEndpointArr[0])) { // ensuring egress gateway range does not contain endpoint of node
				logger.Log(2, "egress IP range of ", iprange, " overlaps with ", node.Endpoint, ", omitting")
				continue // skip adding egress range if overlaps with node's ip
			}
			// TODO: Could put in a lot of great logic to avoid conflicts / bad routes
			if ipnet.Contains(net.ParseIP(node.LocalAddress)) { // ensuring egress gateway range does not contain public ip of node
				logger.Log(2, "egress IP range of ", iprange, " overlaps with ", node.LocalAddress, ", omitting")
				continue // skip adding egress range if overlaps with node's local ip
			}
			if err != nil {
				log.Println("ERROR ENCOUNTERED SETTING GATEWAY")
			} else {
				allowedips = append(allowedips, *ipnet)
			}
		}
	}
	if peer.Address6 != "" && dualstack {
		var addr6 = net.IPNet{
			IP:   net.ParseIP(peer.Address6),
			Mask: net.CIDRMask(128, 128),
		}
		allowedips = append(allowedips, addr6)
	}
	return allowedips
}
