package mq

import (
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var DefaultHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "MQTT Message: Topic: "+string(msg.Topic())+" Message: "+string(msg.Payload()))
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
		if err := logic.UpdateNode(&node, &node) ; err != nil {
			logger.Log(0, "error updating node "+ err.Error())
		}
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
		if err := logic.UpdateNode(&node, &node) ; err != nil {
			logger.Log(0, "error updating node "+ err.Error())
		}
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
		if err := logic.UpdateNode(&node, &node) ; err != nil {
			logger.Log(0, "error updating node "+ err.Error())
		}
		if err != UpdatePeers(client, node) {
			logger.Log(0, "error updating peers "+err.Error())
		}
	}()
}

func UpdatePeers(client mqtt.Client, newnode models.Node) error {
	networkNodes, err := logic.GetNetworkNodes(newnode.Network)
	if err != nil {
		return err
	}
        keepalive, _ := time.ParseDuration(string(newnode.PersistentKeepalive)+"s")
        for _, node := range  networkNodes {
                var peers []wgtypes.PeerConfig
		var peerUpdate models.PeerUpdate
                for _, peer := range  networkNodes{
                        if peer.ID == node.ID {
                                //skip
                                continue
                        }
                        pubkey, err := wgtypes.ParseKey(peer.PublicKey)
                        if err != nil {
				return err
                        }
                        if node.Endpoint == peer.Endpoint {
                                if node.LocalAddress != peer.LocalAddress && peer.LocalAddress != "" {
                                        peer.Endpoint = peer.LocalAddress
                                }else {
                                        continue
                                }
                        }
                        endpoint := peer.Endpoint + ":" + strconv.Itoa(int(peer.ListenPort))
                        //fmt.Println("endpoint: ", endpoint, peer.Endpoint, peer.ListenPort)
                        address, err := net.ResolveUDPAddr("udp", endpoint)
                        if err != nil {
				return err
                        }
                        //calculate Allowed IPs.
                        var peerData wgtypes.PeerConfig
                        peerData = wgtypes.PeerConfig{
                                PublicKey: pubkey,
                                Endpoint: address,
                                PersistentKeepaliveInterval: &keepalive,
                                //AllowedIPs: allowedIPs
                        }
                        peers = append (peers, peerData)
                }
		peerUpdate.Network = node.Network
		peerUpdate.Peers = peers 
		data, err := json.Marshal(&peerUpdate)
		if err != nil {
			logger.Log(0, "error marshaling peer update "+err.Error())
			return err
		}
			if token := client.Publish("/update/peers/"+node.ID, 0, false, data); token.Wait() && token.Error() != nil {
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

func NewPeer(node models.Node) error {
	opts := mqtt.NewClientOptions()
	broker := servercfg.GetMessageQueueEndpoint()
	logger.Log(0, "broker: "+broker)
	opts.AddBroker(broker)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	
	if err := UpdatePeers(client, node); err != nil {
		return err
	}
	return nil
}
