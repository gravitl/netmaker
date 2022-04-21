package functions

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
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
func JoinNetwork(cfg *config.ClientConfig, privateKey string, iscomms bool) error {
	if cfg.Node.Network == "" {
		return errors.New("no network provided")
	}

	var err error
	if local.HasNetwork(cfg.Network) {
		err := errors.New("ALREADY_INSTALLED. Netclient appears to already be installed for " + cfg.Network + ". To re-install, please remove by executing 'sudo netclient leave -n " + cfg.Network + "'. Then re-run the install command.")
		return err
	}

	err = config.Write(cfg, cfg.Network)
	if err != nil {
		return err
	}
	if cfg.Node.Password == "" {
		cfg.Node.Password = logic.GenKey()
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

	if cfg.Node.LocalAddress == "" {
		intIP, err := getPrivateAddr()
		if err == nil {
			cfg.Node.LocalAddress = intIP
		} else {
			logger.Log(1, "error retrieving private address: ", err.Error())
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
			logger.Log(0, "Error setting cfg.Node.Endpoint.")
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
		if err != nil || iscomms {
			//if macaddress can't be found set to random string
			cfg.Node.MacAddress = ncutils.MakeRandomString(18)
		} else {
			cfg.Node.MacAddress = macs[0]
		}
	}

	//	if ncutils.IsLinux() {
	//		_, err := exec.LookPath("resolvectl")
	//		if err != nil {
	//			logger.Log("resolvectl not present", 2)
	//			logger.Log("unable to configure DNS automatically, disabling automated DNS management", 2)
	//			cfg.Node.DNSOn = "no"
	//		}
	//	}
	if ncutils.IsFreeBSD() {
		cfg.Node.UDPHolePunch = "no"
	}
	// make sure name is appropriate, if not, give blank name
	cfg.Node.Name = formatName(cfg.Node)
	// differentiate between client/server here
	var node = models.Node{
		Password:   cfg.Node.Password,
		Address:    cfg.Node.Address,
		Address6:   cfg.Node.Address6,
		ID:         cfg.Node.ID,
		MacAddress: cfg.Node.MacAddress,
		AccessKey:  cfg.Server.AccessKey,
		IsStatic:   cfg.Node.IsStatic,
		//Roaming:             cfg.Node.Roaming,
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
		UDPHolePunch:        cfg.Node.UDPHolePunch,
		TrafficKeys:         cfg.Node.TrafficKeys,
		OS:                  runtime.GOOS,
		Version:             ncutils.Version,
	}

	logger.Log(0, "joining "+cfg.Network+" at "+cfg.Server.GRPCAddress)
	var wcclient nodepb.NodeServiceClient

	conn, err := grpc.Dial(cfg.Server.GRPCAddress,
		ncutils.GRPCRequestOpts(cfg.Server.GRPCSSL))

	if err != nil {
		log.Fatalf("Unable to establish client connection to "+cfg.Server.GRPCAddress+": %v", err)
	}
	defer conn.Close()
	wcclient = nodepb.NewNodeServiceClient(conn)

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
		logger.Log(0, "Node is marked as PENDING.")
		logger.Log(0, "Awaiting approval from Admin before configuring WireGuard.")
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
	logger.Log(1, "node created on remote server...updating configs")

	// keep track of the old listenport value
	oldListenPort := node.ListenPort

	nodeData := res.Data
	if err = json.Unmarshal([]byte(nodeData), &node); err != nil {
		return err
	}

	cfg.Node = node

	setListenPort(oldListenPort, cfg)

	err = config.ModConfig(&cfg.Node)
	if err != nil {
		return err
	}
	// attempt to make backup
	if err = config.SaveBackup(node.Network); err != nil {
		logger.Log(0, "failed to make backup, node will not auto restore if config is corrupted")
	}

	logger.Log(0, "retrieving peers")
	peers, hasGateway, gateways, err := server.GetPeers(node.MacAddress, cfg.Network, cfg.Server.GRPCAddress, node.IsDualStack == "yes", node.IsIngressGateway == "yes", node.IsServer == "yes")
	if err != nil && !ncutils.IsEmptyRecord(err) {
		logger.Log(0, "failed to retrieve peers")
		return err
	}

	logger.Log(0, "starting wireguard")
	err = wireguard.InitWireguard(&node, privateKey, peers, hasGateway, gateways, false)
	if err != nil {
		return err
	}
	//	if node.DNSOn == "yes" {
	//		for _, server := range node.NetworkSettings.DefaultServerAddrs {
	//			if server.IsLeader {
	//				go func()
	//					if !local.SetDNSWithRetry(node, server.Address) {
	//						cfg.Node.DNSOn = "no"
	//						var currentCommsCfg = getCommsCfgByNode(&cfg.Node)
	//						PublishNodeUpdate(&currentCommsCfg, &cfg)
	//					}
	//				}()
	//				break
	//			}
	//		}
	//	}

	if !iscomms {
		if cfg.Daemon != "off" {
			err = daemon.InstallDaemon(cfg)
		}
		if err != nil {
			return err
		} else {
			daemon.Restart()
		}
	}

	return nil
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
		logger.Log(1, "could not properly format name: "+node.Name)
		logger.Log(1, "setting name to blank")
		node.Name = ""
	}
	return node.Name
}

func setListenPort(oldListenPort int32, cfg *config.ClientConfig) {
	// keep track of the returned listenport value
	newListenPort := cfg.Node.ListenPort

	if newListenPort != oldListenPort {
		var errN error
		// get free port based on returned default listen port
		cfg.Node.ListenPort, errN = ncutils.GetFreePort(cfg.Node.ListenPort)
		if errN != nil {
			cfg.Node.ListenPort = newListenPort
			logger.Log(1, "Error retrieving port: ", errN.Error())
		}

		// if newListenPort has been modified to find an available port, publish to server
		if cfg.Node.ListenPort != newListenPort {
			PublishNodeUpdate(cfg)
		}
	}
}
