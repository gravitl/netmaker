package logic

import (
	"errors"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// == Join, Checkin, and Leave for Server ==

// ServerJoin - responsible for joining a server to a network
func ServerJoin(network string, serverID string, privateKey string) error {

	if network == "" {
		return errors.New("no network provided")
	}

	var err error
	var node *models.Node // fill this object with server node specifics
	node = &models.Node{
		IsServer:     "yes",
		DNSOn:        "no",
		IsStatic:     "yes",
		Name:         models.NODE_SERVER_NAME,
		MacAddress:   serverID,
		UDPHolePunch: "no",
	}
	node.SetDefaults()

	if node.LocalRange != "" && node.LocalAddress == "" {
		Log("local vpn, getting local address from range: "+node.LocalRange, 1)
		node.LocalAddress = GetLocalIP(*node)
	}

	if node.Endpoint == "" {
		if node.IsLocal == "yes" && node.LocalAddress != "" {
			node.Endpoint = node.LocalAddress
		} else {
			node.Endpoint, err = ncutils.GetPublicIP()
		}
		if err != nil || node.Endpoint == "" {
			Log("Error setting server node Endpoint.", 0)
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
		node.PublicKey = wgPrivatekey.PublicKey().String()
	}
	// should never set mac address for server anymore

	var postnode *models.Node
	postnode = &models.Node{
		Password:            node.Password,
		MacAddress:          node.MacAddress,
		AccessKey:           node.AccessKey,
		Network:             network,
		ListenPort:          node.ListenPort,
		PostUp:              node.PostUp,
		PostDown:            node.PostDown,
		PersistentKeepalive: node.PersistentKeepalive,
		LocalAddress:        node.LocalAddress,
		Interface:           node.Interface,
		PublicKey:           node.PublicKey,
		DNSOn:               node.DNSOn,
		Name:                node.Name,
		Endpoint:            node.Endpoint,
		SaveConfig:          node.SaveConfig,
		UDPHolePunch:        node.UDPHolePunch,
	}

	Log("adding a server instance on network "+postnode.Network, 2)
	*node, err = CreateNode(*postnode, network)
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
	if node.IsLocal == "yes" && node.LocalRange != "" {
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

	peers, hasGateway, gateways, err := GetServerPeers(node.MacAddress, network, node.IsDualStack == "yes", node.IsIngressGateway == "yes")
	if err != nil && !ncutils.IsEmptyRecord(err) {
		Log("failed to retrieve peers", 1)
		return err
	}

	err = initWireguard(node, privateKey, peers, hasGateway, gateways)
	if err != nil {
		return err
	}

	return nil
}

// ServerCheckin - runs pulls and pushes for server
func ServerCheckin(mac string, network string) error {
	var serverNode models.Node
	var newNode *models.Node
	var err error
	serverNode, err = GetNode(mac, network)
	if err != nil {
		return err
	}

	newNode, err = ServerPull(mac, network, false)
	if isDeleteError(err) {
		return ServerLeave(mac, network)
	} else if err != nil {
		return err
	}

	actionCompleted := checkNodeActions(newNode, network, &serverNode)
	if actionCompleted == models.NODE_DELETE {
		return errors.New("node has been removed")
	}

	return ServerPush(newNode.MacAddress, newNode.Network)
}

// ServerPull - pulls current config/peers for server
func ServerPull(mac string, network string, onErr bool) (*models.Node, error) {

	var serverNode models.Node
	var err error
	serverNode, err = GetNode(mac, network)
	if err != nil {
		return &serverNode, err
	}

	if serverNode.IPForwarding == "yes" {
		if err = setIPForwardingLinux(); err != nil {
			return &serverNode, err
		}
	}
	serverNode.OS = runtime.GOOS

	if serverNode.PullChanges == "yes" || onErr {
		// check for interface change
		var isIfacePresent bool
		var oldIfaceName string
		// checks if address is in use by another interface
		oldIfaceName, isIfacePresent = isInterfacePresent(serverNode.Interface, serverNode.Address)
		if !isIfacePresent {
			if err = deleteInterface(oldIfaceName, serverNode.PostDown); err != nil {
				Log("could not delete old interface "+oldIfaceName, 1)
			}
			Log("removed old interface "+oldIfaceName, 1)
		}
		serverNode.PullChanges = "no"
		if err = setWGConfig(serverNode, network, false); err != nil {
			return &serverNode, err
		}
		// handle server side update
		if err = serverNode.Update(&serverNode); err != nil {
			return &serverNode, err
		}
	} else {
		if err = setWGConfig(serverNode, network, true); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return ServerPull(serverNode.MacAddress, serverNode.Network, true)
			} else {
				return &serverNode, err
			}
		}
	}

	return &serverNode, nil
}

// ServerPush - pushes config changes for server checkins/join
func ServerPush(mac string, network string) error {

	var serverNode models.Node
	var err error
	serverNode, err = GetNode(mac, network)
	if err != nil /* && !ncutils.IsEmptyRecord(err) May not be necessary */ {
		return err
	}
	serverNode.OS = runtime.GOOS
	serverNode.SetLastCheckIn()
	return serverNode.Update(&serverNode)
}

// ServerLeave - removes a server node
func ServerLeave(mac string, network string) error {

	var serverNode models.Node
	var err error
	serverNode, err = GetNode(mac, network)
	if err != nil {
		return err
	}
	serverNode.SetID()
	return DeleteNode(serverNode.ID, true)
}

// GetServerPeers - gets peers of server
func GetServerPeers(macaddress string, network string, dualstack bool, isIngressGateway bool) ([]wgtypes.PeerConfig, bool, []string, error) {
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
		Log("Issue with format of keepalive value. Please view server config. "+err.Error(), 1)
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
					Log("could not parse gateway IP range. Not adding "+iprange, 1)
					continue // if can't parse CIDR
				}
				nodeEndpointArr := strings.Split(node.Endpoint, ":") // getting the public ip of node
				if ipnet.Contains(net.ParseIP(nodeEndpointArr[0])) { // ensuring egress gateway range does not contain public ip of node
					Log("egress IP range of "+iprange+" overlaps with "+node.Endpoint+", omitting", 2)
					continue // skip adding egress range if overlaps with node's ip
				}
				if ipnet.Contains(net.ParseIP(nodecfg.LocalAddress)) { // ensuring egress gateway range does not contain public ip of node
					Log("egress IP range of "+iprange+" overlaps with "+nodecfg.LocalAddress+", omitting", 2)
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
		extPeers, err := GetServerExtPeers(macaddress, network, dualstack)
		if err == nil {
			peers = append(peers, extPeers...)
		} else {
			Log("ERROR RETRIEVING EXTERNAL PEERS ON SERVER", 1)
		}
	}
	return peers, hasGateway, gateways, err
}

// GetServerExtPeers - gets the extpeers for a client
func GetServerExtPeers(macaddress string, network string, dualstack bool) ([]wgtypes.PeerConfig, error) {
	var peers []wgtypes.PeerConfig
	var nodecfg models.Node
	var extPeers []models.Node
	var err error
	// fill above fields from either client or server

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

// == Private ==

func isDeleteError(err error) bool {
	return err != nil && strings.Contains(err.Error(), models.NODE_DELETE)
}

func checkNodeActions(node *models.Node, networkName string, localNode *models.Node) string {
	if (node.Action == models.NODE_UPDATE_KEY || localNode.Action == models.NODE_UPDATE_KEY) &&
		node.IsStatic != "yes" {
		err := setWGKeyConfig(*node)
		if err != nil {
			Log("unable to process reset keys request: "+err.Error(), 1)
			return ""
		}
	}
	if node.Action == models.NODE_DELETE || localNode.Action == models.NODE_DELETE {
		err := ServerLeave(node.MacAddress, networkName)
		if err != nil {
			Log("error deleting locally: "+err.Error(), 1)
		}
		return models.NODE_DELETE
	}
	return ""
}
