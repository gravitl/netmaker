package functions

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-ping/ping"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl"
)

func Daemon() error {
	networks, err := ncutils.GetSystemNetworks()
	if err != nil {
		return err
	}
	for _, network := range networks {
		go Netclient(network)
	}
	for {
	}
	return nil
}

func Netclient(network string) {
	var cfg config.ClientConfig
	cfg.Network = network
	cfg.ReadConfig()
	ncutils.Log("daemon started for network:" + network)
	//setup MQTT
	opts := mqtt.NewClientOptions()
	ncutils.Log("setting broker to " + cfg.Server.CoreDNSAddr + ":1883")
	opts.AddBroker(cfg.Server.CoreDNSAddr + ":1883")
	opts.SetDefaultPublishHandler(All)
	opts.SetClientID("netclient-mqtt")
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	if token := client.Subscribe("#", 0, nil); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	client.AddRoute("update/"+network+"/"+cfg.Node.MacAddress, NodeUpdate)
	client.AddRoute("update/"+network+"/peers", UpdatePeers)
	client.AddRoute("update/"+network+"/keys", UpdateKeys)
	client.AddRoute("update/"+network+"/keys/"+cfg.Node.MacAddress, UpdateKeys)
	defer client.Disconnect(250)
	go Checkin(client, network)
	//go Metrics(client, network)
	//go Connectivity(client, network)
	for {
	}
}

var All mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("Topic: " + string(msg.Topic()))
	ncutils.Log("Message: " + string(msg.Payload()))
}

var NodeUpdate mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("received message to update node " + string(msg.Payload()))
}

var UpdatePeers mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("received message to update peers " + string(msg.Payload()))
}

var UpdateKeys mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("received message to update keys " + string(msg.Payload()))
}

func Checkin(client mqtt.Client, network string) {
	var cfg config.ClientConfig
	cfg.Network = network
	cfg.ReadConfig()
	for {
		time.Sleep(time.Duration(cfg.Node.NetworkSettings.DefaultCheckInInterval) * time.Second)
		ncutils.Log("Checkin running")
		if cfg.Node.Roaming == "yes" && cfg.Node.IsStatic != "yes" {
			extIP, err := ncutils.GetPublicIP()
			if err != nil {
				ncutils.PrintLog("error encountered checking ip addresses: "+err.Error(), 1)
			}
			if cfg.Node.Endpoint != extIP && extIP != "" {
				ncutils.PrintLog("endpoint has changed from "+cfg.Node.Endpoint+" to "+extIP, 1)
				UpdateEndpoint(client, network, extIP)
			}
			intIP, err := getPrivateAddr()
			if err != nil {
				ncutils.PrintLog("error encountered checking ip addresses: "+err.Error(), 1)
			}
			if cfg.Node.LocalAddress != intIP && intIP != "" {
				ncutils.PrintLog("local Address has changed from "+cfg.Node.LocalAddress+" to "+intIP, 1)
				UpdateLocalAddress(client, network, intIP)
			}
		} else {
			localIP, err := ncutils.GetLocalIP(cfg.Node.LocalRange)
			if err != nil {
				ncutils.PrintLog("error encountered checking ip addresses: "+err.Error(), 1)
			}
			if cfg.Node.Endpoint != localIP && localIP != "" {
				ncutils.PrintLog("endpoint has changed from "+cfg.Node.Endpoint+" to "+localIP, 1)
				UpdateEndpoint(client, network, localIP)
			}
		}
		Ping(client, network)
	}
}

func Ping(client mqtt.Client, network string) {
	var cfg config.ClientConfig
	cfg.Network = network
	cfg.ReadConfig()
	if token := client.Publish("ping/"+network+"/"+cfg.Node.ID, 0, false, []byte("ping")); token.Wait() && token.Error() != nil {
		ncutils.Log("error publishing ping " + token.Error().Error())
	}
}

func Metrics(client mqtt.Client, network string) {
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	var cfg config.ClientConfig
	cfg.Network = network
	cfg.ReadConfig()
	for {
		time.Sleep(time.Second * 60)
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
		if token := client.Publish("metrics/"+network+"/"+cfg.Node.ID, 1, false, bytes); token.Wait() && token.Error() != nil {
			ncutils.Log("error publishing metrics " + token.Error().Error())
			break
		}
		wg.Close()
	}
}

type PingStat struct {
	Name      string
	Reachable bool
}

func Connectivity(client mqtt.Client, network string) {
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	var cfg config.ClientConfig
	cfg.Network = network
	cfg.ReadConfig()
	for {
		time.Sleep(time.Duration(cfg.NetworkSettings.DefaultCheckInInterval) * time.Second)
		ncutils.Log("Connectivity running")
		var pingStats []PingStat
		peers, err := ncutils.GetPeers(cfg.Node.Interface)
		if err != nil {
			ncutils.Log("error retriving peers " + err.Error())
			break
		}
		for _, peer := range peers {
			var pingStat PingStat
			pingStat.Name = peer.PublicKey.String()
			pingStat.Reachable = true
			ip := peer.Endpoint.IP.String()
			fmt.Println("----------", peer.Endpoint.IP, ip)
			pinger, err := ping.NewPinger(ip)
			if err != nil {
				ncutils.Log("error creating pinger " + err.Error())
				break
			}
			pinger.Timeout = 2 * time.Second
			pinger.Run()
			stats := pinger.Statistics()
			if stats.PacketLoss == 100 {
				pingStat.Reachable = false
			}
			pingStats = append(pingStats, pingStat)
		}
		bytes, err := json.Marshal(pingStats)
		if err != nil {
			ncutils.Log("error marshaling stats" + err.Error())
			break
		}
		if token := client.Publish("connectivity/"+network+"/"+cfg.Node.ID, 1, false, bytes); token.Wait() && token.Error() != nil {
			ncutils.Log("error publishing ping stats " + token.Error().Error())
			break
		}
	}
}

func UpdateEndpoint(client mqtt.Client, network, ip string) {
	ncutils.Log("Updating endpoint")
}

func UpdateLocalAddress(client mqtt.Client, network, ip string) {
	ncutils.Log("Updating local address")
}
