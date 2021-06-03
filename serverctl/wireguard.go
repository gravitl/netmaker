package serverctl

import (
        //"github.com/davecgh/go-spew/spew"
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
			log.Println("WireGuard interface " + ifaceSettings.Name + " already exists. Skipping...")
		} else {
			return err
		}
	}
	address, err := netlink.ParseAddr(wgconfig.GRPCWGAddress + "/24")
	if err != nil {
		return err
	}

	err = netlink.AddrAdd(wglink, address)
        if err != nil && !os.IsExist(err){
                        return err
        }
	err = netlink.LinkSetUp(wglink)
	if err != nil {
		log.Println("could not bring up wireguard interface")
		return err
	}
	var client models.IntClient
	client.PrivateKey = wgconfig.GRPCWGPrivKey
	client.PublicKey = wgconfig.GRPCWGPubKey
	client.ServerPublicEndpoint = servercfg.GetAPIHost()
	client.ServerAPIPort = servercfg.GetAPIPort()
	client.ServerPrivateAddress = servercfg.GetGRPCWGAddress()
	client.ServerWGPort = servercfg.GetGRPCWGPort()
	client.ServerGRPCPort = servercfg.GetGRPCPort()
	client.Address = servercfg.GetGRPCWGAddress()
	client.IsServer = "yes"
	client.Network = "comms"
	exists, _ := functions.ServerIntClientExists()
	if exists {

	}
	err = RegisterServer(client)
        return err
}

func DeleteServerClient() error {
	return nil
}


func RegisterServer(client models.IntClient) error {
        if client.PrivateKey == "" {
                privateKey, err := wgtypes.GeneratePrivateKey()
                if err != nil {
                        return err
                }

                client.PrivateKey = privateKey.String()
                client.PublicKey = privateKey.PublicKey().String()
        }

        if client.Address == "" {
                newAddress, err := functions.UniqueAddress(client.Network)
                if err != nil {
                        return err
                }
		if newAddress == "" {
			return errors.New("Could not retrieve address")
		}
                client.Address = newAddress
        }
	if client.Network == "" { client.Network = "comms" }
        client.ServerKey = client.PublicKey

        collection := mongoconn.Client.Database("netmaker").Collection("intclients")
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

	peers, err := functions.GetIntPeersList()
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
	wgiface := servercfg.GetGRPCWGInterface()
	err = wgserver.ConfigureDevice(wgiface, wgconf)
	if err != nil {
		return err
	}
	return nil
}
