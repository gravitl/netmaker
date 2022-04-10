package functions

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"github.com/gravitl/netmaker/tls"
	"github.com/kr/pretty"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// JoinNetwork - helps a client join a network
func JoinNetwork(cfg *config.ClientConfig, privateKey string) error {
	log.Println("starting join")
	pretty.Println(cfg.Node)
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

	// == handle keys ==
	if err = auth.StoreSecret(cfg.Node.Password, cfg.Node.Network); err != nil {
		return err
	}

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
		if err != nil {
			//if macaddress can't be found set to random string
			cfg.Node.MacAddress = ncutils.MakeRandomString(18)
		} else {
			cfg.Node.MacAddress = macs[0]
		}
	}

	if ncutils.IsFreeBSD() {
		cfg.Node.UDPHolePunch = "no"
	}
	// make sure name is appropriate, if not, give blank name
	cfg.Node.Name = formatName(cfg.Node)

	seed := tls.NewKey()
	key, err := seed.Ed25519PrivateKey()
	if err != nil {
		return err
	}

	request := config.JoinRequest{
		Node: cfg.Node,
		Key:  key.Public().(ed25519.PublicKey),
	}

	log.Println("calling api ", cfg.Server.API+"/api/nodes/join")
	response, err := join(request, "https://"+cfg.Server.API+"/api/nodes/join", cfg.Node.AccessKey)
	if err != nil {
		return fmt.Errorf("error joining network %w", err)
	}
	node := response.Config.Node
	peers := response.Peers

	// safety check. If returned no:de from server is local, but not currently configured as local, set to local addr
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

	// keep track of the old listenport value
	oldListenPort := node.ListenPort

	cfg.Node = node

	setListenPort(oldListenPort, cfg)

	log.Println("join- modconfig")
	err = config.ModConfig(&cfg.Node)
	if err != nil {
		return err
	}
	// attempt to make backup
	if err = config.SaveBackup(node.Network); err != nil {
		logger.Log(0, "failed to make backup, node will not auto restore if config is corrupted")
	}
	log.Println("init wireguard")
	err = wireguard.InitWireguard(&node, privateKey, peers, false)
	if err != nil {
		return err
	}

	log.Println("save certs")
	//save CA, certificate and key
	if err := tls.SaveCert("/etc/netclient/"+cfg.Server.ServerName, "root.pem", &response.CA); err != nil {
		return err
	}
	if err := tls.SaveCert("/etc/netclient/"+cfg.Server.ServerName, "client.pem", &response.Certificate); err != nil {
		return err
	}
	if err := tls.SaveKey("/etc/netclient/"+cfg.Server.ServerName, "client.key", key); err != nil {
		return err
	}

	log.Println("start daemaon")
	if cfg.Daemon != "off" {
		err = daemon.InstallDaemon(cfg)
	}
	if err != nil {
		return err
	} else {
		daemon.Restart()
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

func join(node config.JoinRequest, url, authorization string) (*config.JoinResponse, error) {
	var request *http.Request
	var joinResponse config.JoinResponse
	payload, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}
	request, err = http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("authorization", "Bearer "+authorization)
	fmt.Println("sending api request", url, authorization)
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error join network %s", response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(&joinResponse); err != nil {
		return nil, err
	}
	return &joinResponse, nil
}
