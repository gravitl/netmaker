package server

import (
	"fmt"
	"context"
	"log"
	"strings"
	"strconv"
	"net"
	"time"
	"io"
        "golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"github.com/gravitl/netmaker/netclient/config"
        "github.com/gravitl/netmaker/netclient/auth"
        "github.com/gravitl/netmaker/netclient/local"
        nodepb "github.com/gravitl/netmaker/grpc"
        "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	//homedir "github.com/mitchellh/go-homedir"
)

func GetNode(network string) nodepb.Node {

        modcfg, err := config.ReadConfig(network)
        if err != nil {
                log.Fatalf("Error: %v", err)
        }

	nodecfg := modcfg.Node
	var node nodepb.Node

	node.Name = nodecfg.Name
	node.Interface = nodecfg.Interface
	node.Nodenetwork = nodecfg.Network
	node.Localaddress = nodecfg.LocalAddress
	node.Address = nodecfg.WGAddress
	node.Address6 = nodecfg.WGAddress6
	node.Listenport = nodecfg.Port
	node.Keepalive = nodecfg.KeepAlive
	node.Postup = nodecfg.PostUp
	node.Postdown = nodecfg.PostDown
	node.Publickey = nodecfg.PublicKey
	node.Macaddress = nodecfg.MacAddress
	node.Endpoint = nodecfg.Endpoint
	node.Password = nodecfg.Password
	if nodecfg.DNS == "on" {
		node.Dnsoff = false
	} else {
		node.Dnsoff = true
	}
        if nodecfg.IsDualStack == "yes" {
                node.Isdualstack = true
        } else {
                node.Isdualstack = false
        }
        if nodecfg.IsIngressGateway == "yes" {
                node.Isingressgateway= true
        } else {
                node.Isingressgateway = false
        }
        return node
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
        conn, err := grpc.Dial(servercfg.GRPCAddress, requestOpts)
	if err != nil {
                log.Printf("Unable to establish client connection to " + servercfg.GRPCAddress + ": %v", err)
		//return err
        }else {
        wcclient = nodepb.NewNodeServiceClient(conn)

        ctx := context.Background()
        fmt.Println("Authenticating with GRPC Server")
        ctx, err = auth.SetJWT(wcclient, network)
        if err != nil {
                //return err
                log.Printf("Failed to authenticate: %v", err)
        } else {
        fmt.Println("Authenticated")

        var header metadata.MD

        _, err = wcclient.DeleteNode(
                ctx,
                &nodepb.DeleteNodeReq{
                        Macaddress: node.MacAddress,
                        NetworkName: node.Network,
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
	err =  local.RemoveSystemDServices(network)
        if err != nil {
                return err
                log.Printf("Unable to remove systemd services: %v", err)
        }
	fmt.Printf("Please investigate any stated errors to ensure proper removal.")
	fmt.Printf("Failure to delete node from server via gRPC will mean node still exists and needs to be manually deleted by administrator.")

	return nil
}

func GetPeers(macaddress string, network string, server string, dualstack bool, isIngressGateway bool) ([]wgtypes.PeerConfig, bool, []string, error) {
        //need to  implement checkin on server side
        hasGateway := false
        var gateways []string
        var peers []wgtypes.PeerConfig
        var wcclient nodepb.NodeServiceClient
        cfg, err := config.ReadConfig(network)
        if err != nil {
                log.Fatalf("Issue retrieving config for network: " + network +  ". Please investigate: %v", err)
        }
        nodecfg := cfg.Node
        keepalive := nodecfg.KeepAlive
        keepalivedur, err := time.ParseDuration(strconv.FormatInt(int64(keepalive), 10) + "s")
        if err != nil {
                log.Fatalf("Issue with format of keepalive value. Please update netconfig: %v", err)
        }


        requestOpts := grpc.WithInsecure()
        conn, err := grpc.Dial(server, requestOpts)
        if err != nil {
                log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
        }
        // Instantiate the BlogServiceClient with our client connection to the server
        wcclient = nodepb.NewNodeServiceClient(conn)

        req := &nodepb.GetPeersReq{
                Macaddress: macaddress,
                Network: network,
        }
        ctx := context.Background()
        ctx, err = auth.SetJWT(wcclient, network)
        if err != nil {
                fmt.Println("Failed to authenticate.")
                return peers, hasGateway, gateways, err
        }
        var header metadata.MD

        stream, err := wcclient.GetPeers(ctx, req, grpc.Header(&header))
        if err != nil {
                fmt.Println("Error retrieving peers")
                fmt.Println(err)
                return nil, hasGateway, gateways, err
        }
        for {
                res, err := stream.Recv()
                // If end of stream, break the loop

                if err == io.EOF {
                        break
                }
                // if err, return an error
                if err != nil {
                        if strings.Contains(err.Error(), "mongo: no documents in result") {
                                continue
                        } else {
                        fmt.Println("ERROR ENCOUNTERED WITH RESPONSE")
                        fmt.Println(res)
                        return peers, hasGateway, gateways, err
                        }
                }
                pubkey, err := wgtypes.ParseKey(res.Peers.Publickey)
                if err != nil {
                        fmt.Println("error parsing key")
                        return peers, hasGateway, gateways, err
                }

                if nodecfg.PublicKey == res.Peers.Publickey {
                        continue
                }
                if nodecfg.Endpoint == res.Peers.Endpoint {
                        continue
                }

                var peer wgtypes.PeerConfig
                var peeraddr = net.IPNet{
                        IP: net.ParseIP(res.Peers.Address),
                        Mask: net.CIDRMask(32, 32),
                }
                var allowedips []net.IPNet
                allowedips = append(allowedips, peeraddr)
                if res.Peers.Isegressgateway {
                        hasGateway = true
                        gateways = append(gateways,res.Peers.Egressgatewayrange)
                        _, ipnet, err := net.ParseCIDR(res.Peers.Egressgatewayrange)
                        if err != nil {
                                fmt.Println("ERROR ENCOUNTERED SETTING GATEWAY")
                                fmt.Println("NOT SETTING GATEWAY")
                                fmt.Println(err)
                        } else {
                                fmt.Println("    Gateway Range: "  + res.Peers.Egressgatewayrange)
                                allowedips = append(allowedips, *ipnet)
                        }
                }
                if res.Peers.Address6 != "" && dualstack {
                        var addr6 = net.IPNet{
                                IP: net.ParseIP(res.Peers.Address6),
                                Mask: net.CIDRMask(128, 128),
                        }
                        allowedips = append(allowedips, addr6)
                }
                if keepalive != 0 {
                peer = wgtypes.PeerConfig{
                        PublicKey: pubkey,
                        PersistentKeepaliveInterval: &keepalivedur,
                        Endpoint: &net.UDPAddr{
                                IP:   net.ParseIP(res.Peers.Endpoint),
                                Port: int(res.Peers.Listenport),
                        },
                        ReplaceAllowedIPs: true,
                        AllowedIPs: allowedips,
                        }
                } else {
                peer = wgtypes.PeerConfig{
                        PublicKey: pubkey,
                        Endpoint: &net.UDPAddr{
                                IP:   net.ParseIP(res.Peers.Endpoint),
                                Port: int(res.Peers.Listenport),
                        },
                        ReplaceAllowedIPs: true,
                        AllowedIPs: allowedips,
                        }
                }
                peers = append(peers, peer)

        }
        if isIngressGateway {
                extPeers, err := GetExtPeers(macaddress, network, server, dualstack)
                if err == nil {
                        peers = append(peers, extPeers...)
                        fmt.Println("Added " + strconv.Itoa(len(extPeers)) + " external clients.")
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
                log.Fatalf("Issue retrieving config for network: " + network +  ". Please investigate: %v", err)
        }
        nodecfg := cfg.Node

        fmt.Println("Registering with GRPC Server")
        requestOpts := grpc.WithInsecure()
        conn, err := grpc.Dial(server, requestOpts)
        if err != nil {
                log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
        }
        // Instantiate the BlogServiceClient with our client connection to the server
        wcclient = nodepb.NewNodeServiceClient(conn)

        req := &nodepb.GetExtPeersReq{
                Macaddress: macaddress,
                Network: network,
        }
        ctx := context.Background()
        ctx, err = auth.SetJWT(wcclient, network)
        if err != nil {
                fmt.Println("Failed to authenticate.")
                return peers, err
        }
        var header metadata.MD

        stream, err := wcclient.GetExtPeers(ctx, req, grpc.Header(&header))
        if err != nil {
                fmt.Println("Error retrieving peers")
                fmt.Println(err)
                return nil, err
        }
        for {
                res, err := stream.Recv()
                // If end of stream, break the loop

                if err == io.EOF {
                        break
                }
                // if err, return an error
                if err != nil {
                        if strings.Contains(err.Error(), "mongo: no documents in result") {
                                continue
                        } else {
                        fmt.Println("ERROR ENCOUNTERED WITH RESPONSE")
                        fmt.Println(res)
                        return peers, err
                        }
                }
                pubkey, err := wgtypes.ParseKey(res.Extpeers.Publickey)
                if err != nil {
                        fmt.Println("error parsing key")
                        return peers, err
                }

                if nodecfg.PublicKey == res.Extpeers.Publickey {
                        continue
                }

                var peer wgtypes.PeerConfig
                var peeraddr = net.IPNet{
                        IP: net.ParseIP(res.Extpeers.Address),
                        Mask: net.CIDRMask(32, 32),
                }
                var allowedips []net.IPNet
                allowedips = append(allowedips, peeraddr)

		if res.Extpeers.Address6 != "" && dualstack {
                        var addr6 = net.IPNet{
                                IP: net.ParseIP(res.Extpeers.Address6),
                                Mask: net.CIDRMask(128, 128),
                        }
                        allowedips = append(allowedips, addr6)
                }
                peer = wgtypes.PeerConfig{
                        PublicKey: pubkey,
                        ReplaceAllowedIPs: true,
                        AllowedIPs: allowedips,
                        }
                peers = append(peers, peer)

        }
        return peers, err
}
