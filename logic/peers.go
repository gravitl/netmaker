package logic

import (
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

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
				ncutils.PrintLog("could not parse gateway IP range. Not adding "+iprange, 1)
				continue // if can't parse CIDR
			}
			nodeEndpointArr := strings.Split(peer.Endpoint, ":") // getting the public ip of node
			if ipnet.Contains(net.ParseIP(nodeEndpointArr[0])) { // ensuring egress gateway range does not contain endpoint of node
				ncutils.PrintLog("egress IP range of "+iprange+" overlaps with "+node.Endpoint+", omitting", 2)
				continue // skip adding egress range if overlaps with node's ip
			}
			// TODO: Could put in a lot of great logic to avoid conflicts / bad routes
			if ipnet.Contains(net.ParseIP(node.LocalAddress)) { // ensuring egress gateway range does not contain public ip of node
				ncutils.PrintLog("egress IP range of "+iprange+" overlaps with "+node.LocalAddress+", omitting", 2)
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
