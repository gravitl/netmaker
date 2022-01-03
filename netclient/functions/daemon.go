package functions

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl"
)

//Daemon runs netclient daemon from command line
func Daemon() error {
	ctx, cancel := context.WithCancel(context.Background())
	networks, err := ncutils.GetSystemNetworks()
	if err != nil {
		return err
	}
	for _, network := range networks {
		go Netclient(ctx, network)
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	<-quit
	cancel()
	ncutils.Log("all done")
	return nil
}

//SetupMQTT creates a connection to broker and return client
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

//Netclient sets up Message Queue and subsribes/publishes updates to/from server
func Netclient(ctx context.Context, network string) {
	ncutils.Log("netclient go routine started for " + network)
	var cfg config.ClientConfig
	cfg.Network = network
	cfg.ReadConfig()
	//fix NodeID to remove ### so NodeID can be used as message topic
	//remove with GRA-73
	cfg.Node.ID = strings.ReplaceAll(cfg.Node.ID, "###", "-")
	ncutils.Log("daemon started for network:" + network)
	client := SetupMQTT(cfg)
	if token := client.Subscribe("#", 0, nil); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	client.AddRoute("update/"+cfg.Node.ID, NodeUpdate)
	client.AddRoute("update/peers/"+cfg.Node.ID, UpdatePeers)
	client.AddRoute("update/keys/"+cfg.Node.ID, UpdateKeys)
	defer client.Disconnect(250)
	go Checkin(ctx, cfg, network)
	go Metrics(ctx, cfg, network)
	<-ctx.Done()
	ncutils.Log("shutting down daemon")
	return
	ncutils.Log("netclient go routine ended for " + network)
}

//All -- mqtt message hander for all ('#') topics
var All mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("Topic: " + string(msg.Topic()))
	ncutils.Log("Message: " + string(msg.Payload()))
}

//NodeUpdate -- mqtt message handler for /update/<NodeID> topic
var NodeUpdate mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("received message to update node " + string(msg.Payload()))
}

//UpdatePeers -- mqtt message handler for /update/peers/<NodeID> topic
var UpdatePeers mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("received message to update peers " + string(msg.Payload()))
}

//UpdateKeys -- mqtt message handler for /update/keys/<NodeID> topic
var UpdateKeys mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("received message to update keys " + string(msg.Payload()))
}

//Checkin  -- go routine that checks for public or local ip changes, publishes changes
//   if there are no updates, simply "pings" the server as a checkin
func Checkin(ctx context.Context, cfg config.ClientConfig, network string) {
	for {
		select {
		case <-ctx.Done():
			ncutils.Log("Checkin cancelled")
			return
			//delay should be configuraable -> use cfg.Node.NetworkSettings.DefaultCheckInInterval ??
		case <-time.After(time.Second * 10):
			ncutils.Log("Checkin running")
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

//UpdateEndpoint -- publishes an endpoint update to broker
func UpdateEndpoint(cfg config.ClientConfig, network, ip string) {
	ncutils.Log("Updating endpoint")
	client := SetupMQTT(cfg)
	if token := client.Publish("update/ip/"+cfg.Node.ID, 0, false, ip); token.Wait() && token.Error() != nil {
		ncutils.Log("error publishing endpoint update " + token.Error().Error())
	}
	client.Disconnect(250)
}

//UpdateLocalAddress -- publishes a local address update to broker
func UpdateLocalAddress(cfg config.ClientConfig, network, ip string) {
	ncutils.Log("Updating local address")
	client := SetupMQTT(cfg)
	if token := client.Publish("update/localaddress/"+cfg.Node.ID, 0, false, ip); token.Wait() && token.Error() != nil {
		ncutils.Log("error publishing local address update " + token.Error().Error())
	}
	client.Disconnect(250)
}

//Hello -- ping the broker to let server know node is alive and doing fine
func Hello(cfg config.ClientConfig, network string) {
	client := SetupMQTT(cfg)
	if token := client.Publish("ping/"+cfg.Node.ID, 0, false, "hello world!"); token.Wait() && token.Error() != nil {
		ncutils.Log("error publishing ping " + token.Error().Error())
	}
	client.Disconnect(250)
}

//Metics --  go routine that collects wireguard metrics and publishes to broker
func Metrics(ctx context.Context, cfg config.ClientConfig, network string) {
	for {
		select {
		case <-ctx.Done():
			ncutils.Log("Metrics collection cancelled")
			return
			//delay should be configuraable -> use cfg.Node.NetworkSettings.DefaultCheckInInterval ??
		case <-time.After(time.Second * 60):
			ncutils.Log("Metrics collection running")
			ncutils.Log("Metrics running")
			wg, err := wgctrl.New()
			if err != nil {
				ncutils.Log("error getting devices " + err.Error())
				break
			}
			device, err := wg.Device(cfg.Node.Interface)
			if err != nil {
				ncutils.Log("error readind wg device " + err.Error())
				break
			}
			bytes, err := json.Marshal(device.Peers)
			if err != nil {
				ncutils.Log("error marshaling peers " + err.Error())
				break
			}
			client := SetupMQTT(cfg)
			if token := client.Publish("metrics/"+cfg.Node.ID, 1, false, bytes); token.Wait() && token.Error() != nil {
				ncutils.Log("error publishing metrics " + token.Error().Error())
			}
			wg.Close()
			client.Disconnect(250)
			ncutils.Log("metrics collection complete")
		}
	}
}
