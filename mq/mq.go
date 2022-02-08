package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
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
		_, decryptErr := decryptMsg(&node, msg.Payload())
		if decryptErr != nil {
			logger.Log(0, "error updating node ", node.ID, err.Error())
			return
		}
		node.SetLastCheckIn()
		if err := logic.UpdateNode(&node, &node); err != nil {
			logger.Log(0, "error updating node ", err.Error())
		}
		logger.Log(3, "ping processed for node", node.ID)
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
		currentNode, err := logic.GetNodeByID(id)
		if err != nil {
			logger.Log(1, "error getting node ", id, err.Error())
			return
		}
		decrypted, decryptErr := decryptMsg(&currentNode, msg.Payload())
		if decryptErr != nil {
			logger.Log(1, "failed to decrypt message for node ", id, decryptErr.Error())
			return
		}
		logger.Log(1, "Update Node Handler", id)
		var newNode models.Node
		if err := json.Unmarshal(decrypted, &newNode); err != nil {
			logger.Log(1, "error unmarshaling payload ", err.Error())
			return
		}
		if err := logic.UpdateNode(&currentNode, &newNode); err != nil {
			logger.Log(1, "error saving node", err.Error())
		}
		if err := PublishPeerUpdate(&newNode); err != nil {
			logger.Log(1, "error publishing peer update ", err.Error())
			return
		}
		logger.Log(1, "no need to update peers")
	}()
}

// PublishPeerUpdate --- deterines and publishes a peer update to all the peers of a node
func PublishPeerUpdate(newNode *models.Node) error {
	if !servercfg.IsMessageQueueBackend() {
		return nil
	}
	networkNodes, err := logic.GetNetworkNodes(newNode.Network)
	if err != nil {
		logger.Log(1, "err getting Network Nodes", err.Error())
		return err
	}
	for _, node := range networkNodes {

		if node.IsServer == "yes" || node.ID == newNode.ID {
			log.Println("skipping update on " + node.Name + " : " + node.ID)
			continue
		}
		log.Println("running update on " + node.Name + " : " + node.ID)
		peerUpdate, err := logic.GetPeerUpdate(&node)
		if err != nil {
			logger.Log(1, "error getting peer update for node", node.ID, err.Error())
			continue
		}
		data, err := json.Marshal(&peerUpdate)
		if err != nil {
			logger.Log(2, "error marshaling peer update for node", node.ID, err.Error())
			continue
		}
		if err = publish(&node, fmt.Sprintf("peers/%s/%s", node.Network, node.ID), data); err != nil {
			logger.Log(1, "failed to publish peer update for node", node.ID)
		} else {
			logger.Log(1, fmt.Sprintf("sent peer update for node %s on network: %s ", node.Name, node.Network))
		}
	}
	return nil
}

// PublishPeerUpdate --- deterines and publishes a peer update to all the peers of a node
func PublishExtPeerUpdate(node *models.Node) error {
	var err error
	if logic.IsLocalServer(node) {
		if err = logic.ServerUpdate(node, false); err != nil {
			logger.Log(1, "server node:", node.ID, "failed to update peers with ext clients")
			return err
		} else {
			return nil
		}
	}
	if !servercfg.IsMessageQueueBackend() {
		return nil
	}
	peerUpdate, err := logic.GetPeerUpdate(node)
	if err != nil {
		return err
	}
	data, err := json.Marshal(&peerUpdate)
	if err != nil {
		return err
	}
	return publish(node, fmt.Sprintf("peers/%s/%s", node.Network, node.ID), data)
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

// NodeUpdate -- publishes a node update
func NodeUpdate(node *models.Node) error {
	if !servercfg.IsMessageQueueBackend() {
		return nil
	}
	logger.Log(3, "publishing node update to "+node.Name)
	data, err := json.Marshal(node)
	if err != nil {
		logger.Log(2, "error marshalling node update ", err.Error())
		return err
	}
	if err = publish(node, fmt.Sprintf("update/%s/%s", node.Network, node.ID), data); err != nil {
		logger.Log(2, "error publishing node update to peer ", node.ID, err.Error())
		return err
	}
	return nil
}

// SetupMQTT creates a connection to broker and return client
func SetupMQTT() mqtt.Client {
	opts := mqtt.NewClientOptions()
	broker := servercfg.GetMessageQueueEndpoint()
	opts.AddBroker(broker)
	id := ncutils.MakeRandomString(23)
	opts.ClientID = id
	client := mqtt.NewClient(opts)
	tperiod := time.Now().Add(10 * time.Second)
	for {
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			logger.Log(2, "unable to connect to broker, retrying ...")
			if time.Now().After(tperiod) {
				log.Fatal(0, "could not connect to broker, exiting ...", token.Error())
			}
		} else {
			break
		}
		time.Sleep(2 * time.Second)
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
				serverNode, errN := logic.GetNetworkServerLeader(network.NetID)
				if errN == nil {
					serverNode.SetLastCheckIn()
					logic.UpdateNode(&serverNode, &serverNode)
					if network.DefaultUDPHolePunch == "yes" {
						logic.ShouldPublishPeerPorts(&serverNode)
					}
					err = PublishPeerUpdate(&serverNode)
					if err != nil {
						logger.Log(1, "error publishing udp port updates for network", network.NetID)
						logger.Log(1, errN.Error())
					}
				} else {
					logger.Log(1, "unable to retrieve leader for network ", network.NetID)
					logger.Log(1, errN.Error())
					continue
				}
				if serverNode.Address == "" {
					logger.Log(1, "leader not defined for network ", network.NetID)
					continue
				}
				publishServerKeepalive(client, &network)
				err = serverctl.SyncServerNetwork(network.NetID)
				if err != nil {
					logger.Log(1, "error syncing server network", err.Error())
				}
			}
			client.Disconnect(MQ_DISCONNECT)
		}
	}
}

func publishServerKeepalive(client mqtt.Client, network *models.Network) {
	nodes, err := logic.GetNetworkNodes(network.NetID)
	if err != nil {
		return
	}
	for _, node := range nodes {
		if token := client.Publish(fmt.Sprintf("serverkeepalive/%s/%s", network.NetID, node.ID), 0, false, servercfg.GetVersion()); token.Wait() && token.Error() != nil {
			logger.Log(1, "error publishing server keepalive for network", network.NetID, token.Error().Error())
		} else {
			logger.Log(2, "keepalive sent for network/node", network.NetID, node.ID)
		}
	}
}
