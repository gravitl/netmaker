package server

import (
	"encoding/json"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// RELAY_KEEPALIVE_MARKER - sets the relay keepalive marker
const RELAY_KEEPALIVE_MARKER = "20007ms"

func getGrpcClient(cfg *config.ClientConfig) (nodepb.NodeServiceClient, error) {
	var wcclient nodepb.NodeServiceClient
	// == GRPC SETUP ==
	conn, err := grpc.Dial(cfg.Server.GRPCAddress,
		ncutils.GRPCRequestOpts(cfg.Server.GRPCSSL))

	if err != nil {
		return nil, err
	}
	defer conn.Close()
	wcclient = nodepb.NewNodeServiceClient(conn)
	return wcclient, nil
}

// CheckIn - checkin for node on a network
func CheckIn(network string) (*models.Node, error) {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return nil, err
	}
	node := cfg.Node
	if cfg.Node.IsServer != "yes" {
		wcclient, err := getGrpcClient(cfg)
		if err != nil {
			return nil, err
		}
		// == run client action ==
		var header metadata.MD
		ctx, err := auth.SetJWT(wcclient, network)
		if err != nil {
			return nil, err
		}
		nodeData, err := json.Marshal(&node)
		if err != nil {
			return nil, err
		}
		response, err := wcclient.ReadNode(
			ctx,
			&nodepb.Object{
				Data: string(nodeData),
				Type: nodepb.NODE_TYPE,
			},
			grpc.Header(&header),
		)
		if err != nil {
			log.Printf("Encountered error checking in node: %v", err)
		}
		if err = json.Unmarshal([]byte(response.GetData()), &node); err != nil {
			return nil, err
		}
	}
	return &node, err
}

// GetPeers - gets the peers for a node
func GetPeers(macaddress string, network string, server string, dualstack bool, isIngressGateway bool, isServer bool) ([]wgtypes.PeerConfig, bool, []string, error) {
	hasGateway := false
	var err error
	var gateways []string
	var peers []wgtypes.PeerConfig
	var nodecfg models.Node
	var nodes []models.Node // fill above fields from server or client

	if !isServer { // set peers client side
		cfg, err := config.ReadConfig(network)
		if err != nil {
			log.Fatalf("Issue retrieving config for network: "+network+". Please investigate: %v", err)
		}
		nodecfg = cfg.Node
		var wcclient nodepb.NodeServiceClient
		conn, err := grpc.Dial(cfg.Server.GRPCAddress,
			ncutils.GRPCRequestOpts(cfg.Server.GRPCSSL))

		if err != nil {
			log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
		}
		defer conn.Close()
		// Instantiate the BlogServiceClient with our client connection to the server
		wcclient = nodepb.NewNodeServiceClient(conn)

		nodeData, err := json.Marshal(&nodecfg)
		if err != nil {
			ncutils.PrintLog("could not parse node data from config during peer fetch for network "+network, 1)
			return peers, hasGateway, gateways, err
		}

		req := &nodepb.Object{
			Data: string(nodeData),
			Type: nodepb.NODE_TYPE,
		}

		ctx, err := auth.SetJWT(wcclient, network)
		if err != nil {
			log.Println("Failed to authenticate.")
			return peers, hasGateway, gateways, err
		}
		var header metadata.MD

		response, err := wcclient.GetPeers(ctx, req, grpc.Header(&header))
		if err != nil {
			log.Println("Error retrieving peers")
			log.Println(err)
			return nil, hasGateway, gateways, err
		}
		if err := json.Unmarshal([]byte(response.GetData()), &nodes); err != nil {
			log.Println("Error unmarshaling data for peers")
			return nil, hasGateway, gateways, err
		}
	}

	keepalive := nodecfg.PersistentKeepalive
	keepalivedur, _ := time.ParseDuration(strconv.FormatInt(int64(keepalive), 10) + "s")
	keepaliveserver, err := time.ParseDuration(strconv.FormatInt(int64(5), 10) + "s")
	if err != nil {
		log.Fatalf("Issue with format of keepalive value. Please update netconfig: %v", err)
	}

	for _, node := range nodes {
		pubkey, err := wgtypes.ParseKey(node.PublicKey)
		if err != nil {
			log.Println("error parsing key")
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
		extPeers, err := GetExtPeers(macaddress, network, server, dualstack)
		if err == nil {
			peers = append(peers, extPeers...)
		} else {
			log.Println("ERROR RETRIEVING EXTERNAL PEERS", err)
		}
	}
	return peers, hasGateway, gateways, err
}

// GetExtPeers - gets the extpeers for a client
func GetExtPeers(macaddress string, network string, server string, dualstack bool) ([]wgtypes.PeerConfig, error) {
	var peers []wgtypes.PeerConfig
	var nodecfg models.Node
	var extPeers []models.Node
	var err error
	// fill above fields from either client or server

	if nodecfg.IsServer != "yes" { // fill extPeers with client side logic
		var cfg *config.ClientConfig
		cfg, err = config.ReadConfig(network)
		if err != nil {
			log.Fatalf("Issue retrieving config for network: "+network+". Please investigate: %v", err)
		}
		nodecfg = cfg.Node
		var wcclient nodepb.NodeServiceClient

		conn, err := grpc.Dial(cfg.Server.GRPCAddress,
			ncutils.GRPCRequestOpts(cfg.Server.GRPCSSL))
		if err != nil {
			log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
		}
		defer conn.Close()
		// Instantiate the BlogServiceClient with our client connection to the server
		wcclient = nodepb.NewNodeServiceClient(conn)

		nodeData, err := json.Marshal(&nodecfg)
		if err != nil {
			ncutils.PrintLog("could not parse node data from config during peer fetch for network "+network, 1)
			return peers, err
		}

		req := &nodepb.Object{
			Data: string(nodeData),
			Type: nodepb.NODE_TYPE,
		}

		ctx, err := auth.SetJWT(wcclient, network)
		if err != nil {
			log.Println("Failed to authenticate.")
			return peers, err
		}
		var header metadata.MD

		responseObject, err := wcclient.GetExtPeers(ctx, req, grpc.Header(&header))
		if err != nil {
			log.Println("Error retrieving peers")
			log.Println(err)
			return nil, err
		}
		if err = json.Unmarshal([]byte(responseObject.Data), &extPeers); err != nil {
			return nil, err
		}
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
