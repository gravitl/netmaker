package mq

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var DefaultHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "MQTT Message: Topic: "+string(msg.Topic())+" Message: "+string(msg.Payload()))
}

var Metrics mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "Metrics Handler")
}

var Ping mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "Ping Handler")
	//test code --- create a node if it doesn't exit for testing only
	createnode := models.Node{PublicKey: "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", Name: "testnode",
		Endpoint: "10.0.0.1", MacAddress: "01:02:03:04:05:06", Password: "password", Network: "skynet"}
	if _, err := logic.GetNode("01:02:03:04:05:06", "skynet"); err != nil {
		err := logic.CreateNode(&createnode)
		if err != nil {
			log.Println(err)
		}
	}
	//end of test code
	go func() {
		mac, net, err := GetMacNetwork(msg.Topic())
		if err != nil {
			logger.Log(0, "error getting node.ID sent on ping topic ")
			return
		}
		logger.Log(0, "ping recieved from "+mac+" on net "+net)
		node, err := logic.GetNodeByMacAddress(net, mac)
		if err != nil {
			logger.Log(0, "mq-ping error getting node: "+err.Error())
			record, err := database.FetchRecord(database.NODES_TABLE_NAME, mac+"###"+net)
			if err != nil {
				logger.Log(0, "error reading database ", err.Error())
				return
			}
			logger.Log(0, "record from database")
			logger.Log(0, record)
			return
		}
		node.SetLastCheckIn()
		// --TODO --set client version once feature is implemented.
		//node.SetClientVersion(msg.Payload())
	}()
}

var PublicKeyUpdate mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "PublicKey Handler")
	go func() {
		logger.Log(0, "public key update "+msg.Topic())
		key := string(msg.Payload())
		mac, network, err := GetMacNetwork(msg.Topic())
		if err != nil {
			logger.Log(0, "error getting node.ID sent on "+msg.Topic()+" "+err.Error())
		}
		node, err := logic.GetNode(mac, network)
		if err != nil {
			logger.Log(0, "error retrieving node "+msg.Topic()+" "+err.Error())
		}
		node.PublicKey = key
		node.SetLastCheckIn()
		UpdatePeers(&node, client)
	}()
}

var IPUpdate mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	go func() {
		ip := string(msg.Payload())
		logger.Log(0, "IPUpdate Handler")
		mac, network, err := GetMacNetwork(msg.Topic())
		logger.Log(0, "ipUpdate recieved from "+mac+" on net "+network)
		if err != nil {
			logger.Log(0, "error getting node.ID sent on update/ip topic ")
			return
		}
		node, err := logic.GetNode(mac, network)
		if err != nil {
			logger.Log(0, "invalid ID recieved on update/ip topic: "+err.Error())
			return
		}
		node.Endpoint = ip
		node.SetLastCheckIn()
		UpdatePeers(&node, client)
	}()
}

func UpdatePeers(node *models.Node, client mqtt.Client) {
	peersToUpdate, err := logic.GetPeers(node)
	if err != nil {
		logger.Log(0, "error retrieving peers")
		return
	}
	for _, peerToUpdate := range peersToUpdate {
		var peerUpdate models.PeerUpdate
		peerUpdate.Network = node.Network

		myPeers, err := logic.GetPeers(&peerToUpdate)
		if err != nil {
			logger.Log(0, "uable to get peers "+err.Error())
			continue
		}
		for i, myPeer := range myPeers {
			var allowedIPs []net.IPNet
			var allowedIP net.IPNet
			endpoint, err := net.ResolveUDPAddr("udp", myPeer.Address+":"+string(myPeer.ListenPort))
			if err != nil {
				logger.Log(0, "error setting endpoint for peer "+err.Error())
			}
			for _, ipString := range myPeer.AllowedIPs {
				_, ipNet, _ := net.ParseCIDR(ipString)
				allowedIP = *ipNet
				allowedIPs = append(allowedIPs, allowedIP)
			}
			key, err := wgtypes.ParseKey(myPeer.PublicKey)
			if err != nil {
				logger.Log(0, "err parsing publickey")
				continue
			}
			peerUpdate.Peers[i].PublicKey = key
			peerUpdate.Peers[i].Endpoint = endpoint
			peerUpdate.Peers[i].PersistentKeepaliveInterval = time.Duration(myPeer.PersistentKeepalive)
			peerUpdate.Peers[i].AllowedIPs = allowedIPs
			peerUpdate.Peers[i].ProtocolVersion = 0
		}
		//PublishPeerUpdate(my)
		data, err := json.Marshal(peerUpdate)
		if err != nil {
			logger.Log(0, "err marshalling data for peer update "+err.Error())
		}
		if token := client.Publish("update/peers/"+peerToUpdate.ID, 0, false, data); token.Wait() && token.Error() != nil {
			logger.Log(0, "error publishing peer update "+token.Error().Error())
		}
		client.Disconnect(250)
	}
}

var LocalAddressUpdate mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "LocalAddressUpdate Handler")
	go func() {
		logger.Log(0, "LocalAddressUpdate handler")
		mac, net, err := GetMacNetwork(msg.Topic())
		if err != nil {
			logger.Log(0, "error getting node.ID "+msg.Topic())
			return
		}
		node, err := logic.GetNode(mac, net)
		if err != nil {
			logger.Log(0, "error get node "+msg.Topic())
			return
		}
		node.LocalAddress = string(msg.Payload())
		node.SetLastCheckIn()
	}()
}

func GetMacNetwork(topic string) (string, string, error) {
	parts := strings.Split(topic, "/")
	count := len(parts)
	if count == 1 {
		return "", "", errors.New("invalid topic")
	}
	macnet := strings.Split(parts[count-1], "---")
	if len(macnet) != 2 {
		return "", "", errors.New("topic id not in mac---network format")
	}
	return macnet[0], macnet[1], nil
}

func GetID(topic string) (string, error) {
	parts := strings.Split(topic, "/")
	count := len(parts)
	if count == 1 {
		return "", errors.New("invalid topic")
	}
	macnet := strings.Split(parts[count-1], "---")
	if len(macnet) != 2 {
		return "", errors.New("topic id not in mac---network format")
	}
	return macnet[0] + "###" + macnet[1], nil
}
