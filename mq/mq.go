package mq

import (
	"encoding/json"
	"errors"
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
		id, err := GetID(msg.Topic())
		if err != nil {
			logger.Log(0, "error getting node.ID sent on ping topic ")
			return
		}
		node, err := logic.GetNodeByID(id)
		if err != nil {
			logger.Log(0, "mq-ping error getting node: "+err.Error())
			record, err := database.FetchRecord(database.NODES_TABLE_NAME, id)
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
		id, err := GetID(msg.Topic())
		if err != nil {
			logger.Log(0, "error getting node.ID sent on "+msg.Topic()+" "+err.Error())
		}
		node, err := logic.GetNodeByID(id)
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
		id, err := GetID(msg.Topic())
		logger.Log(0, "ipUpdate recieved from "+id)
		if err != nil {
			logger.Log(0, "error getting node.ID sent on update/ip topic ")
			return
		}
		node, err := logic.GetNodeByID(id)
		if err != nil {
			logger.Log(0, "invalid ID recieved on update/ip topic: "+err.Error())
			return
		}
		node.Endpoint = ip
		node.SetLastCheckIn()
		if err != UpdatePeers(client, node) {
			logger.Log(0, "error updating peers "+err.Error())
		}
	}()
}

func UpdatePeers(client mqtt.Client, node models.Node) error {
	peersToUpdate, err := logic.GetNetworkNodes(node.Network)
	if err != nil {
		logger.Log(0, "error retrieving peers to be updated "+err.Error())
		return err
	}
	for _, peerToUpdate := range peersToUpdate {
		peers, _, _, err := logic.GetServerPeers(&peerToUpdate)
		if err != nil {
			logger.Log(0, "error retrieving peers "+err.Error())
			return err
		}
		if peerToUpdate.ID == node.ID {
			continue
		}
		var peerUpdate models.PeerUpdate
		peerUpdate.Network = node.Network
		peerUpdate.Peers = peers
		data, err := json.Marshal(peerUpdate)
		if err != nil {
			logger.Log(0, "error marshaling peer update "+err.Error())
			return err
		}
		if token := client.Publish("/update/peers/"+peerToUpdate.ID, 0, false, data); token.Wait() && token.Error() != nil {
			logger.Log(0, "error sending peer updatte to no")
			return err
		}
	}
	return nil
}

var LocalAddressUpdate mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "LocalAddressUpdate Handler")
	go func() {
		logger.Log(0, "LocalAddressUpdate handler")
		id, err := GetID(msg.Topic())
		if err != nil {
			logger.Log(0, "error getting node.ID "+msg.Topic())
			return
		}
		node, err := logic.GetNodeByID(id)
		if err != nil {
			logger.Log(0, "error get node "+msg.Topic())
			return
		}
		node.LocalAddress = string(msg.Payload())
		node.SetLastCheckIn()
		if err := UpdatePeers(client, node); err != nil {
			logger.Log(0, "error updating peers "+err.Error())
		}
	}()
}

func GetID(topic string) (string, error) {
	parts := strings.Split(topic, "/")
	count := len(parts)
	if count == 1 {
		return "", errors.New("invalid topic")
	}
	//the last part of the topic will be the node.ID
	return parts[count-1], nil
}
