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
	client := setupMQTT(false)
	defer client.Disconnect(250)
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	networks, _ := ncutils.GetSystemNetworks()
	for _, network := range networks {
		var cfg config.ClientConfig
		cfg.Network = network
		cfg.ReadConfig()
		initialPull(cfg.Network)
	}
	wg.Add(1)
	go Checkin(ctx, wg)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	<-quit
	cancel()
	ncutils.Log("shutting down message queue ")
	wg.Wait()
	ncutils.Log("shutdown complete")
	return nil
}

// UpdateKeys -- updates private key and returns new publickey
func UpdateKeys(cfg *config.ClientConfig, client mqtt.Client) error {
	ncutils.Log("received message to update keys")
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		ncutils.Log("error generating privatekey " + err.Error())
		return err
	}
	file := ncutils.GetNetclientPathSpecific() + cfg.Node.Interface + ".conf"
	if err := wireguard.UpdatePrivateKey(file, key.String()); err != nil {
		ncutils.Log("error updating wireguard key " + err.Error())
		return err
	}
	cfg.Node.PublicKey = key.PublicKey().String()
	if err := config.ModConfig(&cfg.Node); err != nil {
		ncutils.Log("error updating local config " + err.Error())
	}
	PublishNodeUpdate(cfg)
	if err = wireguard.ApplyConf(&cfg.Node, cfg.Node.Interface, file); err != nil {
		ncutils.Log("error applying new config " + err.Error())
		return err
	}
	return nil
}

// PingServer -- checks if server is reachable
func PingServer(cfg *config.ClientConfig) error {
	node := getServerAddress(cfg)
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

// setupMQTT creates a connection to broker and return client
func setupMQTT(publish bool) mqtt.Client {
	var cfg *config.ClientConfig
	cfg.Network = ncutils.COMMS_NETWORK_NAME
	cfg.ReadConfig()
	opts := mqtt.NewClientOptions()
	server := getServerAddress(cfg)
	opts.AddBroker(server + ":1883")
	id := ncutils.MakeRandomString(23)
	opts.ClientID = id
	opts.SetDefaultPublishHandler(All)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(time.Second << 2)
	opts.SetKeepAlive(time.Minute >> 1)
	opts.SetWriteTimeout(time.Minute)
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		if !publish {
			SetSubscriptions(client, cfg)
		}
	})
	opts.SetOrderMatters(true)
	opts.SetResumeSubs(true)
	opts.SetConnectionLostHandler(func(c mqtt.Client, e error) {
		ncutils.Log("detected broker connection lost, running pull for " + cfg.Node.Network)
		_, err := Pull(cfg.Node.Network, true)
		if err != nil {
			ncutils.Log("could not run pull, server unreachable: " + err.Error())
			ncutils.Log("waiting to retry...")
			/*
				//Consider putting in logic to restart - daemon may take long time to refresh
				time.Sleep(time.Minute * 5)
					ncutils.Log("restarting netclient")
					daemon.Restart()
			*/
		}
		ncutils.Log("connection re-established with mqtt server")
	})

	client := mqtt.NewClient(opts)
	tperiod := time.Now().Add(12 * time.Second)
	for {
		//if after 12 seconds, try a gRPC pull on the last try
		if time.Now().After(tperiod) {
			ncutils.Log("running pull for " + cfg.Node.Network)
			_, err := Pull(cfg.Node.Network, true)
			if err != nil {
				ncutils.Log("could not run pull, exiting " + cfg.Node.Network + " setup: " + err.Error())
				return client
			}
			time.Sleep(time.Second)
		}
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			ncutils.Log("unable to connect to broker, retrying ...")
			if time.Now().After(tperiod) {
				ncutils.Log("could not connect to broker, exiting " + cfg.Node.Network + " setup: " + token.Error().Error())
				if strings.Contains(token.Error().Error(), "connectex") || strings.Contains(token.Error().Error(), "i/o timeout") {
					ncutils.PrintLog("connection issue detected.. pulling and restarting daemon", 0)
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

// SetSubscriptions - sets MQ subscriptions
func SetSubscriptions(client mqtt.Client, cfg *config.ClientConfig) {
	if cfg.DebugOn {
		if token := client.Subscribe("#", 0, nil); token.Wait() && token.Error() != nil {
			ncutils.Log(token.Error().Error())
			return
		}
		ncutils.Log("subscribed to all topics for debugging purposes")
	}
	networks, err := ncutils.GetSystemNetworks()
	if err != nil {
		ncutils.Log("error retriving networks " + err.Error())
	}
	for _, network := range networks {
		var cfg config.ClientConfig
		cfg.Network = network
		cfg.ReadConfig()

		if token := client.Subscribe(fmt.Sprintf("update/%s/%s", cfg.Node.Network, cfg.Node.ID), 0, mqtt.MessageHandler(NodeUpdate)); token.Wait() && token.Error() != nil {
			ncutils.Log(token.Error().Error())
			return
		}
		if cfg.DebugOn {
			ncutils.Log(fmt.Sprintf("subscribed to node updates for node %s update/%s/%s", cfg.Node.Name, cfg.Node.Network, cfg.Node.ID))
		}
		if token := client.Subscribe(fmt.Sprintf("peers/%s/%s", cfg.Node.Network, cfg.Node.ID), 0, mqtt.MessageHandler(UpdatePeers)); token.Wait() && token.Error() != nil {
			ncutils.Log(token.Error().Error())
			return
		}
		if cfg.DebugOn {
			ncutils.Log(fmt.Sprintf("subscribed to peer updates for node %s peers/%s/%s", cfg.Node.Name, cfg.Node.Network, cfg.Node.ID))
		}
	}
}

// publishes a message to server to update peers on this peer's behalf
func publishSignal(cfg *config.ClientConfig, signal byte) error {
	if err := publish(cfg, fmt.Sprintf("signal/%s", cfg.Node.ID), []byte{signal}, 1); err != nil {
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

func decryptMsg(cfg *config.ClientConfig, msg []byte) ([]byte, error) {
	if len(msg) <= 24 { // make sure message is of appropriate length
		return nil, fmt.Errorf("recieved invalid message from broker %v", msg)
	}

	// setup the keys
	diskKey, keyErr := auth.RetrieveTrafficKey(cfg.Node.Network)
	if keyErr != nil {
		return nil, keyErr
	}

	serverPubKey, err := ncutils.ConvertBytesToKey(cfg.Node.TrafficKeys.Server)
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

// == End Message Caches ==
