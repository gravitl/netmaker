package logic

import (
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// == Join, Checkin, and Leave for Server ==

// KUBERNETES_LISTEN_PORT - starting port for Kubernetes in order to use NodePort range
const KUBERNETES_LISTEN_PORT = 31821

// KUBERNETES_SERVER_MTU - ideal mtu for kubernetes deployments right now
const KUBERNETES_SERVER_MTU = 1024

// ServerJoin - responsible for joining a server to a network
func ServerJoin(networkSettings *models.Network, serverID string) error {

	if networkSettings == nil || networkSettings.NetID == "" {
		return errors.New("no network provided")
	}

	var err error
	var node = &models.Node{
		IsServer:     "yes",
		DNSOn:        "no",
		IsStatic:     "yes",
		Name:         models.NODE_SERVER_NAME,
		MacAddress:   serverID,
		UDPHolePunch: "no",
		IsLocal:      networkSettings.IsLocal,
		LocalRange:   networkSettings.LocalRange,
	}
	SetNodeDefaults(node)

	if servercfg.GetPlatform() == "Kubernetes" {
		node.ListenPort = KUBERNETES_LISTEN_PORT
		node.MTU = KUBERNETES_SERVER_MTU
	}

	if node.LocalRange != "" && node.LocalAddress == "" {
		logger.Log(1, "local vpn, getting local address from range:", node.LocalRange)
		node.LocalAddress = GetLocalIP(*node)
		var _, currentCIDR, cidrErr = net.ParseCIDR(node.LocalRange)
		if cidrErr != nil {
			return err
		}
		if !currentCIDR.Contains(net.IP(node.LocalAddress)) {
			node.LocalAddress = ""
		}
	}

	if node.Endpoint == "" {
		if node.IsLocal == "yes" && node.LocalAddress != "" {
			node.Endpoint = node.LocalAddress
		} else {
			node.Endpoint, err = ncutils.GetPublicIP()
		}
		if err != nil || node.Endpoint == "" {
			logger.Log(0, "Error setting server node Endpoint.")
			return err
		}
	}

	var privateKey = ""

	// Generate and set public/private WireGuard Keys
	if privateKey == "" {
		wgPrivatekey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			logger.Log(1, err.Error())
			return err
		}
		privateKey = wgPrivatekey.String()
		node.PublicKey = wgPrivatekey.PublicKey().String()
	}

	node.Network = networkSettings.NetID

	logger.Log(2, "adding a server instance on network", node.Network)
	err = CreateNode(node)
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
		logger.Log(2, "Error retrieving port:", err.Error())
	} else {
		logger.Log(1, "Set client port to", fmt.Sprintf("%d", node.ListenPort), "for network", node.Network)
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
	if err = ServerPush(node); err != nil {
		return err
	}

	peers, hasGateway, gateways, err := GetServerPeers(node)
	if err != nil && !ncutils.IsEmptyRecord(err) {
		logger.Log(1, "failed to retrieve peers")
		return err
	}

	err = initWireguard(node, privateKey, peers[:], hasGateway, gateways[:])
	if err != nil {
		return err
	}

	return nil
}

// ServerCheckin - runs pulls and pushes for server
func ServerCheckin(mac string, network string) error {
	var serverNode = &models.Node{}
	var currentNode, err = GetNode(mac, network)
	if err != nil {
		return err
	}
	serverNode = &currentNode

	err = ServerPull(serverNode, false)
	if isDeleteError(err) {
		return ServerLeave(mac, network)
	} else if err != nil {
		return err
	}

	actionCompleted := checkNodeActions(serverNode)
	if actionCompleted == models.NODE_DELETE {
		return errors.New("node has been removed")
	}

	return ServerPush(serverNode)
}

// ServerPull - pulls current config/peers for server
func ServerPull(serverNode *models.Node, onErr bool) error {

	var err error
	if serverNode.IPForwarding == "yes" {
		if err = setIPForwardingLinux(); err != nil {
			return err
		}
	}
	serverNode.OS = runtime.GOOS

	if serverNode.PullChanges == "yes" || onErr {
		// check for interface change
		// checks if address is in use by another interface
		var oldIfaceName, isIfacePresent = isInterfacePresent(serverNode.Interface, serverNode.Address)
		if !isIfacePresent {
			if err = deleteInterface(oldIfaceName, serverNode.PostDown); err != nil {
				logger.Log(1, "could not delete old interface", oldIfaceName)
			}
			logger.Log(1, "removed old interface", oldIfaceName)
		}
		serverNode.PullChanges = "no"
		if err = setWGConfig(serverNode, false); err != nil {
			return err
		}
		// handle server side update
		if err = UpdateNode(serverNode, serverNode); err != nil {
			return err
		}
	} else {
		if err = setWGConfig(serverNode, true); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return ServerPull(serverNode, true)
			} else {
				return err
			}
		}
	}

	return nil
}

// ServerPush - pushes config changes for server checkins/join
func ServerPush(serverNode *models.Node) error {
	serverNode.OS = runtime.GOOS
	serverNode.SetLastCheckIn()
	return UpdateNode(serverNode, serverNode)
}

// ServerLeave - removes a server node
func ServerLeave(mac string, network string) error {

	var serverNode, err = GetNode(mac, network)
	if err != nil {
		return err
	}
	serverNode.SetID()
	return DeleteNode(&serverNode, true)
}

/**
 * Below function needs major refactor
 *
 */

// GetServerPeers - gets peers of server
func GetServerPeers(serverNode *models.Node) ([]wgtypes.PeerConfig, bool, []string, error) {
	hasGateway := false
	var gateways []string
	var peers []wgtypes.PeerConfig
	var nodes []models.Node // fill above fields from server or client

	var nodecfg, err = GetNode(serverNode.MacAddress, serverNode.Network)
	if err != nil {
		return nil, hasGateway, gateways, err
	}
	nodes, err = GetPeers(&nodecfg)
	if err != nil {
		return nil, hasGateway, gateways, err
	}

	keepalive := nodecfg.PersistentKeepalive
	keepalivedur, err := time.ParseDuration(strconv.FormatInt(int64(keepalive), 10) + "s")
	if err != nil {
		logger.Log(1, "Issue with format of keepalive duration value, Please view server config:", err.Error())
		return nil, hasGateway, gateways, err
	}

	for _, node := range nodes {
		pubkey, err := wgtypes.ParseKey(node.PublicKey)
		if err != nil {
			logger.Log(1, "error parsing key", pubkey.String())
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
		var allowedips = []net.IPNet{
			peeraddr,
		}
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
					logger.Log(1, "could not parse gateway IP range. Not adding", iprange)
					continue // if can't parse CIDR
				}
				nodeEndpointArr := strings.Split(node.Endpoint, ":") // getting the public ip of node
				if ipnet.Contains(net.ParseIP(nodeEndpointArr[0])) { // ensuring egress gateway range does not contain public ip of node
					logger.Log(2, "egress IP range of", iprange, "overlaps with", node.Endpoint, ", omitting")
					continue // skip adding egress range if overlaps with node's ip
				}
				if ipnet.Contains(net.ParseIP(nodecfg.LocalAddress)) { // ensuring egress gateway range does not contain public ip of node
					logger.Log(2, "egress IP range of", iprange, "overlaps with", nodecfg.LocalAddress, ", omitting")
					continue // skip adding egress range if overlaps with node's local ip
				}
				gateways = append(gateways, iprange)
				if err != nil {
					logger.Log(1, "ERROR ENCOUNTERED SETTING GATEWAY:", err.Error())
				} else {
					allowedips = append(allowedips, *ipnet)
				}
			}
			ranges = nil
		}
		if node.Address6 != "" && serverNode.IsDualStack == "yes" {
			var addr6 = net.IPNet{
				IP:   net.ParseIP(node.Address6),
				Mask: net.CIDRMask(128, 128),
			}
			allowedips = append(allowedips, addr6)
		}
		peer = wgtypes.PeerConfig{
			PublicKey:                   pubkey,
			PersistentKeepaliveInterval: &(keepalivedur),
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  allowedips,
		}
		peers = append(peers, peer)
	}
	if serverNode.IsIngressGateway == "yes" {
		extPeers, err := GetServerExtPeers(serverNode)
		if err == nil {
			peers = append(peers, extPeers...)
		} else {
			logger.Log(1, "ERROR RETRIEVING EXTERNAL PEERS ON SERVER:", err.Error())
		}
		extPeers = nil
	}
	return peers, hasGateway, gateways, err
}

// GetServerExtPeers - gets the extpeers for a client
func GetServerExtPeers(serverNode *models.Node) ([]wgtypes.PeerConfig, error) {
	var peers []wgtypes.PeerConfig
	var extPeers []models.Node
	var err error
	var tempPeers []models.ExtPeersResponse

	tempPeers, err = GetExtPeersList(serverNode.MacAddress, serverNode.Network)
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

		if serverNode.PublicKey == extPeer.PublicKey {
			continue
		}

		var peer wgtypes.PeerConfig
		var peeraddr = net.IPNet{
			IP:   net.ParseIP(extPeer.Address),
			Mask: net.CIDRMask(32, 32),
		}
		var allowedips = []net.IPNet{
			peeraddr,
		}

		if extPeer.Address6 != "" && serverNode.IsDualStack == "yes" {
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
		allowedips = nil
	}
	tempPeers = nil
	extPeers = nil
	return peers, err
}

// == Private ==

func isDeleteError(err error) bool {
	return err != nil && strings.Contains(err.Error(), models.NODE_DELETE)
}

func checkNodeActions(node *models.Node) string {
	if (node.Action == models.NODE_UPDATE_KEY) &&
		node.IsStatic != "yes" {
		err := setWGKeyConfig(node)
		if err != nil {
			logger.Log(1, "unable to process reset keys request:", err.Error())
			return ""
		}
	}
	if node.Action == models.NODE_DELETE {
		err := ServerLeave(node.MacAddress, node.Network)
		if err != nil {
			logger.Log(1, "error deleting locally:", err.Error())
		}
		return models.NODE_DELETE
	}
	return ""
}
