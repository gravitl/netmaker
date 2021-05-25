package serverctl

import (
	"os"
	"log"
	"context"
	"time"
	"net"
	"strconv"
	"errors"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
        "github.com/gravitl/netmaker/servercfg"
        "github.com/gravitl/netmaker/functions"
        "github.com/gravitl/netmaker/models"
        "github.com/gravitl/netmaker/mongoconn"
)

func InitServerWireGuard() error {

	created, err := CreateCommsNetwork()
	if !created {
		return err
	}
	wgconfig := servercfg.GetWGConfig()
	if !(wgconfig.GRPCWireGuard == "on") {
		return errors.New("WireGuard not enabled on this server.")
	}
	ifaceSettings := netlink.NewLinkAttrs()

	if wgconfig.GRPCWGInterface == "" {
		return errors.New("No WireGuard Interface Name set.")
	}
	ifaceSettings.Name = wgconfig.GRPCWGInterface
	wglink := &models.WireGuardLink{LinkAttrs: &ifaceSettings}

	err = netlink.LinkAdd(wglink)
	if err != nil {
		if os.IsExist(err) {
			log.Println("interface " + ifaceSettings.Name + " already exists")
			log.Println("continuing setup using existing interface")
		} else {
			return err
		}
	}
	address, err := netlink.ParseAddr(wgconfig.GRPCWGAddress + "/32")
	if err != nil {
		return err
	}

	err = netlink.AddrAdd(wglink, address)
        if err != nil {
                if os.IsExist(err) {
                        log.Println("address " + wgconfig.GRPCWGAddress + " already exists")
                        log.Println("continuing with existing setup")
                } else {
                        return err
                }
        }
	err = netlink.LinkSetUp(wglink)
	if err != nil {
		log.Println("could not bring up wireguard interface")
		return err
	}
	var client models.ServerClient
	client.PrivateKey = servercfg.GetGRPCWGPrivKey()
	client.PublicKey = servercfg.GetGRPCWGPubKey()
	client.ServerEndpoint = servercfg.GetGRPCHost()
	client.Address6 = servercfg.GetGRPCWGAddress()
	client.IsServer = "yes"
	client.Network = "comms"
	err = RegisterServer(client)
        return err
}

func RegisterServer(client models.ServerClient) error {
        if client.PrivateKey == "" {
                privateKey, err := wgtypes.GeneratePrivateKey()
                if err != nil {
                        return err
                }

                client.PrivateKey = privateKey.String()
                client.PublicKey = privateKey.PublicKey().String()
        }

        if client.Address == "" {
                newAddress, err := functions.UniqueAddress6(client.Network)
                if err != nil {
                        return err
                }
                client.Address6 = newAddress
        }
	if client.Network == "" { client.Network = "comms" }
        client.ServerKey = client.PublicKey

        collection := mongoconn.Client.Database("netmaker").Collection("serverclients")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        // insert our network into the network table
        _, err := collection.InsertOne(ctx, client)
        defer cancel()

        ReconfigureServerWireGuard()

        return err
}

func ReconfigureServerWireGuard() error {
	server, err := GetServerWGConf()
	if err != nil {
                return err
        }
	serverkey, err := wgtypes.ParseKey(server.PrivateKey)
        if err != nil {
                return err
        }
	serverport, err := strconv.Atoi(servercfg.GetGRPCWGPort())
        if err != nil {
                return err
        }

	peers, err := functions.GetPeersList("comms")
        if err != nil {
                return err
        }

	wgserver, err := wgctrl.New()
	if err != nil {
		return err
	}
        var serverpeers []wgtypes.PeerConfig
	for _, peer := range peers {

                pubkey, err := wgtypes.ParseKey(peer.PublicKey)
		if err != nil {
			return err
		}
                var peercfg wgtypes.PeerConfig
                var allowedips []net.IPNet
                if peer.Address != "" {
			var peeraddr = net.IPNet{
	                        IP: net.ParseIP(peer.Address),
	                        Mask: net.CIDRMask(32, 32),
	                }
	                allowedips = append(allowedips, peeraddr)
		}
		if peer.Address6 != "" {
                        var addr6 = net.IPNet{
                                IP: net.ParseIP(peer.Address6),
                                Mask: net.CIDRMask(128, 128),
                        }
                        allowedips = append(allowedips, addr6)
                }
		peercfg = wgtypes.PeerConfig{
                        PublicKey: pubkey,
                        Endpoint: &net.UDPAddr{
                                IP:   net.ParseIP(peer.Endpoint),
                                Port: int(peer.ListenPort),
                        },
                        ReplaceAllowedIPs: true,
                        AllowedIPs: allowedips,
                }
                serverpeers = append(serverpeers, peercfg)
	}

	wgconf := wgtypes.Config{
		PrivateKey:   &serverkey,
		ListenPort:   &serverport,
		ReplacePeers: true,
		Peers:        serverpeers,
	}
	err = wgserver.ConfigureDevice(servercfg.GetGRPCWGInterface(), wgconf)
	if err != nil {
		return err
	}

	return nil
}
