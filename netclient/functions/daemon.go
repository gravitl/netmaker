package functions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
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
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// == Message Caches ==
var keepalive = new(sync.Map)
var messageCache = new(sync.Map)
var networkcontext = new(sync.Map)

const lastNodeUpdate = "lnu"
const lastPeerUpdate = "lpu"

type cachedMessage struct {
	Message  string
	LastSeen time.Time
}

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
		if time.Now().After(readMessage.LastSeen.Add(time.Minute)) { // check if message has been there over a minute
			messageCache.Delete(fmt.Sprintf("%s%s", network, which)) // remove old message if expired
			return ""
		}
		return readMessage.Message // return current message if not expired
	}
	return ""
}

// == End Message Caches ==

// Daemon runs netclient daemon from command line
func Daemon() error {
	networks, err := ncutils.GetSystemNetworks()
	if err != nil {
		return err
	}
	for _, network := range networks {
		ctx, cancel := context.WithCancel(context.Background())
		networkcontext.Store(network, cancel)
		go MessageQueue(ctx, network)
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	<-quit
	for _, network := range networks {
		if cancel, ok := networkcontext.Load(network); ok {
			cancel.(context.CancelFunc)()
		}
	}
	ncutils.Log("all done")
	return nil

}

// SetupMQTT creates a connection to broker and return client
func SetupMQTT(cfg *config.ClientConfig, publish bool) mqtt.Client {
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
			if cfg.DebugOn {
				if token := client.Subscribe("#", 0, nil); token.Wait() && token.Error() != nil {
					ncutils.Log(token.Error().Error())
					return
				}
				ncutils.Log("subscribed to all topics for debugging purposes")
			}
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
			opts.SetOrderMatters(true)
			opts.SetResumeSubs(true)
		}
	})
	opts.SetConnectionLostHandler(func(c mqtt.Client, e error) {
		ncutils.Log("detected broker connection lost, running pull for " + cfg.Node.Network)
		_, err := Pull(cfg.Node.Network, true)
		if err != nil {
			ncutils.Log("could not run pull, please restart daemon or examine network connectivity --- " + err.Error())
		}
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

// MessageQueue sets up Message Queue and subsribes/publishes updates to/from server
func MessageQueue(ctx context.Context, network string) {
	ncutils.Log("netclient go routine started for " + network)
	var cfg config.ClientConfig
	cfg.Network = network
	var configPath = fmt.Sprintf("%snetconfig-%s", ncutils.GetNetclientPathSpecific(), network)
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		ncutils.Log("could not stat config file: " + configPath)
	}
	// speed up UDP rest
	if time.Now().After(fileInfo.ModTime().Add(time.Minute)) {
		sleepTime := 2
		ncutils.Log("pulling latest config for " + cfg.Network)
		for {
			_, err := Pull(network, true)
			if err == nil {
				break
			} else {
				ncutils.PrintLog("error pulling config for "+network+": "+err.Error(), 1)
			}
			if sleepTime > 3600 {
				sleepTime = 3600
			}
			ncutils.Log("failed to pull for network " + network)
			ncutils.Log(fmt.Sprintf("waiting %d seconds to retry...", sleepTime))
			time.Sleep(time.Second * time.Duration(sleepTime))
			sleepTime = sleepTime * 2
		}
	}
	time.Sleep(time.Second << 1)
	cfg.ReadConfig()
	ncutils.Log("daemon started for network: " + network)
	client := SetupMQTT(&cfg, false)

	defer client.Disconnect(250)
	wg := &sync.WaitGroup{}
	wg.Add(2)
	checkinctx, checkincancel := context.WithCancel(context.Background())
	go Checkin(checkinctx, wg, &cfg, network)
	<-ctx.Done()
	checkincancel()
	ncutils.Log("shutting down message queue for network " + network)
	wg.Wait()
	ncutils.Log("shutdown complete")
}

// All -- mqtt message hander for all ('#') topics
var All mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("default message handler -- received message but not handling")
	ncutils.Log("Topic: " + string(msg.Topic()))
	//ncutils.Log("Message: " + string(msg.Payload()))
}

// NodeUpdate -- mqtt message handler for /update/<NodeID> topic
func NodeUpdate(client mqtt.Client, msg mqtt.Message) {
	var newNode models.Node
	var cfg config.ClientConfig
	var network = parseNetworkFromTopic(msg.Topic())
	cfg.Network = network
	cfg.ReadConfig()

	data, dataErr := decryptMsg(&cfg, msg.Payload())
	if dataErr != nil {
		return
	}
	err := json.Unmarshal([]byte(data), &newNode)
	if err != nil {
		ncutils.Log("error unmarshalling node update data" + err.Error())
		return
	}

	ncutils.Log("received message to update node " + newNode.Name)
	// see if cache hit, if so skip
	var currentMessage = read(newNode.Network, lastNodeUpdate)
	if currentMessage == string(data) {
		return
	}
	insert(newNode.Network, lastNodeUpdate, string(data)) // store new message in cache

	//check if interface name has changed if so delete.
	if cfg.Node.Interface != newNode.Interface {
		if err = wireguard.RemoveConf(cfg.Node.Interface, true); err != nil {
			ncutils.PrintLog("could not delete old interface "+cfg.Node.Interface+": "+err.Error(), 1)
		}
	}
	newNode.PullChanges = "no"
	//ensure that OS never changes
	newNode.OS = runtime.GOOS
	// check if interface needs to delta
	ifaceDelta := ncutils.IfaceDelta(&cfg.Node, &newNode)
	shouldDNSChange := cfg.Node.DNSOn != newNode.DNSOn

	cfg.Node = newNode
	switch newNode.Action {
	case models.NODE_DELETE:
		if cancel, ok := networkcontext.Load(newNode.Network); ok {
			ncutils.Log("cancelling message queue context for " + newNode.Network)
			cancel.(context.CancelFunc)()
		} else {
			ncutils.Log("failed to kill go routines for network " + newNode.Network)
		}
		ncutils.PrintLog(fmt.Sprintf("received delete request for %s", cfg.Node.Name), 1)
		if err = LeaveNetwork(cfg.Node.Network); err != nil {
			if !strings.Contains("rpc error", err.Error()) {
				ncutils.PrintLog(fmt.Sprintf("failed to leave, please check that local files for network %s were removed", cfg.Node.Network), 1)
			}
			ncutils.PrintLog(fmt.Sprintf("%s was removed", cfg.Node.Name), 1)
			return
		}
		ncutils.PrintLog(fmt.Sprintf("%s was removed", cfg.Node.Name), 1)
		return
	case models.NODE_UPDATE_KEY:
		if err := UpdateKeys(&cfg, client); err != nil {
			ncutils.PrintLog("err updating wireguard keys: "+err.Error(), 1)
		}
	case models.NODE_NOOP:
	default:
	}
	// Save new config
	cfg.Node.Action = models.NODE_NOOP
	if err := config.Write(&cfg, cfg.Network); err != nil {
		ncutils.PrintLog("error updating node configuration: "+err.Error(), 1)
	}
	nameserver := cfg.Server.CoreDNSAddr
	privateKey, err := wireguard.RetrievePrivKey(newNode.Network)
	if err != nil {
		ncutils.Log("error reading PrivateKey " + err.Error())
		return
	}
	file := ncutils.GetNetclientPathSpecific() + cfg.Node.Interface + ".conf"
	if err := wireguard.UpdateWgInterface(file, privateKey, nameserver, newNode); err != nil {
		ncutils.Log("error updating wireguard config " + err.Error())
		return
	}
	if ifaceDelta {
		ncutils.Log("applying WG conf to " + file)
		err = wireguard.ApplyConf(&cfg.Node, cfg.Node.Interface, file)
		if err != nil {
			ncutils.Log("error restarting wg after node update " + err.Error())
			return
		}
		time.Sleep(time.Second >> 1)
		// if err = Resubscribe(client, &cfg); err != nil {
		// 	ncutils.Log("error resubscribing after interface change " + err.Error())
		// 	return
		// }
		if newNode.DNSOn == "yes" {
			for _, server := range newNode.NetworkSettings.DefaultServerAddrs {
				if server.IsLeader {
					go local.SetDNSWithRetry(newNode, server.Address)
					break
				}
			}
		}
	}
	//deal with DNS
	if newNode.DNSOn != "yes" && shouldDNSChange && cfg.Node.Interface != "" {
		ncutils.Log("settng DNS off")
		_, err := ncutils.RunCmd("/usr/bin/resolvectl revert "+cfg.Node.Interface, true)
		if err != nil {
			ncutils.Log("error applying dns" + err.Error())
		}
	}
}

// UpdatePeers -- mqtt message handler for peers/<Network>/<NodeID> topic
func UpdatePeers(client mqtt.Client, msg mqtt.Message) {
	var peerUpdate models.PeerUpdate
	var network = parseNetworkFromTopic(msg.Topic())
	var cfg = config.ClientConfig{}
	cfg.Network = network
	cfg.ReadConfig()

	data, dataErr := decryptMsg(&cfg, msg.Payload())
	if dataErr != nil {
		return
	}
	err := json.Unmarshal([]byte(data), &peerUpdate)
	if err != nil {
		ncutils.Log("error unmarshalling peer data")
		return
	}
	// see if cached hit, if so skip
	var currentMessage = read(peerUpdate.Network, lastPeerUpdate)
	if currentMessage == string(data) {
		return
	}
	insert(peerUpdate.Network, lastPeerUpdate, string(data))

	file := ncutils.GetNetclientPathSpecific() + cfg.Node.Interface + ".conf"
	err = wireguard.UpdateWgPeers(file, peerUpdate.Peers)
	if err != nil {
		ncutils.Log("error updating wireguard peers" + err.Error())
		return
	}
	//err = wireguard.SyncWGQuickConf(cfg.Node.Interface, file)
	err = wireguard.SetPeers(cfg.Node.Interface, cfg.Node.Address, cfg.Node.PersistentKeepalive, peerUpdate.Peers)
	if err != nil {
		ncutils.Log("error syncing wg after peer update " + err.Error())
		return
	}
	ncutils.Log(fmt.Sprintf("received peer update on network, %s", cfg.Network))
}

// MonitorKeepalive - checks time last server keepalive received.  If more than 3+ minutes, notify and resubscribe
// func MonitorKeepalive(ctx context.Context, wg *sync.WaitGroup, client mqtt.Client, cfg *config.ClientConfig) {
// 	defer wg.Done()
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			ncutils.Log("cancel recieved, monitor keepalive exiting")
// 			return
// 		case <-time.After(time.Second * 150):
// 			var keepalivetime time.Time
// 			keepaliveval, ok := keepalive.Load(cfg.Node.Network)
// 			if ok {
// 				keepalivetime = keepaliveval.(time.Time)
// 				if !keepalivetime.IsZero() && time.Since(keepalivetime) > time.Second*120 { // more than 2+ minutes
// 					// ncutils.Log("server keepalive not recieved recently, resubscribe to message queue")
// 					// err := Resubscribe(client, cfg)
// 					// if err != nil {
// 					// 	ncutils.Log("closing " + err.Error())
// 					// }
// 					ncutils.Log("maybe wanna call something")
// 				}
// 			}
// 		}
// 	}
// }

// ServerKeepAlive -- handler to react to keepalive messages published by server
func ServerKeepAlive(client mqtt.Client, msg mqtt.Message) {
	var currentTime = time.Now()
	keepalive.Store(parseNetworkFromTopic(msg.Topic()), currentTime)
	ncutils.PrintLog("received server keepalive at "+currentTime.String(), 2)
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

// Checkin  -- go routine that checks for public or local ip changes, publishes changes
//   if there are no updates, simply "pings" the server as a checkin
func Checkin(ctx context.Context, wg *sync.WaitGroup, cfg *config.ClientConfig, network string) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			ncutils.Log("Checkin cancelled")
			return
			//delay should be configuraable -> use cfg.Node.NetworkSettings.DefaultCheckInInterval ??
		case <-time.After(time.Second * 60):
			// ncutils.Log("Checkin running")
			//read latest config
			cfg.ReadConfig()
			if cfg.Node.Roaming == "yes" && cfg.Node.IsStatic != "yes" {
				extIP, err := ncutils.GetPublicIP()
				if err != nil {
					ncutils.PrintLog("error encountered checking ip addresses: "+err.Error(), 1)
				}
				if cfg.Node.Endpoint != extIP && extIP != "" {
					ncutils.PrintLog("endpoint has changed from "+cfg.Node.Endpoint+" to "+extIP, 1)
					cfg.Node.Endpoint = extIP
					if err := PublishNodeUpdate(cfg); err != nil {
						ncutils.Log("could not publish endpoint change")
					}
				}
				intIP, err := getPrivateAddr()
				if err != nil {
					ncutils.PrintLog("error encountered checking ip addresses: "+err.Error(), 1)
				}
				if cfg.Node.LocalAddress != intIP && intIP != "" {
					ncutils.PrintLog("local Address has changed from "+cfg.Node.LocalAddress+" to "+intIP, 1)
					cfg.Node.LocalAddress = intIP
					if err := PublishNodeUpdate(cfg); err != nil {
						ncutils.Log("could not publish local address change")
					}
				}
			} else {
				localIP, err := ncutils.GetLocalIP(cfg.Node.LocalRange)
				if err != nil {
					ncutils.PrintLog("error encountered checking ip addresses: "+err.Error(), 1)
				}
				if cfg.Node.Endpoint != localIP && localIP != "" {
					ncutils.PrintLog("endpoint has changed from "+cfg.Node.Endpoint+" to "+localIP, 1)
					cfg.Node.Endpoint = localIP
					if err := PublishNodeUpdate(cfg); err != nil {
						ncutils.Log("could not publish localip change")
					}
				}
			}
			if err := pingServer(cfg); err != nil {
				ncutils.PrintLog("could not ping server "+err.Error(), 0)
			}
			Hello(cfg, network)
			// ncutils.Log("Checkin complete")
		}
	}
}

// PublishNodeUpdates -- saves node and pushes changes to broker
func PublishNodeUpdate(cfg *config.ClientConfig) error {
	if err := config.Write(cfg, cfg.Network); err != nil {
		return err
	}
	data, err := json.Marshal(cfg.Node)
	if err != nil {
		return err
	}
	if err = publish(cfg, fmt.Sprintf("update/%s", cfg.Node.ID), data); err != nil {
		return err
	}
	return nil
}

// Hello -- ping the broker to let server know node is alive and doing fine
func Hello(cfg *config.ClientConfig, network string) {
	if err := publish(cfg, fmt.Sprintf("ping/%s", cfg.Node.ID), []byte(ncutils.Version)); err != nil {
		ncutils.Log(fmt.Sprintf("error publishing ping, %v", err))
		ncutils.Log("running pull on " + cfg.Node.Network + " to reconnect")
		_, err := Pull(cfg.Node.Network, true)
		if err != nil {
			ncutils.Log("could not run pull on " + cfg.Node.Network + ", error: " + err.Error())
		}

	}
}

// == Private ==

func publish(cfg *config.ClientConfig, dest string, msg []byte) error {
	// setup the keys
	trafficPrivKey, err := auth.RetrieveTrafficKey(cfg.Node.Network)
	if err != nil {
		return err
	}

	serverPubKey, err := ncutils.ConvertBytesToKey(cfg.Node.TrafficKeys.Server)
	if err != nil {
		return err
	}

	client := SetupMQTT(cfg, true)
	defer client.Disconnect(250)
	encrypted, err := ncutils.BoxEncrypt(msg, serverPubKey, trafficPrivKey)
	if err != nil {
		return err
	}

	if token := client.Publish(dest, 0, false, encrypted); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func parseNetworkFromTopic(topic string) string {
	return strings.Split(topic, "/")[1]
}

func decryptMsg(cfg *config.ClientConfig, msg []byte) ([]byte, error) {
	if len(msg) <= 24 { // make sure message is of appropriate length
		return nil, fmt.Errorf("recieved invalid message from broker %s", string(msg))
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

	return ncutils.BoxDecrypt(msg, serverPubKey, diskKey)
}

func pingServer(cfg *config.ClientConfig) error {
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

func getServerAddress(cfg *config.ClientConfig) string {
	var server models.ServerAddr
	for _, server = range cfg.Node.NetworkSettings.DefaultServerAddrs {
		if server.Address != "" && server.IsLeader {
			break
		}
	}
	return server.Address
}
