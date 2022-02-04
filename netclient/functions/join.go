package functions

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"runtime"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/server"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.org/x/crypto/nacl/box"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
)

// JoinNetwork - helps a client join a network
func JoinNetwork(cfg config.ClientConfig, privateKey string) error {
	if cfg.Node.Network == "" {
		return errors.New("no network provided")
	}

	var err error
	if local.HasNetwork(cfg.Network) {
		err := errors.New("ALREADY_INSTALLED. Netclient appears to already be installed for " + cfg.Network + ". To re-install, please remove by executing 'sudo netclient leave -n " + cfg.Network + "'. Then re-run the install command.")
		return err
	}

	err = config.Write(&cfg, cfg.Network)
	if err != nil {
		return err
	}
	if cfg.Node.Password == "" {
		cfg.Node.Password = ncutils.GenPass()
	}
	var trafficPubKey, trafficPrivKey, errT = box.GenerateKey(rand.Reader) // generate traffic keys
	if errT != nil {
		return errT
	}

	// == handle keys ==
	if err = auth.StoreSecret(cfg.Node.Password, cfg.Node.Network); err != nil {
		return err
	}

	if err = auth.StoreTrafficKey(trafficPrivKey, cfg.Node.Network); err != nil {
		return err
	}

	trafficPubKeyBytes, err := ncutils.ConvertKeyToBytes(trafficPubKey)
	if err != nil {
		return err
	} else if trafficPubKeyBytes == nil {
		return fmt.Errorf("traffic key is nil")
	}

	cfg.Node.TrafficKeys.Mine = trafficPubKeyBytes
	cfg.Node.TrafficKeys.Server = nil
	// == end handle keys ==

	if cfg.Node.LocalRange != "" && cfg.Node.LocalAddress == "" {
		log.Println("local vpn, getting local address from range: " + cfg.Node.LocalRange)
		cfg.Node.LocalAddress = getLocalIP(cfg.Node)
	} else if cfg.Node.LocalAddress == "" {
		intIP, err := getPrivateAddr()
		if err == nil {
			cfg.Node.LocalAddress = intIP
		}
	}

	// set endpoint if blank. set to local if local net, retrieve from function if not
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
			log.Fatal(err)
		}
		privateKey = wgPrivatekey.String()
		cfg.Node.PublicKey = wgPrivatekey.PublicKey().String()
	}

	// Find and set node MacAddress
	if cfg.Node.MacAddress == "" {
		macs, err := ncutils.GetMacAddr()
		if err != nil {
			return err
		} else {
			cfg.Node.MacAddress = macs[0]
		}
	}

	if ncutils.IsLinux() {
		_, err := exec.LookPath("resolvectl")
		if err != nil {
			ncutils.PrintLog("resolvectl not present", 2)
			ncutils.PrintLog("unable to configure DNS automatically, disabling automated DNS management", 2)
			cfg.Node.DNSOn = "no"
		}
	}
	if ncutils.IsFreeBSD() {
		cfg.Node.UDPHolePunch = "no"
	}
	// make sure name is appropriate, if not, give blank name
	cfg.Node.Name = formatName(cfg.Node)
	// differentiate between client/server here
	var node = models.Node{
		Password:            cfg.Node.Password,
		Address:             cfg.Node.Address,
		Address6:            cfg.Node.Address6,
		ID:                  cfg.Node.ID,
		MacAddress:          cfg.Node.MacAddress,
		AccessKey:           cfg.Server.AccessKey,
		IsStatic:            cfg.Node.IsStatic,
		Roaming:             cfg.Node.Roaming,
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
		TrafficKeys:         cfg.Node.TrafficKeys,
		OS:                  runtime.GOOS,
		Version:             ncutils.Version,
	}

	ncutils.Log("joining " + cfg.Network + " at " + cfg.Server.GRPCAddress)
	var wcclient nodepb.NodeServiceClient

	conn, err := grpc.Dial(cfg.Server.GRPCAddress,
		ncutils.GRPCRequestOpts(cfg.Server.GRPCSSL))

	if err != nil {
		log.Fatalf("Unable to establish client connection to "+cfg.Server.GRPCAddress+": %v", err)
	}
	defer conn.Close()
	wcclient = nodepb.NewNodeServiceClient(conn)

	// get free port based on returned default listen port
	node.ListenPort, err = ncutils.GetFreePort(node.ListenPort)
	if err != nil {
		fmt.Printf("Error retrieving port: %v", err)
	}

	// safety check. If returned node from server is local, but not currently configured as local, set to local addr
	if cfg.Node.IsLocal != "yes" && node.IsLocal == "yes" && node.LocalRange != "" {
		node.LocalAddress, err = ncutils.GetLocalIP(node.LocalRange)
		if err != nil {
			return err
		}
		node.Endpoint = node.LocalAddress
	}
	if ncutils.IsFreeBSD() {
		node.UDPHolePunch = "no"
		cfg.Node.IsStatic = "yes"
	}

	err = wireguard.StorePrivKey(privateKey, cfg.Network)
	if err != nil {
		return err
	}
	if node.IsPending == "yes" {
		ncutils.Log("Node is marked as PENDING.")
		ncutils.Log("Awaiting approval from Admin before configuring WireGuard.")
		if cfg.Daemon != "off" {
			return daemon.InstallDaemon(cfg)
		}
	}
	data, err := json.Marshal(&node)
	if err != nil {
		return err
	}
	// Create node on server
	res, err := wcclient.CreateNode(
		context.TODO(),
		&nodepb.Object{
			Data: string(data),
			Type: nodepb.NODE_TYPE,
		},
	)
	if err != nil {
		return err
	}
	ncutils.PrintLog("node created on remote server...updating configs", 1)

	nodeData := res.Data
	if err = json.Unmarshal([]byte(nodeData), &node); err != nil {
		return err
	}

	cfg.Node = node
	err = config.ModConfig(&node)
	if err != nil {
		return err
	}
	// attempt to make backup
	if err = config.SaveBackup(node.Network); err != nil {
		ncutils.Log("failed to make backup, node will not auto restore if config is corrupted")
	}

	ncutils.Log("retrieving peers")
	peers, hasGateway, gateways, err := server.GetPeers(node.MacAddress, cfg.Network, cfg.Server.GRPCAddress, node.IsDualStack == "yes", node.IsIngressGateway == "yes", node.IsServer == "yes")
	if err != nil && !ncutils.IsEmptyRecord(err) {
		ncutils.Log("failed to retrieve peers")
		return err
	}

	ncutils.Log("starting wireguard")
	err = wireguard.InitWireguard(&node, privateKey, peers, hasGateway, gateways, false)
	if err != nil {
		return err
	}
	if cfg.Daemon != "off" {
		err = daemon.InstallDaemon(cfg)
	}
	if err != nil {
		return err
	} else {
		daemon.Restart()
	}

	return err
}

// format name appropriately. Set to blank on failure
func formatName(node models.Node) string {
	// Logic to properly format name
	if !node.NameInNodeCharSet() {
		node.Name = ncutils.DNSFormatString(node.Name)
	}
	if len(node.Name) > models.MAX_NAME_LENGTH {
		node.Name = ncutils.ShortenString(node.Name, models.MAX_NAME_LENGTH)
	}
	if !node.NameInNodeCharSet() || len(node.Name) > models.MAX_NAME_LENGTH {
		ncutils.PrintLog("could not properly format name: "+node.Name, 1)
		ncutils.PrintLog("setting name to blank", 1)
		node.Name = ""
	}
	return node.Name
}
