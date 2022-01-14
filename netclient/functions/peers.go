package functions

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

func CalculatePeers(thisNode models.Node, peernodes []models.Node, dualstack, egressgateway, server string) ([]wgtypes.Peer, error) {
	//hasGateway := false
	var gateways []string
	var peers []wgtypes.Peer

	keepalive := thisNode.PersistentKeepalive
	keepalivedur, _ := time.ParseDuration(strconv.FormatInt(int64(keepalive), 10) + "s")
	keepaliveserver, err := time.ParseDuration(strconv.FormatInt(int64(5), 10) + "s")
	if err != nil {
		log.Fatalf("Issue with format of keepalive value. Please update netconfig: %v", err)
	}
	for _, node := range peernodes {
		pubkey, err := wgtypes.ParseKey(node.PublicKey)
		if err != nil {
			log.Println("error parsing key")
			//return peers, hasGateway, gateways, err
		}

		if thisNode.PublicKey == node.PublicKey {
			continue
		}
		if thisNode.Endpoint == node.Endpoint {
			if thisNode.LocalAddress != node.LocalAddress && node.LocalAddress != "" {
				node.Endpoint = node.LocalAddress
			} else {
				continue
			}
		}

		var peer wgtypes.Peer
		var peeraddr = net.IPNet{
			IP:   net.ParseIP(node.Address),
			Mask: net.CIDRMask(32, 32),
		}
		var allowedips []net.IPNet
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
				if ipnet.Contains(net.ParseIP(thisNode.LocalAddress)) { // ensuring egress gateway range does not contain public ip of node
					ncutils.PrintLog("egress IP range of "+iprange+" overlaps with "+thisNode.LocalAddress+", omitting", 2)
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
		if node.Address6 != "" && dualstack == "yes" {
			var addr6 = net.IPNet{
				IP:   net.ParseIP(node.Address6),
				Mask: net.CIDRMask(128, 128),
			}
			allowedips = append(allowedips, addr6)
		}
		if thisNode.IsServer == "yes" && !(node.IsServer == "yes") {
			peer = wgtypes.Peer{
				PublicKey:                   pubkey,
				PersistentKeepaliveInterval: keepaliveserver,
				AllowedIPs:                  allowedips,
			}
		} else if keepalive != 0 {
			peer = wgtypes.Peer{
				PublicKey:                   pubkey,
				PersistentKeepaliveInterval: keepalivedur,
				Endpoint: &net.UDPAddr{
					IP:   net.ParseIP(node.Endpoint),
					Port: int(node.ListenPort),
				},
				AllowedIPs: allowedips,
			}
		} else {
			peer = wgtypes.Peer{
				PublicKey: pubkey,
				Endpoint: &net.UDPAddr{
					IP:   net.ParseIP(node.Endpoint),
					Port: int(node.ListenPort),
				},
				AllowedIPs: allowedips,
			}
		}
		peers = append(peers, peer)
	}
	return peers, nil
}

func CalculateExtPeers(thisNode models.Node, extPeers []models.ExtPeersResponse) ([]wgtypes.Peer, error) {
	var peers []wgtypes.Peer
	var err error
	for _, extPeer := range extPeers {
		pubkey, err := wgtypes.ParseKey(extPeer.PublicKey)
		if err != nil {
			log.Println("error parsing key")
			return peers, err
		}

		if thisNode.PublicKey == extPeer.PublicKey {
			continue
		}

		var peer wgtypes.Peer
		var peeraddr = net.IPNet{
			IP:   net.ParseIP(extPeer.Address),
			Mask: net.CIDRMask(32, 32),
		}
		var allowedips []net.IPNet
		allowedips = append(allowedips, peeraddr)

		if extPeer.Address6 != "" && thisNode.IsDualStack == "yes" {
			var addr6 = net.IPNet{
				IP:   net.ParseIP(extPeer.Address6),
				Mask: net.CIDRMask(128, 128),
			}
			allowedips = append(allowedips, addr6)
		}
		peer = wgtypes.Peer{
			PublicKey:  pubkey,
			AllowedIPs: allowedips,
		}
		peers = append(peers, peer)
	}
	return peers, err
}
