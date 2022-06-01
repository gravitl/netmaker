package logic

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/c-robinson/iplib"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// GetNodePeers - fetches peers for a given node
func GetNodePeers(network *models.Network, nodeid string, excludeRelayed bool, isP2S bool) ([]models.Node, error) {
	var peers []models.Node

	// networkNodes = all nodes in network
	// egressNetworkNodes = all egress gateways in network
	var networkNodes, egressNetworkNodes, err = getNetworkEgressAndNodes(network.NetID)
	if err != nil {
		return peers, nil
	}

	// udppeers = the peers parsed from the local interface
	// gives us correct port to reach
	udppeers, errN := database.GetPeers(network.NetID)
	if errN != nil {
		logger.Log(2, errN.Error())
	}

	// gets all the ACL rules
	currentNetworkACLs, aclErr := nodeacls.FetchAllACLs(nodeacls.NetworkID(network.NetID))
	if aclErr != nil {
		return peers, aclErr
	}

	/*
		at this point we have 4 lists of node information:
		- networkNodes: all nodes in network (models.Node)
		- egressNetworkNodes: all egress gateways in network (models.Node)
		- udppeers: all peers in database (parsed by server off of active WireGuard interface)
		- currentNetworkACLs: all ACL rules associated with the network
		- peers: a currently empty list that will be filled and returned

	*/

	// we now parse through all networkNodes and format properly to set as "peers"
	for _, node := range networkNodes {

		// skip over any node that is disallowed by ACL rules
		if !currentNetworkACLs.IsAllowed(acls.AclID(nodeid), acls.AclID(node.ID)) {
			continue
		}

		// create an empty model to fill with peer info
		var peer = models.Node{}

		// set egress gateway information if it's an egress gateway
		if node.IsEgressGateway == "yes" { // handle egress stuff
			peer.EgressGatewayRanges = node.EgressGatewayRanges
			peer.IsEgressGateway = node.IsEgressGateway
		}

		// set ingress gateway information
		peer.IsIngressGateway = node.IsIngressGateway

		/*
			- similar to ACLs, we must determine if peer is allowed based on Relay information
			- if the nodes is "not relayed" (not behind a relay), it is ok
			- if the node IS relayed, but excludeRelay has not been marked, it is ok
			- excludeRelayed is marked for any node that is NOT a Relay Server
			- therefore, the peer is allowed as long as it is not "relayed", or the node it is being sent to is its relay server
		*/
		allow := node.IsRelayed != "yes" || !excludeRelayed

		// confirm conditions allow node to be added as peer
		// node should be in same network, not pending, and "allowed" based on above logic
		if node.Network == network.NetID && node.IsPending != "yes" && allow {

			// node info is cleansed to remove sensitive info using setPeerInfo
			peer = setPeerInfo(&node)

			// Sets ListenPort to UDP Hole Punching Port assuming:
			// - UDP Hole Punching is enabled
			// - udppeers retrieval did not return an error
			// - the endpoint is valid
			if node.UDPHolePunch == "yes" && errN == nil && CheckEndpoint(udppeers[node.PublicKey]) {
				endpointstring := udppeers[node.PublicKey]
				endpointarr := strings.Split(endpointstring, ":")
				if len(endpointarr) == 2 {
					port, err := strconv.Atoi(endpointarr[1])
					if err == nil {
						peer.ListenPort = int32(port)
					}
				}
			}

			// if udp hole punching is on, but the node's port is still set to default (e.g. 51821), use the LocalListenPort
			// or, if port is for some reason zero use the LocalListenPort
			// but only do this if LocalListenPort is not zero
			if node.UDPHolePunch == "yes" &&
				((peer.ListenPort == node.ListenPort || peer.ListenPort == 0) && node.LocalListenPort != 0) {
				peer.ListenPort = node.LocalListenPort
			}

			// if the node is a relay, append the network cidr and any relayed egress ranges
			if node.IsRelay == "yes" { // TODO, check if addressrange6 needs to be appended
				peer.AllowedIPs = append(peer.AllowedIPs, network.AddressRange)
				for _, egressNode := range egressNetworkNodes {
					if egressNode.IsRelayed == "yes" && StringSliceContains(node.RelayAddrs, egressNode.Address) {
						peer.AllowedIPs = append(peer.AllowedIPs, egressNode.EgressGatewayRanges...)
					}
				}
			}

			// if the node is an ingress gateway, append all the extclient allowedips
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

			// dont appent if this isn't a p2p network or if ACLs disallow
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
	if refnode.IsRelayed != "yes" {
		// if the node is not being relayed, retrieve peers as normal
		peers, err = GetNodePeers(&network, refnode.ID, excludeRelayed, isP2S)
	} else {
		var relayNode models.Node
		// If this node IS being relayed node, we must first retrieve its relay
		relayNode, err = GetNodeRelay(networkName, relayedNodeAddr)
		if relayNode.Address != "" && err == nil {
			// we must cleanse sensitive info from the relay node
			var peerNode = setPeerInfo(&relayNode)

			// we must append the CIDR to the relay so the relayed node can reach the network
			peerNode.AllowedIPs = append(peerNode.AllowedIPs, network.AddressRange)

			// we must append the egress ranges to the relay so the relayed node can reach egress
			var _, egressNetworkNodes, err = getNetworkEgressAndNodes(networkName)
			if err == nil {
				for _, egress := range egressNetworkNodes {
					if egress.Address != relayedNodeAddr {
						peerNode.AllowedIPs = append(peerNode.AllowedIPs, egress.EgressGatewayRanges...)
					}
				}
			}

			// all of this logic is to traverse and get the port of relay server
			/*
				nodepeers, err := GetNodePeers(&network, refnode.ID, false, isP2S)
				if err == nil && peerNode.UDPHolePunch == "yes" {
					for _, nodepeer := range nodepeers {

						// im not sure if this is good either
						if nodepeer.Address == peerNode.Address {
							// peerNode.Endpoint = nodepeer.Endpoint
							peerNode.ListenPort = nodepeer.ListenPort
						}
					}
				}
			*/
			if peerNode.UDPHolePunch == "yes" {
				udppeers, errN := database.GetPeers(network.NetID)
				if errN != nil {
					logger.Log(2, errN.Error())
				} else if CheckEndpoint(udppeers[peerNode.PublicKey]) {
					endpointstring := udppeers[peerNode.PublicKey]
					endpointarr := strings.Split(endpointstring, ":")
					if len(endpointarr) == 2 {
						port, err := strconv.Atoi(endpointarr[1])
						if err == nil {
							peerNode.ListenPort = int32(port)
						}
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

	// #1 Set Keepalive values: set_keepalive
	// #2 Set local address: set_local - could be a LOT BETTER and fix some bugs with additional logic
	// #3 Set allowedips: set_allowedips
	for _, peer := range currentPeers {
		if peer.ID == node.ID {
			//skip yourself
			continue
		}
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
	peerUpdate.ServerVersion = servercfg.Version
	peerUpdate.Peers = peers
	peerUpdate.ServerAddrs = serverNodeAddresses
	peerUpdate.DNS = getPeerDNS(node.Network)
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
		peers = append(peers, peer)
	}
	return peers, nil

}

// GetAllowedIPs - calculates the wireguard allowedip field for a peer of a node based on the peer and node settings
func GetAllowedIPs(node, peer *models.Node) []net.IPNet {
	var allowedips []net.IPNet

	if peer.Address != "" {
		var peeraddr = net.IPNet{
			IP:   net.ParseIP(peer.Address),
			Mask: net.CIDRMask(32, 32),
		}
		allowedips = append(allowedips, peeraddr)
	}

	if peer.Address6 != "" {
		var addr6 = net.IPNet{
			IP:   net.ParseIP(peer.Address6),
			Mask: net.CIDRMask(128, 128),
		}
		allowedips = append(allowedips, addr6)
	}
	// handle manually set peers
	for _, allowedIp := range peer.AllowedIPs {

		// parsing as a CIDR first. If valid CIDR, append
		if _, ipnet, err := net.ParseCIDR(allowedIp); err == nil {
			nodeEndpointArr := strings.Split(node.Endpoint, ":")
			if !ipnet.Contains(net.IP(nodeEndpointArr[0])) && ipnet.IP.String() != peer.Address { // don't need to add an allowed ip that already exists..
				allowedips = append(allowedips, *ipnet)
			}

		} else { // parsing as an IP second. If valid IP, check if ipv4 or ipv6, then append
			if iplib.Version(net.ParseIP(allowedIp)) == 4 && allowedIp != peer.Address {
				ipnet := net.IPNet{
					IP:   net.ParseIP(allowedIp),
					Mask: net.CIDRMask(32, 32),
				}
				allowedips = append(allowedips, ipnet)
			} else if iplib.Version(net.ParseIP(allowedIp)) == 6 && allowedIp != peer.Address6 {
				ipnet := net.IPNet{
					IP:   net.ParseIP(allowedIp),
					Mask: net.CIDRMask(128, 128),
				}
				allowedips = append(allowedips, ipnet)
			}
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
				logger.Log(1, "error encountered when setting egress range", err.Error())
			} else {
				allowedips = append(allowedips, *ipnet)
			}
		}
	}
	return allowedips
}

func getPeerDNS(network string) string {
	var dns string
	if nodes, err := GetNetworkNodes(network); err == nil {
		for i := range nodes {
			dns = dns + fmt.Sprintf("%s %s.%s\n", nodes[i].Address, nodes[i].Name, nodes[i].Network)
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
