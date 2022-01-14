package mq

import (
	"errors"
	"fmt"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

var DefaultHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "MQTT Message: Topic: "+string(msg.Topic())+" Message: "+string(msg.Payload()))
}

var Metrics mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "Metrics Handler")
	//TODOD -- handle metrics data ---- store to database?
}

var Ping mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "Ping Handler: "+msg.Topic())
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
		logger.Log(0, "ping processed")
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
		if err := UpdatePeers(client, node); err != nil {
			logger.Log(0, "error updating peers "+err.Error())
		}
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
		if err := UpdatePeers(client, node); err != nil {
			logger.Log(0, "error updating peers "+err.Error())
		}
	}()
}

func UpdatePeers(client mqtt.Client, node models.Node) error {
	var peerUpdate models.PeerUpdate
	peerUpdate.Network = node.Network

	nodes, err := logic.GetNetworkNodes(node.Network)
	if err != nil {
		return fmt.Errorf("unable to get network nodes %v: ", err)
	}
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	for _, peer := range nodes {
		//don't need to update the initiatiing client
		if peer.ID == node.ID {
			continue
		}
		peerUpdate.Nodes = append(peerUpdate.Nodes, peer)
		peerUpdate.ExtPeers, err = logic.GetExtPeersList(node.MacAddress, node.Network)
		if err != nil {
			logger.Log(0)
		}
		if token := client.Publish("update/peers/"+peer.ID, 0, false, nodes); token.Wait() && token.Error() != nil {
			logger.Log(0, "error publishing peer update "+peer.ID+" "+token.Error().Error())
		}
	}

	return nil
}

func UpdateLocalPeers(client mqtt.Client, node models.Node) error {
	nodes, err := logic.GetNetworkNodes(node.Network)
	if err != nil {
		return fmt.Errorf("unable to get network nodes %v: ", err)
	}
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	for _, peer := range nodes {
		//don't need to update the initiatiing client
		if peer.ID == node.ID {
			continue
		}
		//if peer.Endpoint is on same lan as node.LocalAddress
		//if TODO{
		//continue
		//}
		if token := client.Publish("update/peers/"+peer.ID, 0, false, nodes); token.Wait() && token.Error() != nil {
			logger.Log(0, "error publishing peer update "+peer.ID+" "+token.Error().Error())
		}
	}
	return nil
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
		if err := UpdateLocalPeers(client, node); err != nil {
			logger.Log(0, "error updating peers "+err.Error())
		}
	}()
}

func GetMacNetwork(topic string) (string, string, error) {
	parts := strings.Split(topic, "/")
	count := len(parts)
	if count == 1 {
		return "", "", errors.New("invalid topic")
	}
	macnet := strings.Split(parts[count-1], "-")
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
	macnet := strings.Split(parts[count-1], "-")
	if len(macnet) != 2 {
		return "", errors.New("topic id not in mac---network format")
	}
	return macnet[0] + "###" + macnet[1], nil
}
