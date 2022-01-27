package mq

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

const KEEPALIVE_TIMEOUT = 60 //timeout in seconds
const MQ_DISCONNECT = 250

// DefaultHandler default message queue handler - only called when GetDebug == true
var DefaultHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "MQTT Message: Topic: ", string(msg.Topic()), " Message: ", string(msg.Payload()))
}

// Ping message Handler -- handles ping topic from client nodes
var Ping mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "Ping Handler: ", msg.Topic())
	go func() {
		id, err := GetID(msg.Topic())
		if err != nil {
			logger.Log(0, "error getting node.ID sent on ping topic ")
			return
		}
		node, err := logic.GetNodeByID(id)
		if err != nil {
			logger.Log(0, "mq-ping error getting node: ", err.Error())
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
			logger.Log(0, "error updating node ", err.Error())
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
			logger.Log(1, "error getting node.ID sent on ", msg.Topic(), err.Error())
			return
		}
		logger.Log(1, "Update Node Handler", id)
		var newNode models.Node
		if err := json.Unmarshal(msg.Payload(), &newNode); err != nil {
			logger.Log(1, "error unmarshaling payload ", err.Error())
			return
		}
		currentNode, err := logic.GetNodeByID(newNode.ID)
		if err != nil {
			logger.Log(1, "error getting node ", newNode.ID, err.Error())
			return
		}
		if err := logic.UpdateNode(&currentNode, &newNode); err != nil {
			logger.Log(1, "error saving node", err.Error())
		}
		if logic.ShouldPeersUpdate(&currentNode, &newNode) {
			if err := PublishPeerUpdate(client, &newNode); err != nil {
				logger.Log(1, "error publishing peer update ", err.Error())
				return
			}
		}
		logger.Log(1, "no need to update peers")
	}()
}

// PublishPeerUpdate --- deterines and publishes a peer update to all the peers of a node
func PublishPeerUpdate(client mqtt.Client, newNode *models.Node) error {
	networkNodes, err := logic.GetNetworkNodes(newNode.Network)
	if err != nil {
		logger.Log(1, "err getting Network Nodes", err.Error())
		return err
	}
	for _, node := range networkNodes {
		peerUpdate, err := logic.GetPeerUpdate(&node)
		if err != nil {
			logger.Log(1, "error getting peer update for node ", node.ID, err.Error())
			continue
		}
		data, err := json.Marshal(&peerUpdate)
		if err != nil {
			logger.Log(2, "error marshaling peer update ", err.Error())
			return err
		}
		if token := client.Publish("update/peers/"+node.ID, 0, false, data); token.Wait() && token.Error() != nil {
			logger.Log(2, "error publishing peer update to peer ", node.ID, token.Error().Error())
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
	client := SetupMQTT()
	defer client.Disconnect(250)
	data, err := json.Marshal(node)
	if err != nil {
		logger.Log(2, "error marshalling node update ", err.Error())
		return err
	}
	if token := client.Publish("update/"+node.ID, 0, false, data); token.Wait() && token.Error() != nil {
		logger.Log(2, "error publishing peer update to peer ", node.ID, token.Error().Error())
		return err
	}
	return nil
}

// UpdatePeers -- publishes a peer update to all the peers of a node
func UpdatePeers(node *models.Node) error {
	client := SetupMQTT()
	defer client.Disconnect(250)
	if err := PublishPeerUpdate(client, node); err != nil {
		return err
	}
	return nil
}

// SetupMQTT creates a connection to broker and return client
func SetupMQTT() mqtt.Client {
	opts := mqtt.NewClientOptions()
	broker := servercfg.GetMessageQueueEndpoint()
	opts.AddBroker(broker)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	logger.Log(2, "connected to message queue", broker)
	return client
}

// Keepalive -- periodically pings all nodes to let them know server is still alive and doing well
func Keepalive(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * KEEPALIVE_TIMEOUT):
			client := SetupMQTT()
			networks, err := logic.GetNetworks()
			if err != nil {
				logger.Log(1, "error retrieving networks for keepalive", err.Error())
			}
			for _, network := range networks {
				var id string
				for _, servAddr := range network.DefaultServerAddrs {
					if servAddr.IsLeader {
						id = servAddr.ID
					}
				}
				if id != "" {
					logger.Log(0, "leader not defined for network", network.NetID)
					continue
				}
				if token := client.Publish("serverkeepalive/"+id, 0, false, servercfg.GetVersion()); token.Wait() && token.Error() != nil {
					logger.Log(1, "error publishing server keepalive for network", network.NetID, token.Error().Error())
				} else {
					logger.Log(2, "keepalive sent for network", network.NetID)
				}
				client.Disconnect(MQ_DISCONNECT)
			}
		}
	}
}
