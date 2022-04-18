package functions

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-ping/ping"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	ssl "github.com/gravitl/netmaker/tls"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var messageCache = new(sync.Map)
var networkcontext = new(sync.Map)

const lastNodeUpdate = "lnu"
const lastPeerUpdate = "lpu"

type cachedMessage struct {
	Message  string
	LastSeen time.Time
}

// Daemon runs netclient daemon from command line
func Daemon() error {
	var exists = struct{}{}
	serverSet := make(map[string]struct{})
	// == initial pull of all networks ==
	networks, _ := ncutils.GetSystemNetworks()
	if len(networks) == 0 {
		return errors.New("no networks")
	}
	for _, network := range networks {
		logger.Log(3, "initializing network", network)
		cfg := config.ClientConfig{}
		cfg.Network = network
		cfg.ReadConfig()
		serverSet[cfg.Server.Server] = exists
		//temporary code --- remove in version v0.13.0
		removeHostDNS(network, ncutils.IsWindows())
		// end of code to be removed in version v0.13.0
		initialPull(cfg.Network)
	}

	// == subscribe to all nodes for each on machine ==
	for server := range serverSet {
		logger.Log(1, "started daemon for server ", server)
		ctx, cancel := context.WithCancel(context.Background())
		networkcontext.Store(server, cancel)
		go messageQueue(ctx, server)
	}

	// == add waitgroup and cancel for checkin routine ==
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go Checkin(ctx, &wg, serverSet)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	<-quit
	for server := range serverSet {
		if cancel, ok := networkcontext.Load(server); ok {
			cancel.(context.CancelFunc)()
		}
	}
	cancel()
	logger.Log(0, "shutting down netclient daemon")
	wg.Wait()
	logger.Log(0, "shutdown complete")
	return nil
}

// UpdateKeys -- updates private key and returns new publickey
func UpdateKeys(nodeCfg *config.ClientConfig, client mqtt.Client) error {
	logger.Log(0, "received message to update wireguard keys for network ", nodeCfg.Network)
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		logger.Log(0, "error generating privatekey ", err.Error())
		return err
	}
	file := ncutils.GetNetclientPathSpecific() + nodeCfg.Node.Interface + ".conf"
	if err := wireguard.UpdatePrivateKey(file, key.String()); err != nil {
		logger.Log(0, "error updating wireguard key ", err.Error())
		return err
	}
	if storeErr := wireguard.StorePrivKey(key.String(), nodeCfg.Network); storeErr != nil {
		logger.Log(0, "failed to save private key", storeErr.Error())
		return storeErr
	}

	nodeCfg.Node.PublicKey = key.PublicKey().String()
	PublishNodeUpdate(nodeCfg)
	return nil
}

// PingServer -- checks if server is reachable
// use commsCfg only*
func PingServer(cfg *config.ClientConfig) error {
	pinger, err := ping.NewPinger(cfg.Server.Server)
	if err != nil {
		return err
	}
	pinger.Timeout = 2 * time.Second
	pinger.Run()
	stats := pinger.Statistics()
	if stats.PacketLoss == 100 {
		return errors.New("ping error")
	}
	logger.Log(3, "ping of server", cfg.Server.Server, "was successful")
	return nil
}

// == Private ==

// sets MQ client subscriptions for a specific node config
// should be called for each node belonging to a given server
func setSubscriptions(client mqtt.Client, nodeCfg *config.ClientConfig) {
	if nodeCfg.DebugOn {
		if token := client.Subscribe("#", 0, nil); token.Wait() && token.Error() != nil {
			logger.Log(0, token.Error().Error())
			return
		}
		logger.Log(0, "subscribed to all topics for debugging purposes")
	}
	if token := client.Subscribe(fmt.Sprintf("update/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID), 0, mqtt.MessageHandler(NodeUpdate)); token.Wait() && token.Error() != nil {
		logger.Log(0, token.Error().Error())
		return
	}
	logger.Log(3, fmt.Sprintf("subscribed to node updates for node %s update/%s/%s", nodeCfg.Node.Name, nodeCfg.Node.Network, nodeCfg.Node.ID))
	if token := client.Subscribe(fmt.Sprintf("peers/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID), 0, mqtt.MessageHandler(UpdatePeers)); token.Wait() && token.Error() != nil {
		logger.Log(0, token.Error().Error())
		return
	}
	logger.Log(3, fmt.Sprintf("subscribed to peer updates for node %s peers/%s/%s", nodeCfg.Node.Name, nodeCfg.Node.Network, nodeCfg.Node.ID))
}

// on a delete usually, pass in the nodecfg to unsubscribe client broker communications
// for the node in nodeCfg
func unsubscribeNode(client mqtt.Client, nodeCfg *config.ClientConfig) {
	client.Unsubscribe(fmt.Sprintf("update/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID))
	var ok = true
	if token := client.Unsubscribe(fmt.Sprintf("update/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID)); token.Wait() && token.Error() != nil {
		logger.Log(1, "unable to unsubscribe from updates for node ", nodeCfg.Node.Name, "\n", token.Error().Error())
		ok = false
	}
	if token := client.Unsubscribe(fmt.Sprintf("peers/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID)); token.Wait() && token.Error() != nil {
		logger.Log(1, "unable to unsubscribe from peer updates for node ", nodeCfg.Node.Name, "\n", token.Error().Error())
		ok = false
	}
	if ok {
		logger.Log(1, "successfully unsubscribed node ", nodeCfg.Node.ID, " : ", nodeCfg.Node.Name)
	}
}

// sets up Message Queue and subsribes/publishes updates to/from server
// the client should subscribe to ALL nodes that exist on server locally
func messageQueue(ctx context.Context, server string) {
	logger.Log(0, "netclient daemon started for server: ", server)
	client := setupMQTTSub(server)
	defer client.Disconnect(250)
	<-ctx.Done()
	logger.Log(0, "shutting down daemon for server ", server)
}

// setupMQTTSub creates a connection to broker and subscribes to topic
// utilizes comms client configs to setup connections
func setupMQTTSub(server string) mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker("ssl://" + server + ":8883") // TODO get the appropriate port of the comms mq server
	opts.TLSConfig = NewTLSConfig(nil, server)
	opts.ClientID = ncutils.MakeRandomString(23) // helps avoid id duplication on broker
	opts.SetDefaultPublishHandler(All)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(time.Second << 2)
	opts.SetKeepAlive(time.Minute >> 1)
	opts.SetWriteTimeout(time.Minute)
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		networks, err := ncutils.GetSystemNetworks()
		if err != nil {
			logger.Log(0, "error retriving networks ", err.Error())
		}
		for _, network := range networks {
			var currNodeCfg config.ClientConfig
			currNodeCfg.Network = network
			currNodeCfg.ReadConfig()
			if currNodeCfg.Server.Server == server {
				setSubscriptions(client, &currNodeCfg)
			}
		}
	})
	opts.SetOrderMatters(true)
	opts.SetResumeSubs(true)
	opts.SetConnectionLostHandler(func(c mqtt.Client, e error) {
		logger.Log(0, "detected broker connection lost, running pull for all nodes")
		networks, err := ncutils.GetSystemNetworks()
		if err != nil {
			logger.Log(0, "error retriving networks ", err.Error())
		}
		for _, network := range networks {
			var cfg config.ClientConfig
			cfg.Network = network
			cfg.ReadConfig()
			_, err := Pull(cfg.Node.Network, true)
			if err != nil {
				logger.Log(0, "could not run pull, server unreachable: ", err.Error())
				logger.Log(0, "waiting to retry...")
			}
		}
		// don't think following log message is accurate
		//logger.Log(0, "connection re-established with mqtt server")
	})

	client := mqtt.NewClient(opts)
	tperiod := time.Now().Add(12 * time.Second)
	for {
		//if after 12 seconds, try a gRPC pull on the last try
		if time.Now().After(tperiod) {
			networks, err := ncutils.GetSystemNetworks()
			if err != nil {
				logger.Log(0, "error retriving networks ", err.Error())
			}
			for _, network := range networks {
				var cfg config.ClientConfig
				cfg.Network = network
				cfg.ReadConfig()
				if cfg.Server.Server == server {
					_, err := Pull(cfg.Node.Network, true)
					if err != nil {
						logger.Log(0, "could not run pull, exiting ", cfg.Node.Network, " setup: ", err.Error())
						return client
					}
				}
			}
			time.Sleep(time.Second)
		}
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			logger.Log(0, "unable to connect to broker, retrying ...")
			if time.Now().After(tperiod) {
				logger.Log(0, "could not connect to broker, exiting ", server, " setup: ", token.Error().Error())
				if strings.Contains(token.Error().Error(), "connectex") || strings.Contains(token.Error().Error(), "i/o timeout") {
					logger.Log(0, "connection issue detected.. restarting daemon")
					daemon.Restart()
				}
				return client
			}
		} else {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return client
}

// NewTLSConf sets up tls configuration to connect to broker securely
func NewTLSConfig(cfg *config.ClientConfig, server string) *tls.Config {
	var file string
	if cfg != nil {
		server = cfg.Server.Server
	}
	file = ncutils.GetNetclientServerPath(server) + "/root.pem"
	certpool := x509.NewCertPool()
	ca, err := os.ReadFile(file)
	if err != nil {
		logger.Log(0, "could not read CA file ", err.Error())
	}
	ok := certpool.AppendCertsFromPEM(ca)
	if !ok {
		logger.Log(0, "failed to append cert")
	}
	clientKeyPair, err := tls.LoadX509KeyPair(ncutils.GetNetclientServerPath(server)+"/client.pem", ncutils.GetNetclientPath()+"/client.key")
	if err != nil {
		log.Fatalf("could not read client cert/key %v \n", err)
	}
	certs := []tls.Certificate{clientKeyPair}
	return &tls.Config{
		RootCAs:      certpool,
		ClientAuth:   tls.NoClientCert,
		ClientCAs:    nil,
		Certificates: certs,
		//InsecureSkipVerify: false  fails ---- so need to use VerifyConnection
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if cs.ServerName != server {
				logger.Log(0, "VerifyConnection - certifiate mismatch")
				return errors.New("certificate doesn't match server")
			}
			ca, err := ssl.ReadCert(ncutils.GetNetclientServerPath(cs.ServerName) + "/root.pem")
			if err != nil {
				logger.Log(0, "VerifyConnection - unable to read ca", err.Error())
				return errors.New("unable to read ca")
			}
			for _, cert := range cs.PeerCertificates {
				if cert.IsCA {
					if string(cert.PublicKey.(ed25519.PublicKey)) != string(ca.PublicKey.(ed25519.PublicKey)) {
						logger.Log(0, "VerifyConnection - public key mismatch")
						return errors.New("cert public key does not match ca public key")
					}
				}
			}
			return nil
		},
	}
}

// setupMQTT creates a connection to broker and return client
// this function is primarily used to create a connection to publish to the broker
func setupMQTT(cfg *config.ClientConfig, publish bool) mqtt.Client {
	opts := mqtt.NewClientOptions()
	server := cfg.Server.Server
	opts.AddBroker("ssl://" + server + ":8883") // TODO get the appropriate port of the comms mq server
	opts.TLSConfig = NewTLSConfig(cfg, "")
	opts.ClientID = ncutils.MakeRandomString(23) // helps avoid id duplication on broker
	opts.SetDefaultPublishHandler(All)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(time.Second << 2)
	opts.SetKeepAlive(time.Minute >> 1)
	opts.SetWriteTimeout(time.Minute)
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		if !publish {
			networks, err := ncutils.GetSystemNetworks()
			if err != nil {
				logger.Log(0, "error retriving networks ", err.Error())
			}
			for _, network := range networks {
				var currNodeCfg config.ClientConfig
				currNodeCfg.Network = network
				currNodeCfg.ReadConfig()
				setSubscriptions(client, &currNodeCfg)
			}
		}
	})
	opts.SetOrderMatters(true)
	opts.SetResumeSubs(true)
	opts.SetConnectionLostHandler(func(c mqtt.Client, e error) {
		logger.Log(0, "detected broker connection lost, running pull for ", cfg.Node.Network)
		_, err := Pull(cfg.Node.Network, true)
		if err != nil {
			logger.Log(0, "could not run pull, server unreachable: ", err.Error())
			logger.Log(0, "waiting to retry...")
		}
		logger.Log(0, "connection re-established with mqtt server")
	})

	client := mqtt.NewClient(opts)
	tperiod := time.Now().Add(12 * time.Second)
	for {
		//if after 12 seconds, try a gRPC pull on the last try
		if time.Now().After(tperiod) {
			logger.Log(0, "running pull for ", cfg.Node.Network)
			_, err := Pull(cfg.Node.Network, true)
			if err != nil {
				logger.Log(0, "could not run pull, exiting ", cfg.Node.Network, " setup: ", err.Error())
				return client
			}
			time.Sleep(time.Second)
		}
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			logger.Log(0, "unable to connect to broker, retrying ...")
			if time.Now().After(tperiod) {
				logger.Log(0, "could not connect to broker, exiting ", cfg.Node.Network, " setup: ", token.Error().Error())
				if strings.Contains(token.Error().Error(), "connectex") || strings.Contains(token.Error().Error(), "i/o timeout") {
					logger.Log(0, "connection issue detected.. pulling and restarting daemon")
					Pull(cfg.Node.Network, true)
					daemon.Restart()
				}
				return client
			}
		} else {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return client
}

// publishes a message to server to update peers on this peer's behalf
func publishSignal(nodeCfg *config.ClientConfig, signal byte) error {
	if err := publish(nodeCfg, fmt.Sprintf("signal/%s", nodeCfg.Node.ID), []byte{signal}, 1); err != nil {
		return err
	}
	return nil
}

func initialPull(network string) {
	logger.Log(0, "pulling latest config for ", network)
	var configPath = fmt.Sprintf("%snetconfig-%s", ncutils.GetNetclientPathSpecific(), network)
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		logger.Log(0, "could not stat config file: ", configPath)
		return
	}
	// speed up UDP rest
	if !fileInfo.ModTime().IsZero() && time.Now().After(fileInfo.ModTime().Add(time.Minute)) {
		sleepTime := 2
		for {
			_, err := Pull(network, true)
			if err == nil {
				break
			}
			if sleepTime > 3600 {
				sleepTime = 3600
			}
			logger.Log(0, "failed to pull for network ", network)
			logger.Log(0, fmt.Sprintf("waiting %d seconds to retry...", sleepTime))
			time.Sleep(time.Second * time.Duration(sleepTime))
			sleepTime = sleepTime * 2
		}
		time.Sleep(time.Second << 1)
	}
}

func parseNetworkFromTopic(topic string) string {
	return strings.Split(topic, "/")[1]
}

// should only ever use node client configs
func decryptMsg(nodeCfg *config.ClientConfig, msg []byte) ([]byte, error) {
	if len(msg) <= 24 { // make sure message is of appropriate length
		return nil, fmt.Errorf("recieved invalid message from broker %v", msg)
	}

	// setup the keys
	diskKey, keyErr := auth.RetrieveTrafficKey(nodeCfg.Node.Network)
	if keyErr != nil {
		return nil, keyErr
	}

	serverPubKey, err := ncutils.ConvertBytesToKey(nodeCfg.Node.TrafficKeys.Server)
	if err != nil {
		return nil, err
	}

	return ncutils.DeChunk(msg, serverPubKey, diskKey)
}

func getServerAddress(cfg *config.ClientConfig) string {
	var server models.ServerAddr
	for _, server = range cfg.Node.NetworkSettings.DefaultServerAddrs {
		if server.Address != "" && server.IsLeader {
			break
		}
	}
	return server.Address
}

// == Message Caches ==

func insert(network, which, cache string) {
	var newMessage = cachedMessage{
		Message:  cache,
		LastSeen: time.Now(),
	}
	messageCache.Store(fmt.Sprintf("%s%s", network, which), newMessage)
}

func read(network, which string) string {
	val, isok := messageCache.Load(fmt.Sprintf("%s%s", network, which))
	if isok {
		var readMessage = val.(cachedMessage) // fetch current cached message
		if readMessage.LastSeen.IsZero() {
			return ""
		}
		if time.Now().After(readMessage.LastSeen.Add(time.Minute * 10)) { // check if message has been there over a minute
			messageCache.Delete(fmt.Sprintf("%s%s", network, which)) // remove old message if expired
			return ""
		}
		return readMessage.Message // return current message if not expired
	}
	return ""
}
