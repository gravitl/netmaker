package functions

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
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

	tcping "github.com/cloverstd/tcping/ping"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	ssl "github.com/gravitl/netmaker/tls"
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
	serverSet := make(map[string]config.ClientConfig)
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
		serverSet[cfg.Server.Server] = cfg
		//temporary code --- remove in version v0.13.0
		removeHostDNS(network, ncutils.IsWindows())
		// end of code to be removed in version v0.13.0
		initialPull(cfg.Network)
	}

	// == subscribe to all nodes for each on machine ==
	for server, config := range serverSet {
		logger.Log(1, "started daemon for server ", server)
		ctx, cancel := context.WithCancel(context.Background())
		networkcontext.Store(server, cancel)
		go messageQueue(ctx, &config)
	}

	// == add waitgroup and cancel for checkin routine ==
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go Checkin(ctx, &wg)
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
func PingServer(cfg *config.ClientConfig) error {
	pinger := tcping.NewTCPing()
	pinger.SetTarget(&tcping.Target{
		Protocol: tcping.TCP,
		Host:     cfg.Server.Server,
		Port:     8883,
		Counter:  3,
		Interval: 1 * time.Second,
		Timeout:  2 * time.Second,
	})
	pingerDone := pinger.Start()
	select {
	case <-pingerDone:
		break
	}

	if pinger.Result().SuccessCounter == 0 {
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
func messageQueue(ctx context.Context, cfg *config.ClientConfig) {
	logger.Log(0, "netclient daemon started for server: ", cfg.Server.Server)
	client := setupMQTT(cfg, false)
	defer client.Disconnect(250)
	<-ctx.Done()
	logger.Log(0, "shutting down daemon for server ", cfg.Server.Server)
}

// NewTLSConf sets up tls configuration to connect to broker securely
func NewTLSConfig(server string) *tls.Config {
	file := ncutils.GetNetclientServerPath(server) + ncutils.GetSeparator() + "root.pem"
	certpool := x509.NewCertPool()
	ca, err := os.ReadFile(file)
	if err != nil {
		logger.Log(0, "could not read CA file ", err.Error())
	}
	ok := certpool.AppendCertsFromPEM(ca)
	if !ok {
		logger.Log(0, "failed to append cert")
	}
	clientKeyPair, err := tls.LoadX509KeyPair(ncutils.GetNetclientServerPath(server)+ncutils.GetSeparator()+"client.pem", ncutils.GetNetclientPath()+ncutils.GetSeparator()+"client.key")
	if err != nil {
		log.Fatalf("could not read client cert/key %v \n", err)
	}
	certs := []tls.Certificate{clientKeyPair}
	return &tls.Config{
		RootCAs:            certpool,
		ClientAuth:         tls.NoClientCert,
		ClientCAs:          nil,
		Certificates:       certs,
		InsecureSkipVerify: false,
	}
}

// setupMQTT creates a connection to broker and returns client
// this function is primarily used to create a connection to publish to the broker
func setupMQTT(cfg *config.ClientConfig, publish bool) mqtt.Client {
	opts := mqtt.NewClientOptions()
	server := cfg.Server.Server
	opts.AddBroker("ssl://" + server + ":8883") // TODO get the appropriate port of the comms mq server
	opts.SetTLSConfig(NewTLSConfig(server))
	opts.SetClientID(ncutils.MakeRandomString(23))
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
		logger.Log(0, "detected broker connection lost for", cfg.Server.Server)
	})
	client := mqtt.NewClient(opts)
	for token := client.Connect(); !token.WaitTimeout(30*time.Second) || token.Error() != nil; token = client.Connect() {
		logger.Log(0, "unable to connect to broker, retrying ...")
		var err error
		if token.Error() == nil {
			err = errors.New("connect timeout")
		} else {
			err = token.Error()
		}
		logger.Log(0, "could not connect to broker", cfg.Server.Server, err.Error())
		if strings.Contains(err.Error(), "connectex") || strings.Contains(err.Error(), "connect timeout") {
			logger.Log(0, "connection issue detected.. attempt connection with new certs")
			key, err := ssl.ReadKey(ncutils.GetNetclientPath() + ncutils.GetSeparator() + "client.key")
			if err != nil {
				_, *key, err = ed25519.GenerateKey(rand.Reader)
				if err != nil {
					log.Fatal("could not generate new key")
				}
			}
			RegisterWithServer(key, cfg)
			daemon.Restart()
		}
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
