package functions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-ping/ping"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
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
	// == initial pull of all networks ==
	networks, _ := ncutils.GetSystemNetworks()
	for _, network := range networks {
		var cfg config.ClientConfig
		cfg.Network = network
		cfg.ReadConfig()
		initialPull(cfg.Network)
	}

	// == get all the comms networks on machine ==
	commsNetworks, err := getCommsNetworks(networks[:])
	if err != nil {
		return errors.New("no comm networks exist")
	}

	// == subscribe to all nodes on each comms network on machine ==
	for currCommsNet := range commsNetworks {
		ncutils.PrintLog("started comms network daemon, "+currCommsNet, 1)
		ctx, cancel := context.WithCancel(context.Background())
		networkcontext.Store(currCommsNet, cancel)
		go messageQueue(ctx, currCommsNet)
	}

	// == add waitgroup and cancel for checkin routine ==
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go Checkin(ctx, &wg, commsNetworks)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	<-quit
	for currCommsNet := range commsNetworks {
		if cancel, ok := networkcontext.Load(currCommsNet); ok {
			cancel.(context.CancelFunc)()
		}
	}
	cancel()
	ncutils.Log("shutting down netclient daemon")
	wg.Wait()
	ncutils.Log("shutdown complete")
	return nil
}

// UpdateKeys -- updates private key and returns new publickey
func UpdateKeys(nodeCfg *config.ClientConfig, client mqtt.Client) error {
	ncutils.Log("received message to update keys")
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		ncutils.Log("error generating privatekey " + err.Error())
		return err
	}
	file := ncutils.GetNetclientPathSpecific() + nodeCfg.Node.Interface + ".conf"
	if err := wireguard.UpdatePrivateKey(file, key.String()); err != nil {
		ncutils.Log("error updating wireguard key " + err.Error())
		return err
	}
	nodeCfg.Node.PublicKey = key.PublicKey().String()
	var commsCfg = getCommsCfgByNode(&nodeCfg.Node)
	PublishNodeUpdate(&commsCfg, nodeCfg)
	return nil
}

// PingServer -- checks if server is reachable
// use commsCfg only*
func PingServer(commsCfg *config.ClientConfig) error {
	node := getServerAddress(commsCfg)
	pinger, err := ping.NewPinger(node)
	if err != nil {
		return err
	}
	pinger.Timeout = 2 * time.Second
	pinger.Run()
	stats := pinger.Statistics()
	if stats.PacketLoss == 100 {
		return errors.New("ping error")
	}
	return nil
}

// == Private ==

// sets MQ client subscriptions for a specific node config
// should be called for each node belonging to a given comms network
func setSubscriptions(client mqtt.Client, nodeCfg *config.ClientConfig) {
	if nodeCfg.DebugOn {
		if token := client.Subscribe("#", 0, nil); token.Wait() && token.Error() != nil {
			ncutils.Log(token.Error().Error())
			return
		}
		ncutils.Log("subscribed to all topics for debugging purposes")
	}

	if token := client.Subscribe(fmt.Sprintf("update/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID), 0, mqtt.MessageHandler(NodeUpdate)); token.Wait() && token.Error() != nil {
		ncutils.Log(token.Error().Error())
		return
	}
	if nodeCfg.DebugOn {
		ncutils.Log(fmt.Sprintf("subscribed to node updates for node %s update/%s/%s", nodeCfg.Node.Name, nodeCfg.Node.Network, nodeCfg.Node.ID))
	}
	if token := client.Subscribe(fmt.Sprintf("peers/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID), 0, mqtt.MessageHandler(UpdatePeers)); token.Wait() && token.Error() != nil {
		ncutils.Log(token.Error().Error())
		return
	}
	if nodeCfg.DebugOn {
		ncutils.Log(fmt.Sprintf("subscribed to peer updates for node %s peers/%s/%s", nodeCfg.Node.Name, nodeCfg.Node.Network, nodeCfg.Node.ID))
	}
}

// on a delete usually, pass in the nodecfg to unsubscribe client broker communications
// for the node in nodeCfg
func unsubscribeNode(client mqtt.Client, nodeCfg *config.ClientConfig) {
	client.Unsubscribe(fmt.Sprintf("update/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID))
	var ok = true
	if token := client.Unsubscribe(fmt.Sprintf("update/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID)); token.Wait() && token.Error() != nil {
		ncutils.PrintLog("unable to unsubscribe from updates for node "+nodeCfg.Node.Name+"\n"+token.Error().Error(), 1)
		ok = false
	}
	if token := client.Unsubscribe(fmt.Sprintf("peers/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID)); token.Wait() && token.Error() != nil {
		ncutils.PrintLog("unable to unsubscribe from peer updates for node "+nodeCfg.Node.Name+"\n"+token.Error().Error(), 1)
		ok = false
	}
	if ok {
		ncutils.PrintLog("successfully unsubscribed node "+nodeCfg.Node.ID+" : "+nodeCfg.Node.Name, 0)
	}
}

// sets up Message Queue and subsribes/publishes updates to/from server
// the client should subscribe to ALL nodes that exist on unique comms network locally
func messageQueue(ctx context.Context, commsNet string) {
	var commsCfg config.ClientConfig
	commsCfg.Network = commsNet
	commsCfg.ReadConfig()
	ncutils.Log("netclient daemon started for network: " + commsNet)
	client := setupMQTT(&commsCfg, false)
	defer client.Disconnect(250)
	<-ctx.Done()
	ncutils.Log("shutting down daemon for comms network " + commsNet)
}

// setupMQTT creates a connection to broker and return client
// utilizes comms client configs to setup connections
func setupMQTT(commsCfg *config.ClientConfig, publish bool) mqtt.Client {
	opts := mqtt.NewClientOptions()
	server := getServerAddress(commsCfg)
	opts.AddBroker(server + ":1883")             // TODO get the appropriate port of the comms mq server
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
				ncutils.Log("error retriving networks " + err.Error())
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
		ncutils.Log("detected broker connection lost, running pull for " + commsCfg.Node.Network)
		_, err := Pull(commsCfg.Node.Network, true)
		if err != nil {
			ncutils.Log("could not run pull, server unreachable: " + err.Error())
			ncutils.Log("waiting to retry...")
		}
		ncutils.Log("connection re-established with mqtt server")
	})

	client := mqtt.NewClient(opts)
	tperiod := time.Now().Add(12 * time.Second)
	for {
		//if after 12 seconds, try a gRPC pull on the last try
		if time.Now().After(tperiod) {
			ncutils.Log("running pull for " + commsCfg.Node.Network)
			_, err := Pull(commsCfg.Node.Network, true)
			if err != nil {
				ncutils.Log("could not run pull, exiting " + commsCfg.Node.Network + " setup: " + err.Error())
				return client
			}
			time.Sleep(time.Second)
		}
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			ncutils.Log("unable to connect to broker, retrying ...")
			if time.Now().After(tperiod) {
				ncutils.Log("could not connect to broker, exiting " + commsCfg.Node.Network + " setup: " + token.Error().Error())
				if strings.Contains(token.Error().Error(), "connectex") || strings.Contains(token.Error().Error(), "i/o timeout") {
					ncutils.PrintLog("connection issue detected.. pulling and restarting daemon", 0)
					Pull(commsCfg.Node.Network, true)
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
func publishSignal(commsCfg, nodeCfg *config.ClientConfig, signal byte) error {
	if err := publish(commsCfg, nodeCfg, fmt.Sprintf("signal/%s", nodeCfg.Node.ID), []byte{signal}, 1); err != nil {
		return err
	}
	return nil
}

func initialPull(network string) {
	ncutils.Log("pulling latest config for " + network)
	var configPath = fmt.Sprintf("%snetconfig-%s", ncutils.GetNetclientPathSpecific(), network)
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		ncutils.Log("could not stat config file: " + configPath)
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
			ncutils.Log("failed to pull for network " + network)
			ncutils.Log(fmt.Sprintf("waiting %d seconds to retry...", sleepTime))
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

func getCommsNetworks(networks []string) (map[string]bool, error) {
	var cfg config.ClientConfig
	var response = make(map[string]bool, 1)
	for _, network := range networks {
		cfg.Network = network
		cfg.ReadConfig()
		response[cfg.Node.CommID] = true
	}
	return response, nil
}

func getCommsCfgByNode(node *models.Node) config.ClientConfig {
	var commsCfg config.ClientConfig
	commsCfg.Network = node.Network
	commsCfg.ReadConfig()
	return commsCfg
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

// == End Message Caches ==
