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
	"github.com/gravitl/netmaker/servercfg"
)

// default message handler - only called in GetDebug == true
var DefaultHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "MQTT Message: Topic: "+string(msg.Topic())+" Message: "+string(msg.Payload()))
}

// Ping message Handler -- handles ping topic from client nodes
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
		if err := logic.UpdateNode(&node, &node); err != nil {
			logger.Log(0, "error updating node "+err.Error())
		}
		logger.Log(0, "ping processed")
		// --TODO --set client version once feature is implemented.
		//node.SetClientVersion(msg.Payload())
	}()
}

// UpdateNode  message Handler -- handles updates from client nodes
var UpdateNode mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	go func() {
		id, err := GetID(msg.Topic())
		if err != nil {
			logger.Log(1, "error getting node.ID sent on "+msg.Topic()+" "+err.Error())
			return
		}
		logger.Log(1, "Update Node Handler"+id)
		var newNode models.Node
		if err := json.Unmarshal(msg.Payload(), &newNode); err != nil {
			logger.Log(1, "error unmarshaling payload "+err.Error())
			return
		}
		currentNode, err := logic.GetNodeByID(newNode.ID)
		if err != nil {
			logger.Log(1, "error getting node "+newNode.ID+" "+err.Error())
			return
		}
		if err := logic.UpdateNode(&currentNode, &newNode); err != nil {
			logger.Log(1, "error saving node"+err.Error())
		}
		if logic.ShouldPeersUpdate(&currentNode, &newNode) {
			if err := PublishPeerUpdate(client, &newNode); err != nil {
				logger.Log(1, "error publishing peer update "+err.Error())
				return
			}
		}
	}()
}

// PublishPeerUpdate --- deterines and publishes a peer update to all the peers of a node
func PublishPeerUpdate(client mqtt.Client, newNode *models.Node) error {
	networkNodes, err := logic.GetNetworkNodes(newNode.Network)
	if err != nil {
		logger.Log(1, "err getting Network Nodes"+err.Error())
		return err
	}
	for _, node := range networkNodes {
		peerUpdate, err := logic.GetPeerUpdate(&node)
		if err != nil {
			logger.Log(1, "error getting peer update for node "+node.ID+" "+err.Error())
			continue
		}
		data, err := json.Marshal(&peerUpdate)
		if err != nil {
			logger.Log(2, "error marshaling peer update "+err.Error())
			return err
		}
		if token := client.Publish("/update/peers/"+node.ID, 0, false, data); token.Wait() && token.Error() != nil {
			logger.Log(2, "error publishing peer update to peer "+node.ID+" "+token.Error().Error())
			return err
		}
	}
	return nil
}

// GetID -- decodes a message queue topic and returns the embedded node.ID
func GetID(topic string) (string, error) {
	parts := strings.Split(topic, "/")
	count := len(parts)
	if count == 1 {
		return "", errors.New("invalid topic")
	}
	//the last part of the topic will be the node.ID
	return parts[count-1], nil
}

// UpdateNode -- publishes a node update
func NodeUpdate(node *models.Node) error {
	logger.Log(3, "publishing node update to "+node.Name)
	opts := mqtt.NewClientOptions()
	broker := servercfg.GetMessageQueueEndpoint()
	logger.Log(0, "broker: "+broker)
	opts.AddBroker(broker)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	data, err := json.Marshal(node)
	if err != nil {
		logger.Log(2, "error marshalling node update "+err.Error())
		return err
	}
	if token := client.Publish("/update/"+node.ID, 0, false, data); token.Wait() && token.Error() != nil {
		logger.Log(2, "error publishing peer update to peer "+node.ID+" "+token.Error().Error())
		return err
	}
	return nil
}

// NewPeer -- publishes a peer update to all the peers of a newNode
func NewPeer(node models.Node) error {
	opts := mqtt.NewClientOptions()
	broker := servercfg.GetMessageQueueEndpoint()
	logger.Log(0, "broker: "+broker)
	opts.AddBroker(broker)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	if err := PublishPeerUpdate(client, &node); err != nil {
		return err
	}
	return nil
}
