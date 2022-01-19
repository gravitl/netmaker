package mq

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
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
		if err := logic.UpdateNode(&node, &node); err != nil {
			logger.Log(0, "error updating node "+err.Error())
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
		if err := logic.UpdateNode(&node, &node); err != nil {
			logger.Log(0, "error updating node "+err.Error())
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
		if err := logic.UpdateNode(&node, &node); err != nil {
			logger.Log(0, "error updating node "+err.Error())
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
	dualstack := false
	keepalive, _ := time.ParseDuration(string(newnode.PersistentKeepalive) + "s")
	defaultkeepalive, _ := time.ParseDuration("25s")
	for _, node := range networkNodes {
		var peers []wgtypes.PeerConfig
		var peerUpdate models.PeerUpdate
		var gateways []string

		for _, peer := range networkNodes {
			if peer.ID == node.ID {
				//skip
				continue
			}
			var allowedips []net.IPNet
			var peeraddr = net.IPNet{
				IP:   net.ParseIP(peer.Address),
				Mask: net.CIDRMask(32, 32),
			}
			//hasGateway := false
			pubkey, err := wgtypes.ParseKey(peer.PublicKey)
			if err != nil {
				return err
			}
			if node.Endpoint == peer.Endpoint {
				if node.LocalAddress != peer.LocalAddress && peer.LocalAddress != "" {
					peer.Endpoint = peer.LocalAddress
				} else {
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
			allowedips = append(allowedips, peeraddr)
			// handle manually set peers
			for _, allowedIp := range node.AllowedIPs {
				if _, ipnet, err := net.ParseCIDR(allowedIp); err == nil {
					nodeEndpointArr := strings.Split(node.Endpoint, ":")
					if !ipnet.Contains(net.IP(nodeEndpointArr[0])) && ipnet.IP.String() != node.Address { // don't need to add an allowed ip that already exists..
						allowedips = append(allowedips, *ipnet)
					}
				} else if appendip := net.ParseIP(allowedIp); appendip != nil && allowedIp != node.Address {
					ipnet := net.IPNet{
						IP:   net.ParseIP(allowedIp),
						Mask: net.CIDRMask(32, 32),
					}
					allowedips = append(allowedips, ipnet)
				}
			}
			// handle egress gateway peers
			if node.IsEgressGateway == "yes" {
				//hasGateway = true
				ranges := node.EgressGatewayRanges
				for _, iprange := range ranges { // go through each cidr for egress gateway
					_, ipnet, err := net.ParseCIDR(iprange) // confirming it's valid cidr
					if err != nil {
						ncutils.PrintLog("could not parse gateway IP range. Not adding "+iprange, 1)
						continue // if can't parse CIDR
					}
					nodeEndpointArr := strings.Split(node.Endpoint, ":") // getting the public ip of node
					if ipnet.Contains(net.ParseIP(nodeEndpointArr[0])) { // ensuring egress gateway range does not contain public ip of node
						ncutils.PrintLog("egress IP range of "+iprange+" overlaps with "+node.Endpoint+", omitting", 2)
						continue // skip adding egress range if overlaps with node's ip
					}
					if ipnet.Contains(net.ParseIP(node.LocalAddress)) { // ensuring egress gateway range does not contain public ip of node
						ncutils.PrintLog("egress IP range of "+iprange+" overlaps with "+node.LocalAddress+", omitting", 2)
						continue // skip adding egress range if overlaps with node's local ip
					}
					gateways = append(gateways, iprange)
					if err != nil {
						log.Println("ERROR ENCOUNTERED SETTING GATEWAY")
					} else {
						allowedips = append(allowedips, *ipnet)
					}
				}
			}
			var peerData wgtypes.PeerConfig
			if node.Address6 != "" && dualstack {
				var addr6 = net.IPNet{
					IP:   net.ParseIP(node.Address6),
					Mask: net.CIDRMask(128, 128),
				}
				allowedips = append(allowedips, addr6)
			}
			if node.IsServer == "yes" && !(node.IsServer == "yes") {
				peerData = wgtypes.PeerConfig{
					PublicKey:                   pubkey,
					PersistentKeepaliveInterval: &defaultkeepalive,
					ReplaceAllowedIPs:           true,
					AllowedIPs:                  allowedips,
				}
			} else if keepalive != 0 {
				peerData = wgtypes.PeerConfig{
					PublicKey:                   pubkey,
					PersistentKeepaliveInterval: &defaultkeepalive,
					//Endpoint: &net.UDPAddr{
					//	IP:   net.ParseIP(node.Endpoint),
					//	Port: int(node.ListenPort),
					//},
					Endpoint:          address,
					ReplaceAllowedIPs: true,
					AllowedIPs:        allowedips,
				}
			} else {
				peerData = wgtypes.PeerConfig{
					PublicKey: pubkey,
					//Endpoint: &net.UDPAddr{
					//	IP:   net.ParseIP(node.Endpoint),
					//	Port: int(node.ListenPort),
					//},
					Endpoint:          address,
					ReplaceAllowedIPs: true,
					AllowedIPs:        allowedips,
				}
			}
			//peerData = wgtypes.PeerConfig{
			//	PublicKey:                   pubkey,
			//	Endpoint:                    address,
			//	PersistentKeepaliveInterval: &keepalive,
			//AllowedIPs: allowedIPs
			//}
			peers = append(peers, peerData)
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
		if err := logic.UpdateNode(&node, &node); err != nil {
			logger.Log(0, "error updating node "+err.Error())
		}
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
