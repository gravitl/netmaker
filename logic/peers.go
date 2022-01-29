package logic

import (
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// GetPeerUpdate - gets a wireguard peer config for each peer of a node
func GetPeerUpdate(node *models.Node) (models.PeerUpdate, error) {
	var peerUpdate models.PeerUpdate
	var peers []wgtypes.PeerConfig
	networkNodes, err := GetNetworkNodes(node.Network)
	if err != nil {
		return models.PeerUpdate{}, err
	}
	var serverNodeAddresses = []models.ServerAddr{}
	for _, peer := range networkNodes {
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
			if node.LocalAddress != peer.LocalAddress && peer.LocalAddress != "" {
				peer.Endpoint = peer.LocalAddress
			} else {
				continue
			}
		}
		endpoint := peer.Endpoint + ":" + strconv.FormatInt(int64(peer.ListenPort), 10)
		address, err := net.ResolveUDPAddr("udp", endpoint)
		if err != nil {
			return models.PeerUpdate{}, err
		}
		allowedips := GetAllowedIPs(node, &peer)
		var keepalive time.Duration
		if node.PersistentKeepalive != 0 {
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
	peerUpdate.Network = node.Network
	peerUpdate.Peers = peers
	peerUpdate.ServerAddrs = serverNodeAddresses
	return peerUpdate, nil
}

// GetAllowedIPs - calculates the wireguard allowedip field for a peer of a node based on the peer and node settings
func GetAllowedIPs(node, peer *models.Node) []net.IPNet {
	var allowedips []net.IPNet
	var gateways []string
	var peeraddr = net.IPNet{
		IP:   net.ParseIP(peer.Address),
		Mask: net.CIDRMask(32, 32),
	}
	dualstack := false
	allowedips = append(allowedips, peeraddr)
	// handle manually set peers
	for _, allowedIp := range node.AllowedIPs {
		if _, ipnet, err := net.ParseCIDR(allowedIp); err == nil {
			nodeEndpointArr := strings.Split(node.Endpoint, ":")
			if !ipnet.Contains(net.IP(nodeEndpointArr[0])) && ipnet.IP.String() != node.Address { // don't need to add an allowed ip that already exists..
				allowedips = append(allowedips, *ipnet)
			}
		} else if appendip := net.ParseIP(allowedIp); appendip != nil && allowedIp != node.Address {
			ipnet := net.IPNet{
				IP:   net.ParseIP(allowedIp),
				Mask: net.CIDRMask(32, 32),
			}
			allowedips = append(allowedips, ipnet)
		}
	}
	// handle egress gateway peers
	if node.IsEgressGateway == "yes" {
		//hasGateway = true
		ranges := node.EgressGatewayRanges
		for _, iprange := range ranges { // go through each cidr for egress gateway
			_, ipnet, err := net.ParseCIDR(iprange) // confirming it's valid cidr
			if err != nil {
				ncutils.PrintLog("could not parse gateway IP range. Not adding "+iprange, 1)
				continue // if can't parse CIDR
			}
			nodeEndpointArr := strings.Split(node.Endpoint, ":") // getting the public ip of node
			if ipnet.Contains(net.ParseIP(nodeEndpointArr[0])) { // ensuring egress gateway range does not contain public ip of node
				ncutils.PrintLog("egress IP range of "+iprange+" overlaps with "+node.Endpoint+", omitting", 2)
				continue // skip adding egress range if overlaps with node's ip
			}
			if ipnet.Contains(net.ParseIP(node.LocalAddress)) { // ensuring egress gateway range does not contain public ip of node
				ncutils.PrintLog("egress IP range of "+iprange+" overlaps with "+node.LocalAddress+", omitting", 2)
				continue // skip adding egress range if overlaps with node's local ip
			}
			gateways = append(gateways, iprange)
			if err != nil {
				log.Println("ERROR ENCOUNTERED SETTING GATEWAY")
			} else {
				allowedips = append(allowedips, *ipnet)
			}
		}
	}
	if node.Address6 != "" && dualstack {
		var addr6 = net.IPNet{
			IP:   net.ParseIP(node.Address6),
			Mask: net.CIDRMask(128, 128),
		}
		allowedips = append(allowedips, addr6)
	}
	return allowedips
}
