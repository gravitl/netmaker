package logic

import (
	"errors"
	"log"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// == Join, Checkin, and Leave for Server ==
func ServerJoin(cfg config.ClientConfig, privateKey string) error {
	var err error

	if cfg.Network == "" {
		return errors.New("no network provided")
	}

	if cfg.Node.LocalRange != "" && cfg.Node.LocalAddress == "" {
		Log("local vpn, getting local address from range: "+cfg.Node.LocalRange, 1)
		cfg.Node.LocalAddress = GetLocalIP(cfg.Node)
	}

	if cfg.Node.Endpoint == "" {
		if cfg.Node.IsLocal == "yes" && cfg.Node.LocalAddress != "" {
			cfg.Node.Endpoint = cfg.Node.LocalAddress
		} else {
			cfg.Node.Endpoint, err = ncutils.GetPublicIP()
		}
		if err != nil || cfg.Node.Endpoint == "" {
			ncutils.Log("Error setting cfg.Node.Endpoint.")
			return err
		}
	}

	// Generate and set public/private WireGuard Keys
	if privateKey == "" {
		wgPrivatekey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			Log(err.Error(), 1)
			return err
		}
		privateKey = wgPrivatekey.String()
		cfg.Node.PublicKey = wgPrivatekey.PublicKey().String()
	}

	if cfg.Node.MacAddress == "" {
		macs, err := ncutils.GetMacAddr()
		if err != nil {
			return err
		} else if len(macs) == 0 {
			Log("could not retrieve mac address for server", 1)
			return errors.New("failed to get server mac")
		} else {
			cfg.Node.MacAddress = macs[0]
		}
	}

	var node models.Node // fill this node with appropriate calls
	var postnode *models.Node
	postnode = &models.Node{
		Password:            cfg.Node.Password,
		MacAddress:          cfg.Node.MacAddress,
		AccessKey:           cfg.Server.AccessKey,
		Network:             cfg.Network,
		ListenPort:          cfg.Node.ListenPort,
		PostUp:              cfg.Node.PostUp,
		PostDown:            cfg.Node.PostDown,
		PersistentKeepalive: cfg.Node.PersistentKeepalive,
		LocalAddress:        cfg.Node.LocalAddress,
		Interface:           cfg.Node.Interface,
		PublicKey:           cfg.Node.PublicKey,
		DNSOn:               cfg.Node.DNSOn,
		Name:                cfg.Node.Name,
		Endpoint:            cfg.Node.Endpoint,
		SaveConfig:          cfg.Node.SaveConfig,
		UDPHolePunch:        cfg.Node.UDPHolePunch,
	}

	Log("adding a server instance on network "+postnode.Network, 2)
	node, err = CreateNode(*postnode, cfg.Network)
	if err != nil {
		return err
	}
	err = SetNetworkNodesLastModified(node.Network)
	if err != nil {
		return err
	}

	// get free port based on returned default listen port
	node.ListenPort, err = ncutils.GetFreePort(node.ListenPort)
	if err != nil {
		Log("Error retrieving port: "+err.Error(), 2)
	}

	// safety check. If returned node from server is local, but not currently configured as local, set to local addr
	if cfg.Node.IsLocal != "yes" && node.IsLocal == "yes" && node.LocalRange != "" {
		node.LocalAddress, err = ncutils.GetLocalIP(node.LocalRange)
		if err != nil {
			return err
		}
		node.Endpoint = node.LocalAddress
	}

	node.SetID()
	if err = StorePrivKey(node.ID, privateKey); err != nil {
		return err
	}
	if err = ServerPush(node.MacAddress, node.Network); err != nil {
		return err
	}

	peers, hasGateway, gateways, err := GetServerPeers(node.MacAddress, cfg.Network, cfg.Server.GRPCAddress, node.IsDualStack == "yes", node.IsIngressGateway == "yes", node.IsServer == "yes")
	if err != nil && !ncutils.IsEmptyRecord(err) {
		ncutils.Log("failed to retrieve peers")
		return err
	}

	err = initWireguard(&node, privateKey, peers, hasGateway, gateways)
	if err != nil {
		return err
	}

	return nil
}

// ServerPush - pushes config changes for server checkins/join
func ServerPush(mac string, network string) error {

	var serverNode models.Node
	var err error
	serverNode, err = GetNode(mac, network)
	if err != nil && !ncutils.IsEmptyRecord(err) {
		return err
	}
	serverNode.OS = runtime.GOOS
	serverNode.SetLastCheckIn()
	err = serverNode.Update(&serverNode)
	return err
}

func GetServerPeers(macaddress string, network string, server string, dualstack bool, isIngressGateway bool, isServer bool) ([]wgtypes.PeerConfig, bool, []string, error) {
	hasGateway := false
	var err error
	var gateways []string
	var peers []wgtypes.PeerConfig
	var nodecfg models.Node
	var nodes []models.Node // fill above fields from server or client

	nodecfg, err = GetNode(macaddress, network)
	if err != nil {
		return nil, hasGateway, gateways, err
	}
	nodes, err = GetPeers(nodecfg)
	if err != nil {
		return nil, hasGateway, gateways, err
	}

	keepalive := nodecfg.PersistentKeepalive
	keepalivedur, err := time.ParseDuration(strconv.FormatInt(int64(keepalive), 10) + "s")
	keepaliveserver, err := time.ParseDuration(strconv.FormatInt(int64(5), 10) + "s")
	if err != nil {
		Log("Issue with format of keepalive value. Please update netconfig: "+err.Error(), 1)
		return nil, hasGateway, gateways, err
	}

	for _, node := range nodes {
		pubkey, err := wgtypes.ParseKey(node.PublicKey)
		if err != nil {
			Log("error parsing key "+pubkey.String(), 1)
			return peers, hasGateway, gateways, err
		}

		if nodecfg.PublicKey == node.PublicKey {
			continue
		}
		if nodecfg.Endpoint == node.Endpoint {
			if nodecfg.LocalAddress != node.LocalAddress && node.LocalAddress != "" {
				node.Endpoint = node.LocalAddress
			} else {
				continue
			}
		}

		var peer wgtypes.PeerConfig
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
			hasGateway = true
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
				if ipnet.Contains(net.ParseIP(nodecfg.LocalAddress)) { // ensuring egress gateway range does not contain public ip of node
					ncutils.PrintLog("egress IP range of "+iprange+" overlaps with "+nodecfg.LocalAddress+", omitting", 2)
					continue // skip adding egress range if overlaps with node's local ip
				}
				gateways = append(gateways, iprange)
				if err != nil {
					Log("ERROR ENCOUNTERED SETTING GATEWAY", 1)
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
		if nodecfg.IsServer == "yes" && !(node.IsServer == "yes") {
			peer = wgtypes.PeerConfig{
				PublicKey:                   pubkey,
				PersistentKeepaliveInterval: &keepaliveserver,
				ReplaceAllowedIPs:           true,
				AllowedIPs:                  allowedips,
			}
		} else if keepalive != 0 {
			peer = wgtypes.PeerConfig{
				PublicKey:                   pubkey,
				PersistentKeepaliveInterval: &keepalivedur,
				Endpoint: &net.UDPAddr{
					IP:   net.ParseIP(node.Endpoint),
					Port: int(node.ListenPort),
				},
				ReplaceAllowedIPs: true,
				AllowedIPs:        allowedips,
			}
		} else {
			peer = wgtypes.PeerConfig{
				PublicKey: pubkey,
				Endpoint: &net.UDPAddr{
					IP:   net.ParseIP(node.Endpoint),
					Port: int(node.ListenPort),
				},
				ReplaceAllowedIPs: true,
				AllowedIPs:        allowedips,
			}
		}
		peers = append(peers, peer)
	}
	if isIngressGateway {
		extPeers, err := GetServerExtPeers(macaddress, network, server, dualstack)
		if err == nil {
			peers = append(peers, extPeers...)
		} else {
			Log("ERROR RETRIEVING EXTERNAL PEERS ON SERVER", 1)
		}
	}
	return peers, hasGateway, gateways, err
}

// GetServerExtPeers - gets the extpeers for a client
func GetServerExtPeers(macaddress string, network string, server string, dualstack bool) ([]wgtypes.PeerConfig, error) {
	var peers []wgtypes.PeerConfig
	var nodecfg models.Node
	var extPeers []models.Node
	var err error
	// fill above fields from either client or server

	// fill extPeers with server side logic
	nodecfg, err = GetNode(macaddress, network)
	if err != nil {
		return nil, err
	}
	var tempPeers []models.ExtPeersResponse
	tempPeers, err = GetExtPeersList(nodecfg.MacAddress, nodecfg.Network)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(tempPeers); i++ {
		extPeers = append(extPeers, models.Node{
			Address:             tempPeers[i].Address,
			Address6:            tempPeers[i].Address6,
			Endpoint:            tempPeers[i].Endpoint,
			PublicKey:           tempPeers[i].PublicKey,
			PersistentKeepalive: tempPeers[i].KeepAlive,
			ListenPort:          tempPeers[i].ListenPort,
			LocalAddress:        tempPeers[i].LocalAddress,
		})
	}
	for _, extPeer := range extPeers {
		pubkey, err := wgtypes.ParseKey(extPeer.PublicKey)
		if err != nil {
			log.Println("error parsing key")
			return peers, err
		}

		if nodecfg.PublicKey == extPeer.PublicKey {
			continue
		}

		var peer wgtypes.PeerConfig
		var peeraddr = net.IPNet{
			IP:   net.ParseIP(extPeer.Address),
			Mask: net.CIDRMask(32, 32),
		}
		var allowedips []net.IPNet
		allowedips = append(allowedips, peeraddr)

		if extPeer.Address6 != "" && dualstack {
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
	return peers, err
}
