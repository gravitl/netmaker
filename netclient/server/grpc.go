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
	"github.com/gravitl/netmaker/netclient/netclientutils"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const RELAY_KEEPALIVE_MARKER = "20007ms"

func getGrpcClient(cfg *config.ClientConfig) (nodepb.NodeServiceClient, error) {
	var wcclient nodepb.NodeServiceClient
	// == GRPC SETUP ==
	conn, err := grpc.Dial(cfg.Server.GRPCAddress, 
		netclientutils.GRPCRequestOpts(cfg.Server.GRPCSSL))

	if err != nil {
		return nil, err
	}
	wcclient = nodepb.NewNodeServiceClient(conn)
	return wcclient, nil
}

func CheckIn(network string) (*models.Node, error) {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return nil, err
	}
	node := cfg.Node
	wcclient, err := getGrpcClient(cfg)
	if err != nil {
		return nil, err
	}
	// == run client action ==
	var header metadata.MD
	ctx, err := auth.SetJWT(wcclient, network)
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
	return &node, err
}

func RemoveNetwork(network string) error {
	//need to  implement checkin on server side
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	servercfg := cfg.Server
	node := cfg.Node
	log.Println("Deleting remote node with MAC: " + node.MacAddress)

	var wcclient nodepb.NodeServiceClient
	conn, err := grpc.Dial(cfg.Server.GRPCAddress, 
		netclientutils.GRPCRequestOpts(cfg.Server.GRPCSSL))
	if err != nil {
		log.Printf("Unable to establish client connection to "+servercfg.GRPCAddress+": %v", err)
		//return err
	} else {
		wcclient = nodepb.NewNodeServiceClient(conn)
		ctx, err := auth.SetJWT(wcclient, network)
		if err != nil {
			//return err
			log.Printf("Failed to authenticate: %v", err)
		} else {

			var header metadata.MD

			_, err = wcclient.DeleteNode(
				ctx,
				&nodepb.Object{
					Data: node.MacAddress + "###" + node.Network,
					Type: nodepb.STRING_TYPE,
				},
				grpc.Header(&header),
			)
			if err != nil {
				log.Printf("Encountered error deleting node: %v", err)
				log.Println(err)
			} else {
				log.Println("Deleted node " + node.MacAddress)
			}
		}
	}
	err = local.WipeLocal(network)
	if err != nil {
		log.Printf("Unable to wipe local config: %v", err)
	}
	if cfg.Daemon != "off" {
		err = local.RemoveSystemDServices(network)
	}
	return err
}

func GetPeers(macaddress string, network string, server string, dualstack bool, isIngressGateway bool) ([]wgtypes.PeerConfig, bool, []string, error) {
	hasGateway := false
	var gateways []string
	var peers []wgtypes.PeerConfig
	var wcclient nodepb.NodeServiceClient
	cfg, err := config.ReadConfig(network)
	if err != nil {
		log.Fatalf("Issue retrieving config for network: "+network+". Please investigate: %v", err)
	}
	nodecfg := cfg.Node
	keepalive := nodecfg.PersistentKeepalive
	keepalivedur, err := time.ParseDuration(strconv.FormatInt(int64(keepalive), 10) + "s")
	keepaliveserver, err := time.ParseDuration(strconv.FormatInt(int64(5), 10) + "s")
	if err != nil {
		log.Fatalf("Issue with format of keepalive value. Please update netconfig: %v", err)
	}

	conn, err := grpc.Dial(cfg.Server.GRPCAddress, 
		netclientutils.GRPCRequestOpts(cfg.Server.GRPCSSL))

	if err != nil {
		log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
	}
	// Instantiate the BlogServiceClient with our client connection to the server
	wcclient = nodepb.NewNodeServiceClient(conn)

	req := &nodepb.Object{
		Data: macaddress + "###" + network,
		Type: nodepb.STRING_TYPE,
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
	var nodes []models.Node
	if err := json.Unmarshal([]byte(response.GetData()), &nodes); err != nil {
		log.Println("Error unmarshaling data for peers")
		return nil, hasGateway, gateways, err
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
				if !ipnet.Contains(net.IP(nodeEndpointArr[0])) { // don't need to add an allowed ip that already exists..
					allowedips = append(allowedips, *ipnet)
				}
			}
		}
		// handle egress gateway peers
		if node.IsEgressGateway == "yes" {
			hasGateway = true
			ranges := node.EgressGatewayRanges
			for _, iprange := range ranges { // go through each cidr for egress gateway
				_, ipnet, err := net.ParseCIDR(iprange) // confirming it's valid cidr
				if err != nil {
					continue // if can't parse CIDR
				}
				nodeEndpointArr := strings.Split(node.Endpoint, ":") // getting the public ip of node
				if ipnet.Contains(net.IP(nodeEndpointArr[0])) {      // ensuring egress gateway range does not contain public ip of node
					continue // skip adding egress range if overlaps with nodes ip
				}
				gateways = append(gateways, iprange)
				if err != nil {
					log.Println("ERROR ENCOUNTERED SETTING GATEWAY")
				} else {
					allowedips = append(allowedips, *ipnet)
				}
			}
		}
		// handle relay servers
		/*
		if node.IsRelay == "yes" {
			hasRelay = true
			relayAddrs = node.RelayAddrs
			log.Println("found relay addresses")
			log.Println(node.RelayAddrs)
			for _, ipstring := range node.RelayAddrs { // go through each ip for relay server
				log.Println("checking addr")
				log.Println(ipstring)
				_, ip, err := net.ParseCIDR(ipstring) // confirming it's a valid IP
				if ip == nil || err != nil {
					if appendip := net.ParseIP(ipstring); appendip != nil {
						ipnet := net.IPNet{
							IP:   net.ParseIP(ipstring),
							Mask: net.CIDRMask(32, 32),
						}
						allowedips = append(allowedips, ipnet)
					} else {
						log.Println(err)
						continue // if can't parse CIDR
					}
				} else {
					allowedips = append(allowedips, *ip)
				}
			}
		}
		*/
		if node.Address6 != "" && dualstack {
			var addr6 = net.IPNet{
				IP:   net.ParseIP(node.Address6),
				Mask: net.CIDRMask(128, 128),
			}
			allowedips = append(allowedips, addr6)
		}
		if nodecfg.IsServer == "yes" {
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
			log.Println("ERROR RETRIEVING EXTERNAL PEERS",err)
		}
	}
	/*
	if hasRelay {
		peers = RemoveRelayAddrsFromPeers(relayAddrs, peers)
	}
	*/
	return peers, hasGateway, gateways, err
}
/*
func RemoveRelayAddrsFromPeers(relayAddrs []string, peers []wgtypes.PeerConfig)([]wgtypes.PeerConfig){
	relayMarker, err := time.ParseDuration(RELAY_KEEPALIVE_MARKER)
	if err != nil {
		log.Println(err)
		log.Println("Could not remove relayed peers. Relay will not be used")
		return peers
	}
	for _, ipstring := range relayAddrs { // go through each ip for relay server
		_, ip, err := net.ParseCIDR(ipstring) // confirming it's a valid IP
		if ip == nil || err != nil {
			continue // if can't parse CIDR
		}
		for i, peer := range peers {
			if *peer.PersistentKeepaliveInterval == relayMarker {
				continue
			}
			for _, nodeip := range peer.AllowedIPs {
				if ip.Contains(nodeip.IP) {
					peers = append(peers[:i], peers[i+1:]...)
				}
			}
		}
	}
	return peers
}
*/
func GetExtPeers(macaddress string, network string, server string, dualstack bool) ([]wgtypes.PeerConfig, error) {
	var peers []wgtypes.PeerConfig
	var wcclient nodepb.NodeServiceClient
	cfg, err := config.ReadConfig(network)
	if err != nil {
		log.Fatalf("Issue retrieving config for network: "+network+". Please investigate: %v", err)
	}
	nodecfg := cfg.Node

	conn, err := grpc.Dial(cfg.Server.GRPCAddress, 
		netclientutils.GRPCRequestOpts(cfg.Server.GRPCSSL))	
	if err != nil {
		log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
	}
	// Instantiate the BlogServiceClient with our client connection to the server
	wcclient = nodepb.NewNodeServiceClient(conn)

	req := &nodepb.Object{
		Data: macaddress + "###" + network,
		Type: nodepb.STRING_TYPE,
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
	var extPeers []models.Node
	if err = json.Unmarshal([]byte(responseObject.Data), &extPeers); err != nil {
		return nil, err
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
