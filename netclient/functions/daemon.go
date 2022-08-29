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

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/global_settings"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	ssl "github.com/gravitl/netmaker/tls"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var messageCache = new(sync.Map)

var serverSet map[string]bool

var mqclient mqtt.Client

const lastNodeUpdate = "lnu"
const lastPeerUpdate = "lpu"

type cachedMessage struct {
	Message  string
	LastSeen time.Time
}

// Daemon runs netclient daemon from command line
func Daemon() error {
	logger.Log(0, "netclient daemon started -- version:", ncutils.Version)
	UpdateClientConfig()
	if err := ncutils.SavePID(); err != nil {
		return err
	}
	// reference required to eliminate unused statticcheck
	serverSet = make(map[string]bool)
	serverSet["dummy"] = false
	// set ipforwarding on startup
	err := local.SetIPForwarding()
	if err != nil {
		logger.Log(0, err.Error())
	}

	// == add waitgroup and cancel for checkin routine ==
	wg := sync.WaitGroup{}
	quit := make(chan os.Signal, 1)
	reset := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	signal.Notify(reset, syscall.SIGHUP)
	cancel := startGoRoutines(&wg)
	for {
		select {
		case <-quit:
			cancel()
			logger.Log(0, "shutting down netclient daemon")
			wg.Wait()
			logger.Log(0, "shutdown complete")
			return nil
		case <-reset:
			logger.Log(0, "received reset")
			cancel()
			wg.Wait()
			logger.Log(0, "restarting daemon")
			cancel = startGoRoutines(&wg)
		}
	}
}

func startGoRoutines(wg *sync.WaitGroup) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go Checkin(ctx, wg)
	serverSet := make(map[string]bool)
	networks, _ := ncutils.GetSystemNetworks()
	for _, network := range networks {
		logger.Log(3, "initializing network", network)
		cfg := config.ClientConfig{}
		cfg.Network = network
		cfg.ReadConfig()
		if err := wireguard.ApplyConf(&cfg.Node, cfg.Node.Interface, ncutils.GetNetclientPathSpecific()+cfg.Node.Interface+".conf"); err != nil {
			logger.Log(0, "failed to start ", cfg.Node.Interface, "wg interface", err.Error())
		}
		if cfg.PublicIPService != "" {
			global_settings.PublicIPServices[network] = cfg.PublicIPService
		}

		server := cfg.Server.Server
		if !serverSet[server] {
			// == subscribe to all nodes for each on machine ==
			serverSet[server] = true
			logger.Log(1, "started daemon for server ", server)
			err := local.SetNetmakerDomainRoute(cfg.Server.API)
			if err != nil {
				logger.Log(0, "error setting route for netmaker: "+err.Error())
			}
			wg.Add(1)
			go messageQueue(ctx, wg, &cfg)
		}
	}
	return cancel
}

// UpdateKeys -- updates private key and returns new publickey
func UpdateKeys(nodeCfg *config.ClientConfig, client mqtt.Client) error {
	logger.Log(0, "interface:", nodeCfg.Node.Interface, "received message to update wireguard keys for network ", nodeCfg.Network)
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		logger.Log(0, "network:", nodeCfg.Node.Network, "error generating privatekey ", err.Error())
		return err
	}
	file := ncutils.GetNetclientPathSpecific() + nodeCfg.Node.Interface + ".conf"
	if err := wireguard.UpdatePrivateKey(file, key.String()); err != nil {
		logger.Log(0, "network:", nodeCfg.Node.Network, "error updating wireguard key ", err.Error())
		return err
	}
	if storeErr := wireguard.StorePrivKey(key.String(), nodeCfg.Network); storeErr != nil {
		logger.Log(0, "network:", nodeCfg.Network, "failed to save private key", storeErr.Error())
		return storeErr
	}

	nodeCfg.Node.PublicKey = key.PublicKey().String()
	PublishNodeUpdate(nodeCfg)
	return nil
}

// == Private ==

// sets MQ client subscriptions for a specific node config
// should be called for each node belonging to a given server
func setSubscriptions(client mqtt.Client, nodeCfg *config.ClientConfig) {
	if token := client.Subscribe(fmt.Sprintf("update/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID), 0, mqtt.MessageHandler(NodeUpdate)); token.WaitTimeout(mq.MQ_TIMEOUT*time.Second) && token.Error() != nil {
		if token.Error() == nil {
			logger.Log(0, "network:", nodeCfg.Node.Network, "connection timeout")
		} else {
			logger.Log(0, "network:", nodeCfg.Node.Network, token.Error().Error())
		}
		return
	}
	logger.Log(3, fmt.Sprintf("subscribed to node updates for node %s update/%s/%s", nodeCfg.Node.Name, nodeCfg.Node.Network, nodeCfg.Node.ID))
	if token := client.Subscribe(fmt.Sprintf("peers/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID), 0, mqtt.MessageHandler(UpdatePeers)); token.Wait() && token.Error() != nil {
		logger.Log(0, "network", nodeCfg.Node.Network, token.Error().Error())
		return
	}
	logger.Log(3, fmt.Sprintf("subscribed to peer updates for node %s peers/%s/%s", nodeCfg.Node.Name, nodeCfg.Node.Network, nodeCfg.Node.ID))
}

// on a delete usually, pass in the nodecfg to unsubscribe client broker communications
// for the node in nodeCfg
func unsubscribeNode(client mqtt.Client, nodeCfg *config.ClientConfig) {
	client.Unsubscribe(fmt.Sprintf("update/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID))
	var ok = true
	if token := client.Unsubscribe(fmt.Sprintf("update/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID)); token.WaitTimeout(mq.MQ_TIMEOUT*time.Second) && token.Error() != nil {
		if token.Error() == nil {
			logger.Log(1, "network:", nodeCfg.Node.Network, "unable to unsubscribe from updates for node ", nodeCfg.Node.Name, "\n", "connection timeout")
		} else {
			logger.Log(1, "network:", nodeCfg.Node.Network, "unable to unsubscribe from updates for node ", nodeCfg.Node.Name, "\n", token.Error().Error())
		}
		ok = false
	}
	if token := client.Unsubscribe(fmt.Sprintf("peers/%s/%s", nodeCfg.Node.Network, nodeCfg.Node.ID)); token.WaitTimeout(mq.MQ_TIMEOUT*time.Second) && token.Error() != nil {
		if token.Error() == nil {
			logger.Log(1, "network:", nodeCfg.Node.Network, "unable to unsubscribe from peer updates for node", nodeCfg.Node.Name, "\n", "connection timeout")
		} else {
			logger.Log(1, "network:", nodeCfg.Node.Network, "unable to unsubscribe from peer updates for node", nodeCfg.Node.Name, "\n", token.Error().Error())
		}
		ok = false
	}
	if ok {
		logger.Log(1, "network:", nodeCfg.Node.Network, "successfully unsubscribed node ", nodeCfg.Node.ID, " : ", nodeCfg.Node.Name)
	}
}

// sets up Message Queue and subsribes/publishes updates to/from server
// the client should subscribe to ALL nodes that exist on server locally
func messageQueue(ctx context.Context, wg *sync.WaitGroup, cfg *config.ClientConfig) {
	defer wg.Done()
	logger.Log(0, "network:", cfg.Node.Network, "netclient message queue started for server:", cfg.Server.Server)
	err := setupMQTT(cfg)
	if err != nil {
		logger.Log(0, "unable to connect to broker", cfg.Server.Server, err.Error())
		return
	}
	//defer mqclient.Disconnect(250)
	<-ctx.Done()
	logger.Log(0, "shutting down message queue for server", cfg.Server.Server)
}

// NewTLSConf sets up tls configuration to connect to broker securely
func NewTLSConfig(server string) (*tls.Config, error) {
	file := ncutils.GetNetclientServerPath(server) + ncutils.GetSeparator() + "root.pem"
	certpool := x509.NewCertPool()
	ca, err := os.ReadFile(file)
	if err != nil {
		logger.Log(0, "could not read CA file", err.Error())
	}
	ok := certpool.AppendCertsFromPEM(ca)
	if !ok {
		logger.Log(0, "failed to append cert")
	}
	clientKeyPair, err := tls.LoadX509KeyPair(ncutils.GetNetclientServerPath(server)+ncutils.GetSeparator()+"client.pem", ncutils.GetNetclientPath()+ncutils.GetSeparator()+"client.key")
	if err != nil {
		logger.Log(0, "could not read client cert/key", err.Error())
		return nil, err
	}
	certs := []tls.Certificate{clientKeyPair}
	return &tls.Config{
		RootCAs:            certpool,
		ClientAuth:         tls.NoClientCert,
		ClientCAs:          nil,
		Certificates:       certs,
		InsecureSkipVerify: false,
	}, nil

}

// setupMQTT creates a connection to broker and returns client
// this function is primarily used to create a connection to publish to the broker
func setupMQTT(cfg *config.ClientConfig) error {
	opts := mqtt.NewClientOptions()
	server := cfg.Server.Server
	port := cfg.Server.MQPort
	opts.AddBroker("ssl://" + server + ":" + port)
	tlsConfig, err := NewTLSConfig(server)
	if err != nil {
		logger.Log(0, "failed to get TLS config for", server, err.Error())
		return err
	}
	opts.SetTLSConfig(tlsConfig)
	opts.SetClientID(ncutils.MakeRandomString(23))
	opts.SetDefaultPublishHandler(All)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(time.Second << 2)
	opts.SetKeepAlive(time.Minute >> 1)
	opts.SetWriteTimeout(time.Minute)

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		networks, err := ncutils.GetSystemNetworks()
		if err != nil {
			logger.Log(0, "error retriving networks", err.Error())
		}
		for _, network := range networks {
			var currNodeCfg config.ClientConfig
			currNodeCfg.Network = network
			currNodeCfg.ReadConfig()
			setSubscriptions(client, &currNodeCfg)
		}
	})
	opts.SetOrderMatters(true)
	opts.SetResumeSubs(true)
	opts.SetConnectionLostHandler(func(c mqtt.Client, e error) {
		logger.Log(0, "network:", cfg.Node.Network, "detected broker connection lost for", cfg.Server.Server)
	})
	mqclient = mqtt.NewClient(opts)
	var connecterr error
	for count := 0; count < 3; count++ {
		connecterr = nil
		if token := mqclient.Connect(); !token.WaitTimeout(30*time.Second) || token.Error() != nil {
			logger.Log(0, "unable to connect to broker, retrying ...")
			if token.Error() == nil {
				connecterr = errors.New("connect timeout")
			} else {
				connecterr = token.Error()
			}
			if err := checkBroker(cfg.Server.Server, cfg.Server.MQPort); err != nil {
				logger.Log(0, "could not connect to broker", cfg.Server.Server, err.Error())
			}
		}
	}
	if connecterr != nil {
		reRegisterWithServer(cfg)
		//try after re-registering
		if token := mqclient.Connect(); !token.WaitTimeout(30*time.Second) || token.Error() != nil {
			return errors.New("unable to connect to broker")
		}
	}

	return nil
}

func reRegisterWithServer(cfg *config.ClientConfig) {
	logger.Log(0, "connection issue detected.. attempt connection with new certs and broker information")
	key, err := ssl.ReadKeyFromFile(ncutils.GetNetclientPath() + ncutils.GetSeparator() + "client.key")
	if err != nil {
		_, *key, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			log.Fatal("could not generate new key")
		}
	}
	RegisterWithServer(key, cfg)
	daemon.Restart()
}

// publishes a message to server to update peers on this peer's behalf
func publishSignal(nodeCfg *config.ClientConfig, signal byte) error {
	if err := publish(nodeCfg, fmt.Sprintf("signal/%s", nodeCfg.Node.ID), []byte{signal}, 1); err != nil {
		return err
	}
	return nil
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
		if time.Now().After(readMessage.LastSeen.Add(time.Hour * 24)) { // check if message has been there over a minute
			messageCache.Delete(fmt.Sprintf("%s%s", network, which)) // remove old message if expired
			return ""
		}
		return readMessage.Message // return current message if not expired
	}
	return ""
}
