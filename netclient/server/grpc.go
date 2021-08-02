package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	//homedir "github.com/mitchellh/go-homedir"
)

func getGrpcClient(cfg *config.ClientConfig) (nodepb.NodeServiceClient, error) {
	servercfg := cfg.Server
	var wcclient nodepb.NodeServiceClient
	// == GRPC SETUP ==
	var requestOpts grpc.DialOption
	requestOpts = grpc.WithInsecure()
	if cfg.Server.GRPCSSL == "on" {
		h2creds := credentials.NewTLS(&tls.Config{NextProtos: []string{"h2"}})
		requestOpts = grpc.WithTransportCredentials(h2creds)
	}
	conn, err := grpc.Dial(servercfg.GRPCAddress, requestOpts)
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
	fmt.Println("Deleting remote node with MAC: " + node.MacAddress)

	var wcclient nodepb.NodeServiceClient
	var requestOpts grpc.DialOption
	requestOpts = grpc.WithInsecure()
	if cfg.Server.GRPCSSL == "on" {
		h2creds := credentials.NewTLS(&tls.Config{NextProtos: []string{"h2"}})
		requestOpts = grpc.WithTransportCredentials(h2creds)
	}
	conn, err := grpc.Dial(servercfg.GRPCAddress, requestOpts)
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
				fmt.Println(err)
			} else {
				fmt.Println("Deleted node " + node.MacAddress)
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
	//need to  implement checkin on server side
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

	requestOpts := grpc.WithInsecure()
	if cfg.Server.GRPCSSL == "on" {
		h2creds := credentials.NewTLS(&tls.Config{NextProtos: []string{"h2"}})
		requestOpts = grpc.WithTransportCredentials(h2creds)
	}

	conn, err := grpc.Dial(server, requestOpts)
	if err != nil {
		log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
	}
	// Instantiate the BlogServiceClient with our client connection to the server
	wcclient = nodepb.NewNodeServiceClient(conn)

	req := &nodepb.Object{
		Data: macaddress + "###" + network,
		Type: nodepb.STRING_TYPE,
	}
	ctx := context.Background()
	ctx, err = auth.SetJWT(wcclient, network)
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
			fmt.Println("error parsing key")
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
		if node.IsEgressGateway == "yes" {
			hasGateway = true
			ranges := node.EgressGatewayRanges
			for _, iprange := range ranges {
				gateways = append(gateways, iprange)
				_, ipnet, err := net.ParseCIDR(iprange)
				if err != nil {
					fmt.Println("ERROR ENCOUNTERED SETTING GATEWAY")
					fmt.Println("NOT SETTING GATEWAY")
					fmt.Println(err)
				} else {
					fmt.Println("    Gateway Range: " + iprange)
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
		if nodecfg.Name == "netmaker" {
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
			fmt.Println("ERROR RETRIEVING EXTERNAL PEERS")
			fmt.Println(err)
		}
	}
	return peers, hasGateway, gateways, err
}

func GetExtPeers(macaddress string, network string, server string, dualstack bool) ([]wgtypes.PeerConfig, error) {
	var peers []wgtypes.PeerConfig
	var wcclient nodepb.NodeServiceClient
	cfg, err := config.ReadConfig(network)
	if err != nil {
		log.Fatalf("Issue retrieving config for network: "+network+". Please investigate: %v", err)
	}
	nodecfg := cfg.Node

	requestOpts := grpc.WithInsecure()
	conn, err := grpc.Dial(server, requestOpts)
	if err != nil {
		log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
	}
	// Instantiate the BlogServiceClient with our client connection to the server
	wcclient = nodepb.NewNodeServiceClient(conn)

	req := &nodepb.Object{
		Data: macaddress + "###" + network,
		Type: nodepb.STRING_TYPE,
	}
	ctx := context.Background()
	ctx, err = auth.SetJWT(wcclient, network)
	if err != nil {
		fmt.Println("Failed to authenticate.")
		return peers, err
	}
	var header metadata.MD

	responseObject, err := wcclient.GetExtPeers(ctx, req, grpc.Header(&header))
	if err != nil {
		fmt.Println("Error retrieving peers")
		fmt.Println(err)
		return nil, err
	}
	var extPeers []models.Node
	if err = json.Unmarshal([]byte(responseObject.Data), extPeers); err != nil {
		return nil, err
	}
	for _, extPeer := range extPeers {
		pubkey, err := wgtypes.ParseKey(extPeer.PublicKey)
		if err != nil {
			fmt.Println("error parsing key")
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
