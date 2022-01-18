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
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

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
func SetupMQTT(cfg config.ClientConfig) mqtt.Client {
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
	client := SetupMQTT(cfg)
	if token := client.Subscribe("#", 0, nil); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	client.AddRoute("update/"+cfg.Node.ID, NodeUpdate)
	client.AddRoute("update/peers/"+cfg.Node.ID, UpdatePeers)
	//handle key updates in node update
	//client.AddRoute("update/keys/"+cfg.Node.ID, UpdateKeys)
	defer client.Disconnect(250)
	go Checkin(ctx, cfg, network)
	<-ctx.Done()
	ncutils.Log("shutting down daemon")
}

// All -- mqtt message hander for all ('#') topics
var All mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("Topic: " + string(msg.Topic()))
	ncutils.Log("Message: " + string(msg.Payload()))
}

// NodeUpdate -- mqtt message handler for /update/<NodeID> topic
var NodeUpdate mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("received message to update node " + string(msg.Payload()))
	//potentiall blocking i/o so do this in a go routine
	go func() {
		var newNode models.Node
		var cfg config.ClientConfig
		cfg.Network = newNode.Network
		cfg.ReadConfig()
		err := json.Unmarshal(msg.Payload(), &newNode)
		if err != nil {
			ncutils.Log("error unmarshalling node update data" + err.Error())
			return
		}
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
		case models.NODE_UPDATE_KEY:
			UpdateKeys(&cfg, client)
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
		if err := wireguard.UpdateWgInterface(cfg.Node.Interface, privateKey, nameserver, newNode); err != nil {
			ncutils.Log("error updating wireguard config " + err.Error())
			return
		}
		// path hardcoded for now... should be updated
		err = wireguard.ApplyWGQuickConf("/etc/netclient/config/" + cfg.Node.Interface + ".conf")
		if err != nil {
			ncutils.Log("error restarting wg after node update " + err.Error())
			return
		}
	}()
}

// UpdatePeers -- mqtt message handler for /update/peers/<NodeID> topic
var UpdatePeers mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("received message to update peers " + string(msg.Payload()))
	go func() {
		var peerUpdate models.PeerUpdate
		err := json.Unmarshal(msg.Payload(), &peerUpdate)
		if err != nil {
			ncutils.Log("error unmarshalling peer data")
			return
		}
		var cfg config.ClientConfig
		cfg.Network = peerUpdate.Network
		cfg.ReadConfig()
		err = wireguard.UpdateWgPeers(cfg.Node.Interface, peerUpdate.Peers)
		if err != nil {
			ncutils.Log("error updating wireguard peers" + err.Error())
			return
		}
		// path hardcoded for now... should be updated
		err = wireguard.ApplyWGQuickConf("/etc/netclient/config/" + cfg.Node.Interface + ".conf")
		if err != nil {
			ncutils.Log("error restarting wg after peer update " + err.Error())
			return
		}
	}()
}

// UpdateKeys -- updates private key and returns new publickey
func UpdateKeys(cfg *config.ClientConfig, client mqtt.Client) (*config.ClientConfig, error) {
	ncutils.Log("received message to update keys")
	//potentiall blocking i/o so do this in a go routine
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		ncutils.Log("error generating privatekey " + err.Error())
		return cfg, err
	}
	if err := wireguard.UpdatePrivateKey(cfg.Node.Interface, key.String()); err != nil {
		ncutils.Log("error updating wireguard key " + err.Error())
		return cfg, err
	}
	publicKey := key.PublicKey()
	if token := client.Publish("update/publickey/"+cfg.Node.ID, 0, false, publicKey.String()); token.Wait() && token.Error() != nil {
		ncutils.Log("error publishing publickey update " + token.Error().Error())
		client.Disconnect(250)
		return cfg, err
	}
	if err := config.ModConfig(&cfg.Node); err != nil {
		ncutils.Log("error updating local config " + err.Error())
	}
	return cfg, nil
}

// Checkin  -- go routine that checks for public or local ip changes, publishes changes
//   if there are no updates, simply "pings" the server as a checkin
func Checkin(ctx context.Context, cfg config.ClientConfig, network string) {
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
					UpdateEndpoint(cfg, network, extIP)
				}
				intIP, err := getPrivateAddr()
				if err != nil {
					ncutils.PrintLog("error encountered checking ip addresses: "+err.Error(), 1)
				}
				if cfg.Node.LocalAddress != intIP && intIP != "" {
					ncutils.PrintLog("local Address has changed from "+cfg.Node.LocalAddress+" to "+intIP, 1)
					UpdateLocalAddress(cfg, network, intIP)
				}
			} else {
				localIP, err := ncutils.GetLocalIP(cfg.Node.LocalRange)
				if err != nil {
					ncutils.PrintLog("error encountered checking ip addresses: "+err.Error(), 1)
				}
				if cfg.Node.Endpoint != localIP && localIP != "" {
					ncutils.PrintLog("endpoint has changed from "+cfg.Node.Endpoint+" to "+localIP, 1)
					UpdateEndpoint(cfg, network, localIP)
				}
			}
			Hello(cfg, network)
			ncutils.Log("Checkin complete")
		}
	}
}

// UpdateEndpoint -- publishes an endpoint update to broker
func UpdateEndpoint(cfg config.ClientConfig, network, ip string) {
	ncutils.Log("Updating endpoint")
	client := SetupMQTT(cfg)
	if token := client.Publish("update/ip/"+cfg.Node.ID, 0, false, ip); token.Wait() && token.Error() != nil {
		ncutils.Log("error publishing endpoint update " + token.Error().Error())
	}
	cfg.Node.Endpoint = ip
	if err := config.Write(&cfg, cfg.Network); err != nil {
		ncutils.Log("error updating local config " + err.Error())
	}
	client.Disconnect(250)
}

// UpdateLocalAddress -- publishes a local address update to broker
func UpdateLocalAddress(cfg config.ClientConfig, network, ip string) {
	ncutils.Log("Updating local address")
	client := SetupMQTT(cfg)
	if token := client.Publish("update/localaddress/"+cfg.Node.ID, 0, false, ip); token.Wait() && token.Error() != nil {
		ncutils.Log("error publishing local address update " + token.Error().Error())
	}
	cfg.Node.LocalAddress = ip
	ncutils.Log("updating local address in local config to: " + cfg.Node.LocalAddress)
	if err := config.Write(&cfg, cfg.Network); err != nil {
		ncutils.Log("error updating local config " + err.Error())
	}
	client.Disconnect(250)
}

// Hello -- ping the broker to let server know node is alive and doing fine
func Hello(cfg config.ClientConfig, network string) {
	client := SetupMQTT(cfg)
	ncutils.Log("sending ping " + cfg.Node.ID)
	if token := client.Publish("ping/"+cfg.Node.ID, 2, false, "hello world!"); token.Wait() && token.Error() != nil {
		ncutils.Log("error publishing ping " + token.Error().Error())
	}
	client.Disconnect(250)
}
