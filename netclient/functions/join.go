package functions

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/models/promodels"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/global_settings"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/term"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// JoinViaSso - Handles the Single Sign-On flow on the end point VPN client side
// Contacts the server provided by the user (and thus specified in cfg.SsoServer)
// get the URL to authenticate with a provider and shows the user the URL.
// Then waits for user to authenticate with the URL.
// Upon user successful auth flow finished - server should return access token to the requested network
// Otherwise the error message is sent which can be displayed to the user
func JoinViaSSo(cfg *config.ClientConfig, privateKey string) error {

	// User must tell us which network he is joining
	if cfg.Node.Network == "" {
		return errors.New("no network provided")
	}

	// Prepare a channel for interrupt
	// Channel to listen for interrupt signal to terminate gracefully
	interrupt := make(chan os.Signal, 1)
	// Notify the interrupt channel for SIGINT
	signal.Notify(interrupt, os.Interrupt)

	// Web Socket is used, construct the URL accordingly ...
	socketUrl := fmt.Sprintf("wss://%s/api/oauth/node-handler", cfg.SsoServer)
	// Dial the netmaker server controller
	conn, _, err := websocket.DefaultDialer.Dial(socketUrl, nil)
	if err != nil {
		logger.Log(0, fmt.Sprintf("error connecting to %s : %s", cfg.Server.API, err.Error()))
		return err
	}
	// Don't forget to close when finished
	defer conn.Close()
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

	var loginMsg promodels.LoginMsg
	loginMsg.Mac = cfg.Node.MacAddress
	loginMsg.Network = cfg.Node.Network
	if global_settings.User != "" {
		fmt.Printf("Continuing with user, %s.\nPlease input password:\n", global_settings.User)
		pass, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil || string(pass) == "" {
			logger.FatalLog("no password provided, exiting")
		}
		loginMsg.User = global_settings.User
		loginMsg.Password = string(pass)
		fmt.Println("attempting login...")
	}

	msgTx, err := json.Marshal(loginMsg)
	if err != nil {
		logger.Log(0, fmt.Sprintf("failed to marshal message %+v", loginMsg))
		return err
	}
	err = conn.WriteMessage(websocket.TextMessage, []byte(msgTx))
	if err != nil {
		logger.FatalLog("Error during writing to websocket:", err.Error())
		return err
	}

	// if user provided, server will handle authentication
	if loginMsg.User == "" {
		// We are going to get instructions on how to authenticate
		// Wait to receive something from server
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		// Print message from the netmaker controller to the user
		fmt.Printf("Please visit:\n %s \n to authenticate", string(msg))
	}

	// Now the user is authenticating and we need to block until received
	// An answer from the server.
	// Server waits ~5 min - If takes too long timeout will be triggered by the server
	done := make(chan struct{})
	defer close(done)
	// Following code will run in a separate go routine
	// it reads a message from the server which either contains 'AccessToken:' string or not
	// if not - then it contains an Error to display.
	// if yes - then AccessToken is to be used to proceed joining the network
	go func() {
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				if msgType < 0 {
					logger.Log(1, "received close message from server")
					done <- struct{}{}
					return
				}
				// Error reading a message from the server
				if !strings.Contains(err.Error(), "normal") {
					logger.Log(0, "read:", err.Error())
				}
				return
			}

			if msgType == websocket.CloseMessage {
				logger.Log(1, "received close message from server")
				done <- struct{}{}
				return
			}
			// Get the access token from the response
			if strings.Contains(string(msg), "AccessToken: ") {
				// Access was granted
				rxToken := strings.TrimPrefix(string(msg), "AccessToken: ")
				accesstoken, err := config.ParseAccessToken(rxToken)
				if err != nil {
					logger.Log(0, fmt.Sprintf("failed to parse received access token %s,err=%s\n", accesstoken, err.Error()))
					return
				}

				cfg.Network = accesstoken.ClientConfig.Network
				cfg.Node.Network = accesstoken.ClientConfig.Network
				cfg.AccessKey = accesstoken.ClientConfig.Key
				cfg.Node.LocalRange = accesstoken.ClientConfig.LocalRange
				//cfg.Server.Server = accesstoken.ServerConfig.Server
				cfg.Server.API = accesstoken.APIConnString
			} else {
				// Access was not granted. Display a message from the server
				logger.Log(0, "Message from server:", string(msg))
				cfg.AccessKey = ""
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			logger.Log(1, "finished")
			return nil
		case <-interrupt:
			logger.Log(0, "interrupt received, closing connection")
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				logger.Log(0, "write close:", err.Error())
				return err
			}
			return nil
		}
	}
}

// JoinNetwork - helps a client join a network
func JoinNetwork(cfg *config.ClientConfig, privateKey string) error {
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
		cfg.Node.Password = logic.GenPassWord()
	}
	//check if ListenPort was set on command line
	if cfg.Node.ListenPort != 0 {
		cfg.Node.UDPHolePunch = "no"
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
			logger.Log(1, "network:", cfg.Network, "error retrieving private address: ", err.Error())
		}
	}
	if len(cfg.Node.Interfaces) == 0 {
		ip, err := getInterfaces()
		if err != nil {
			logger.Log(0, "failed to retrive local interfaces", err.Error())
		} else {
			cfg.Node.Interfaces = *ip
		}
	}

	// set endpoint if blank. set to local if local net, retrieve from function if not
	if cfg.Node.Endpoint == "" {
		if cfg.Node.IsLocal == "yes" && cfg.Node.LocalAddress != "" {
			cfg.Node.Endpoint = cfg.Node.LocalAddress
		} else {
			cfg.Node.Endpoint, err = ncutils.GetPublicIP(cfg.Server.API)
		}
		if err != nil || cfg.Node.Endpoint == "" {
			logger.Log(0, "network:", cfg.Network, "error setting cfg.Node.Endpoint.")
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
		if err != nil || len(macs) == 0 {
			//if macaddress can't be found set to random string
			cfg.Node.MacAddress = ncutils.MakeRandomString(18)
		} else {
			cfg.Node.MacAddress = macs[0]
		}
	}

	if ncutils.IsFreeBSD() {
		cfg.Node.UDPHolePunch = "no"
		cfg.Node.FirewallInUse = models.FIREWALL_IPTABLES // nftables not supported by FreeBSD
	}

	if cfg.Node.FirewallInUse == "" {
		if ncutils.IsNFTablesPresent() {
			cfg.Node.FirewallInUse = models.FIREWALL_NFTABLES
		} else if ncutils.IsIPTablesPresent() {
			cfg.Node.FirewallInUse = models.FIREWALL_IPTABLES
		} else {
			cfg.Node.FirewallInUse = models.FIREWALL_NONE
		}
	}

	// make sure name is appropriate, if not, give blank name
	cfg.Node.Name = formatName(cfg.Node)
	cfg.Node.OS = runtime.GOOS
	cfg.Node.Version = ncutils.Version
	cfg.Node.AccessKey = cfg.AccessKey
	//not sure why this is needed ... setnode defaults should take care of this on server
	cfg.Node.IPForwarding = "yes"
	logger.Log(0, "joining "+cfg.Network+" at "+cfg.Server.API)
	url := "https://" + cfg.Server.API + "/api/nodes/" + cfg.Network
	response, err := API(cfg.Node, http.MethodPost, url, cfg.AccessKey)
	if err != nil {
		return fmt.Errorf("error creating node %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		bodybytes, _ := io.ReadAll(response.Body)
		return fmt.Errorf("error creating node %s %s", response.Status, string(bodybytes))
	}
	var nodeGET models.NodeGet
	if err := json.NewDecoder(response.Body).Decode(&nodeGET); err != nil {
		//not sure the next line will work as response.Body probably needs to be reset before it can be read again
		bodybytes, _ := io.ReadAll(response.Body)
		return fmt.Errorf("error decoding node from server %w %s", err, string(bodybytes))
	}
	node := nodeGET.Node
	if nodeGET.Peers == nil {
		nodeGET.Peers = []wgtypes.PeerConfig{}
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
	cfg.Server = nodeGET.ServerConfig

	err = wireguard.StorePrivKey(privateKey, cfg.Network)
	if err != nil {
		return err
	}
	if node.IsPending == "yes" {
		logger.Log(0, "network:", cfg.Network, "node is marked as PENDING.")
		logger.Log(0, "network:", cfg.Network, "awaiting approval from Admin before configuring WireGuard.")
		if cfg.Daemon != "off" {
			return daemon.InstallDaemon()
		}
	}
	logger.Log(1, "network:", cfg.Node.Network, "node created on remote server...updating configs")
	err = ncutils.ModPort(&node)
	if err != nil {
		return err
	}
	informPortChange(&node)

	err = config.ModNodeConfig(&node)
	if err != nil {
		return err
	}
	err = config.ModServerConfig(&cfg.Server, node.Network)
	if err != nil {
		return err
	}
	// attempt to make backup
	if err = config.SaveBackup(node.Network); err != nil {
		logger.Log(0, "network:", node.Network, "failed to make backup, node will not auto restore if config is corrupted")
	}

	local.SetNetmakerDomainRoute(cfg.Server.API)
	cfg.Node = node
	logger.Log(0, "starting wireguard")
	err = wireguard.InitWireguard(&node, privateKey, nodeGET.Peers[:])
	if err != nil {
		return err
	}
	if cfg.Server.Server == "" {
		return errors.New("did not receive broker address from registration")
	}
	if cfg.Daemon == "install" || ncutils.IsFreeBSD() {
		err = daemon.InstallDaemon()
		if err != nil {
			return err
		}
	}

	if err := daemon.Restart(); err != nil {
		logger.Log(3, "daemon restart failed:", err.Error())
		if err := daemon.Start(); err != nil {
			return err
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
		logger.Log(1, "network:", node.Network, "could not properly format name: "+node.Name)
		logger.Log(1, "network:", node.Network, "setting name to blank")
		node.Name = ""
	}
	return node.Name
}
