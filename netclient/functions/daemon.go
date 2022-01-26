package functions

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// ServerKeepalive  - stores time of last server keepalive message
var KeepaliveReceived time.Time

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
	ncutils.Log("setting broker to " + cfg.Server.CoreDNSAddr + ":1883")
	opts.AddBroker(cfg.Server.CoreDNSAddr + ":1883")
	opts.SetDefaultPublishHandler(All)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	return client
}

// MessageQueue sets up Message Queue and subsribes/publishes updates to/from server
func MessageQueue(ctx context.Context, network string) {
	ncutils.Log("netclient go routine started for " + network)
	var cfg config.ClientConfig
	cfg.Network = network
	cfg.ReadConfig()
	ncutils.Log("daemon started for network:" + network)
	KeepaliveReceived = time.Now()
	client := SetupMQTT(&cfg)
	if cfg.DebugOn {
		if token := client.Subscribe("#", 0, nil); token.Wait() && token.Error() != nil {
			log.Fatal(token.Error())
		}
		ncutils.Log("subscribed to all topics for debugging purposes")
	}
	if token := client.Subscribe("update/"+cfg.Node.ID, 0, NodeUpdate); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	if cfg.DebugOn {
		ncutils.Log("subscribed to node updates for node " + cfg.Node.Name + " update/" + cfg.Node.ID)
	}
	if token := client.Subscribe("update/peers/"+cfg.Node.ID, 0, UpdatePeers); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	if cfg.DebugOn {
		ncutils.Log("subscribed to node updates for node " + cfg.Node.Name + " update/peers/" + cfg.Node.ID)
	}
	if token := client.Subscribe("serverkeepalive/"+cfg.Node.ID, 0, mqtt.MessageHandler(ServerKeepAlive)); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	if cfg.DebugOn {
		ncutils.Log("subscribed to server keepalives")
	}
	defer client.Disconnect(250)
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
var NodeUpdate mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("received message to update node " + string(msg.Payload()))
	//potentiall blocking i/o so do this in a go routine
	go func() {
		var newNode models.Node
		var cfg config.ClientConfig
		err := json.Unmarshal(msg.Payload(), &newNode)
		if err != nil {
			ncutils.Log("error unmarshalling node update data" + err.Error())
			return
		}
		cfg.Network = newNode.Network
		cfg.ReadConfig()
		//check if interface name has changed if so delete.
		if cfg.Node.Interface != newNode.Interface {
			if err = wireguard.RemoveConf(cfg.Node.Interface, true); err != nil {
				ncutils.PrintLog("could not delete old interface "+cfg.Node.Interface+": "+err.Error(), 1)
			}
		}
		newNode.PullChanges = "no"
		//ensure that OS never changes
		newNode.OS = runtime.GOOS
		cfg.Node = newNode
		switch newNode.Action {
		case models.NODE_DELETE:
			if err := RemoveLocalInstance(&cfg, cfg.Network); err != nil {
				ncutils.PrintLog("error deleting local instance: "+err.Error(), 1)
				return
			}
			if token := client.Unsubscribe("update/"+newNode.ID, "update/peers/"+newNode.ID); token.Wait() && token.Error() != nil {
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
		ncutils.Log("applyWGQuickConf to " + file)
		err = wireguard.ApplyWGQuickConf(file)
		if err != nil {
			ncutils.Log("error restarting wg after node update " + err.Error())
			return
		}
		//deal with DNS
		if newNode.DNSOn == "yes" {
			ncutils.Log("setting up DNS")
			if err = local.UpdateDNS(cfg.Node.Interface, cfg.Network, cfg.Server.CoreDNSAddr); err != nil {
				ncutils.Log("error applying dns" + err.Error())
			}
		} else {
			ncutils.Log("settng DNS off")
			_, err := ncutils.RunCmd("/usr/bin/resolvectl revert "+cfg.Node.Interface, true)
			if err != nil {
				ncutils.Log("error applying dns" + err.Error())
			}
		}
	}()
}

// UpdatePeers -- mqtt message handler for /update/peers/<NodeID> topic
var UpdatePeers mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	go func() {
		var peerUpdate models.PeerUpdate
		err := json.Unmarshal(msg.Payload(), &peerUpdate)
		if err != nil {
			ncutils.Log("error unmarshalling peer data")
			return
		}
		ncutils.Log("update peer handler")
		var cfg config.ClientConfig
		cfg.Network = peerUpdate.Network
		cfg.ReadConfig()
		file := ncutils.GetNetclientPathSpecific() + cfg.Node.Interface + ".conf"
		err = wireguard.UpdateWgPeers(file, peerUpdate.Peers)
		if err != nil {
			ncutils.Log("error updating wireguard peers" + err.Error())
			return
		}
		ncutils.Log("applyWGQuickConf to " + file)
		err = wireguard.ApplyWGQuickConf(file)
		if err != nil {
			ncutils.Log("error restarting wg after peer update " + err.Error())
			return
		}
	}()
}

// ServerKeepAlive -- handler to react to keepalive messages published by server
func ServerKeepAlive(client mqtt.Client, msg mqtt.Message) {
	if time.Now().Sub(KeepaliveReceived) < time.Second*200 { // more than 3+ minutes

		KeepaliveReceived = time.Now()
		return
	}
	ncutils.Log("server keepalive not recieved in last 3 minutes")
	///do other stuff
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
			ncutils.Log("Checkin running")
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
			ncutils.Log("Checkin complete")
		}
	}
}

// PublishNodeUpdates -- saves node and pushes changes to broker
func PublishNodeUpdate(cfg *config.ClientConfig) {
	if err := config.Write(cfg, cfg.Network); err != nil {
		ncutils.Log("error saving configuration" + err.Error())
	}
	client := SetupMQTT(cfg)
	data, err := json.Marshal(cfg.Node)
	if err != nil {
		ncutils.Log("error marshling node update " + err.Error())
	}
	if token := client.Publish("update/"+cfg.Node.ID, 0, false, data); token.Wait() && token.Error() != nil {
		ncutils.Log("error publishing endpoint update " + token.Error().Error())
	}
	client.Disconnect(250)
}

// Hello -- ping the broker to let server know node is alive and doing fine
func Hello(cfg *config.ClientConfig, network string) {
	client := SetupMQTT(cfg)
	if token := client.Publish("ping/"+cfg.Node.ID, 2, false, "hello world!"); token.Wait() && token.Error() != nil {
		ncutils.Log("error publishing ping " + token.Error().Error())
	}
	client.Disconnect(250)
}
