package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// ServerKeepalive  - stores time of last server keepalive message
var keepalive = new(sync.Map)
var messageCache = new(sync.Map)

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
	ncutils.Log("storing new message: " + cache)
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
			ncutils.Log("cached message expired")
			return ""
		}
		ncutils.Log("cache hit, skipping probably " + readMessage.Message)
		return readMessage.Message // return current message if not expired
	}
	return ""
}

// Daemon runs netclient daemon from command line
func Daemon() error {
	ctx, cancel := context.WithCancel(context.Background())
	networks, err := ncutils.GetSystemNetworks()
	if err != nil {
		cancel()
		return err
	}
	for _, network := range networks {
		go MessageQueue(ctx, network)
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	<-quit
	cancel()
	ncutils.Log("all done")
	return nil
}

// SetupMQTT creates a connection to broker and return client
func SetupMQTT(cfg *config.ClientConfig) mqtt.Client {
	opts := mqtt.NewClientOptions()
	for _, server := range cfg.Node.NetworkSettings.DefaultServerAddrs {
		if server.Address != "" && server.IsLeader {
			// ncutils.Log(fmt.Sprintf("adding server (%s) to listen on network %s", server.Address, cfg.Node.Network))
			opts.AddBroker(server.Address + ":1883")
			break
		}
	}
	opts.SetDefaultPublishHandler(All)
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
			time.Sleep(2 * time.Second)
		}
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			ncutils.Log("unable to connect to broker, retrying ...")
			if time.Now().After(tperiod) {
				ncutils.Log("could not connect to broker, exiting " + cfg.Node.Network + " setup: " + token.Error().Error())
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
	client := SetupMQTT(&cfg)
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
	var found bool
	for _, server := range cfg.NetworkSettings.DefaultServerAddrs {
		if !server.IsLeader {
			continue
		}
		if server.Address != "" {
			if token := client.Subscribe("serverkeepalive/"+cfg.Node.Network, 0, mqtt.MessageHandler(ServerKeepAlive)); token.Wait() && token.Error() != nil {
				ncutils.Log(token.Error().Error())
				return
			}
			found = true
			if cfg.DebugOn {
				ncutils.Log("subscribed to server keepalives for server " + cfg.Node.Network)
			}
		}
	}
	if !found {
		ncutils.Log("leader not defined for network " + cfg.Node.Network)
	}
	defer client.Disconnect(250)
	go MonitorKeepalive(ctx, client, &cfg)
	go Checkin(ctx, &cfg, network)
	<-ctx.Done()
	ncutils.Log("shutting down daemon")
}

// All -- mqtt message hander for all ('#') topics
var All mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("default message handler -- received message but not handling")
	ncutils.Log("Topic: " + string(msg.Topic()))
	//ncutils.Log("Message: " + string(msg.Payload()))
}

// NodeUpdate -- mqtt message handler for /update/<NodeID> topic
func NodeUpdate(client mqtt.Client, msg mqtt.Message) {
	//potentiall blocking i/o so do this in a go routine
	go func() {
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
		insert(newNode.Network, lastNodeUpdate, string(data))
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
			if err := RemoveLocalInstance(&cfg, cfg.Network); err != nil {
				ncutils.PrintLog("error deleting local instance: "+err.Error(), 1)
				return
			}
			if token := client.Unsubscribe(fmt.Sprintf("update/%s/%s", newNode.Network, newNode.ID), fmt.Sprintf("peers/%s/%s", newNode.Network, newNode.ID)); token.Wait() && token.Error() != nil {
				ncutils.PrintLog("error unsubscribing during node deletion", 1)
			}
			return
		case models.NODE_UPDATE_KEY:
			if err := UpdateKeys(&cfg, client); err != nil {
				ncutils.PrintLog("err updating wireguard keys: "+err.Error(), 1)
			}
		case models.NODE_NOOP:
		default:
		}
		//Save new config
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
			err = wireguard.ApplyWGQuickConf(file, cfg.Node.Interface)
			if err != nil {
				ncutils.Log("error restarting wg after node update " + err.Error())
				return
			}
			time.Sleep(time.Second >> 1)
			if err = Resubscribe(client, &cfg); err != nil {
				ncutils.Log("error resubscribing after interface change " + err.Error())
				return
			}
			if newNode.DNSOn == "yes" {
				ncutils.Log("setting up DNS")
				for _, server := range cfg.Node.NetworkSettings.DefaultServerAddrs {
					if server.IsLeader {
						go setDNS(cfg.Node.Interface, cfg.Network, server.Address)
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
	}()
}

// UpdatePeers -- mqtt message handler for peers/<Network>/<NodeID> topic
func UpdatePeers(client mqtt.Client, msg mqtt.Message) {
	go func() {
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
		// see if cache hit, if so skip
		var currentMessage = read(peerUpdate.Network, lastPeerUpdate)
		if currentMessage == string(data) {
			ncutils.Log("cache hit")
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
		err = wireguard.SetPeers(cfg.Node.Interface, cfg.Node.PersistentKeepalive, peerUpdate.Peers)
		if err != nil {
			ncutils.Log("error syncing wg after peer update " + err.Error())
			return
		}
		ncutils.Log(fmt.Sprintf("received peer update on network, %s", cfg.Network))
	}()
}

// MonitorKeepalive - checks time last server keepalive received.  If more than 3+ minutes, notify and resubscribe
func MonitorKeepalive(ctx context.Context, client mqtt.Client, cfg *config.ClientConfig) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 150):
			var keepalivetime time.Time
			keepaliveval, ok := keepalive.Load(cfg.Node.Network)
			if ok {
				keepalivetime = keepaliveval.(time.Time)
			} else {
				ncutils.Log("unable to parse timestamp " + keepalivetime.String())
				continue
			}
			if time.Since(keepalivetime) > time.Second*120 { // more than 2+ minutes
				ncutils.Log("server keepalive not recieved recently, resubscribe to message queue")
				err := Resubscribe(client, cfg)
				if err != nil {
					ncutils.Log("closing " + err.Error())
				}
			}
		}
	}
}

// ServerKeepAlive -- handler to react to keepalive messages published by server
func ServerKeepAlive(client mqtt.Client, msg mqtt.Message) {
	keepalive.Store(parseNetworkFromTopic(msg.Topic()), time.Now())
}

// Resubscribe --- handles resubscribing if needed
func Resubscribe(client mqtt.Client, cfg *config.ClientConfig) error {
	if err := config.ModConfig(&cfg.Node); err == nil {
		ncutils.Log("resubbing on network " + cfg.Node.Network)
		client.Disconnect(250)
		client = SetupMQTT(cfg)
		if token := client.Subscribe(fmt.Sprintf("update/%s/%s", cfg.Node.Network, cfg.Node.ID), 0, NodeUpdate); token.Wait() && token.Error() != nil {
			ncutils.Log("error resubscribing to updates for " + cfg.Node.Network)
			return token.Error()
		}
		if cfg.DebugOn {
			ncutils.Log("subscribed to node updates for node " + cfg.Node.Name + " update/" + cfg.Node.ID)
		}
		if token := client.Subscribe(fmt.Sprintf("peers/%s/%s", cfg.Node.Network, cfg.Node.ID), 0, UpdatePeers); token.Wait() && token.Error() != nil {
			ncutils.Log("error resubscribing to peers for " + cfg.Node.Network)
			return token.Error()
		}
		var found bool
		for _, server := range cfg.NetworkSettings.DefaultServerAddrs {
			if !server.IsLeader {
				continue
			}
			if server.Address != "" {
				if token := client.Subscribe("serverkeepalive/"+cfg.Node.Network, 0, mqtt.MessageHandler(ServerKeepAlive)); token.Wait() && token.Error() != nil {
					ncutils.Log("error resubscribing to serverkeepalive for " + cfg.Node.Network)
					return token.Error()
				}
				found = true
				if cfg.DebugOn {
					ncutils.Log("subscribed to server keepalives for server " + cfg.Node.Network)
				}
			}
		}
		if !found {
			ncutils.Log("leader not defined for network " + cfg.Node.Network)
		}
		ncutils.Log("finished re subbing")
		return nil
	} else {
		ncutils.Log("could not mod config when re-subbing")
		return err
	}
}

// UpdateKeys -- updates private key and returns new publickey
func UpdateKeys(cfg *config.ClientConfig, client mqtt.Client) error {
	ncutils.Log("received message to update keys")
	//potentiall blocking i/o so do this in a go routine
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
	PublishNodeUpdate(cfg)
	if err := config.ModConfig(&cfg.Node); err != nil {
		ncutils.Log("error updating local config " + err.Error())
	}
	return nil
}

// Checkin  -- go routine that checks for public or local ip changes, publishes changes
//   if there are no updates, simply "pings" the server as a checkin
func Checkin(ctx context.Context, cfg *config.ClientConfig, network string) {
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
					PublishNodeUpdate(cfg)
				}
				intIP, err := getPrivateAddr()
				if err != nil {
					ncutils.PrintLog("error encountered checking ip addresses: "+err.Error(), 1)
				}
				if cfg.Node.LocalAddress != intIP && intIP != "" {
					ncutils.PrintLog("local Address has changed from "+cfg.Node.LocalAddress+" to "+intIP, 1)
					cfg.Node.LocalAddress = intIP
					PublishNodeUpdate(cfg)
				}
			} else {
				localIP, err := ncutils.GetLocalIP(cfg.Node.LocalRange)
				if err != nil {
					ncutils.PrintLog("error encountered checking ip addresses: "+err.Error(), 1)
				}
				if cfg.Node.Endpoint != localIP && localIP != "" {
					ncutils.PrintLog("endpoint has changed from "+cfg.Node.Endpoint+" to "+localIP, 1)
					cfg.Node.Endpoint = localIP
					PublishNodeUpdate(cfg)
				}
			}
			Hello(cfg, network)
			// ncutils.Log("Checkin complete")
		}
	}
}

// PublishNodeUpdates -- saves node and pushes changes to broker
func PublishNodeUpdate(cfg *config.ClientConfig) {
	if err := config.Write(cfg, cfg.Network); err != nil {
		ncutils.Log("error saving configuration: " + err.Error())
	}
	data, err := json.Marshal(cfg.Node)
	if err != nil {
		ncutils.Log("error marshling node update: " + err.Error())
	}
	if err = publish(cfg, fmt.Sprintf("update/%s", cfg.Node.ID), data); err != nil {
		ncutils.Log(fmt.Sprintf("error publishing endpoint update, %v", err))
	}
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

	client := SetupMQTT(cfg)
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

func setDNS(iface, network, address string) {
	var reachable bool
	for counter := 0; !reachable && counter < 5; counter++ {
		reachable = local.IsDNSReachable(address)
		time.Sleep(time.Second << 1)
	}
	if !reachable {
		ncutils.Log("not setting dns, server unreachable: " + address)
	} else if err := local.UpdateDNS(iface, network, address); err != nil {
		ncutils.Log("error applying dns" + err.Error())
	}
}
