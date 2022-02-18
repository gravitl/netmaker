package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
)

// All -- mqtt message hander for all ('#') topics
var All mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	ncutils.Log("default message handler -- received message but not handling")
	ncutils.Log("Topic: " + string(msg.Topic()))
	//ncutils.Log("Message: " + string(msg.Payload()))
}

// NodeUpdate -- mqtt message handler for /update/<NodeID> topic
func NodeUpdate(client mqtt.Client, msg mqtt.Message) {
	var newNode models.Node
	var cfg config.ClientConfig
	var network = parseNetworkFromTopic(msg.Topic())
	cfg.Network = network
	cfg.ReadConfig()

	data, dataErr := decryptMsg(&cfg, msg.Payload())
	if dataErr != nil {
		return
	}
	err := json.Unmarshal([]byte(data), &newNode)
	if err != nil {
		ncutils.Log("error unmarshalling node update data" + err.Error())
		return
	}

	ncutils.Log("received message to update node " + newNode.Name)
	// see if cache hit, if so skip
	var currentMessage = read(newNode.Network, lastNodeUpdate)
	if currentMessage == string(data) {
		return
	}
	insert(newNode.Network, lastNodeUpdate, string(data)) // store new message in cache

	//check if interface name has changed if so delete.
	if cfg.Node.Interface != newNode.Interface {
		if err = wireguard.RemoveConf(cfg.Node.Interface, true); err != nil {
			ncutils.PrintLog("could not delete old interface "+cfg.Node.Interface+": "+err.Error(), 0)
		}
	}
	//ensure that OS never changes
	newNode.OS = runtime.GOOS
	// check if interface needs to delta
	ifaceDelta := ncutils.IfaceDelta(&cfg.Node, &newNode)
	shouldDNSChange := cfg.Node.DNSOn != newNode.DNSOn

	cfg.Node = newNode
	switch newNode.Action {
	case models.NODE_DELETE:
		if cancel, ok := networkcontext.Load(newNode.Network); ok {
			ncutils.Log("cancelling message queue context for " + newNode.Network)
			cancel.(context.CancelFunc)()
		} else {
			ncutils.Log("failed to kill go routines for network " + newNode.Network)
		}
		ncutils.PrintLog(fmt.Sprintf("received delete request for %s", cfg.Node.Name), 0)
		if err = LeaveNetwork(cfg.Node.Network); err != nil {
			if !strings.Contains("rpc error", err.Error()) {
				ncutils.PrintLog(fmt.Sprintf("failed to leave, please check that local files for network %s were removed", cfg.Node.Network), 0)
			}
			ncutils.PrintLog(fmt.Sprintf("%s was removed", cfg.Node.Name), 0)
			return
		}
		ncutils.PrintLog(fmt.Sprintf("%s was removed", cfg.Node.Name), 0)
		return
	case models.NODE_UPDATE_KEY:
		if err := UpdateKeys(&cfg, client); err != nil {
			ncutils.PrintLog("err updating wireguard keys: "+err.Error(), 0)
		}
	case models.NODE_NOOP:
	default:
	}
	// Save new config
	cfg.Node.Action = models.NODE_NOOP
	if err := config.Write(&cfg, cfg.Network); err != nil {
		ncutils.PrintLog("error updating node configuration: "+err.Error(), 0)
	}
	nameserver := cfg.Server.CoreDNSAddr
	privateKey, err := wireguard.RetrievePrivKey(newNode.Network)
	if err != nil {
		ncutils.Log("error reading PrivateKey " + err.Error())
		return
	}
	file := ncutils.GetNetclientPathSpecific() + cfg.Node.Interface + ".conf"

	if err := wireguard.UpdateWgInterface(file, privateKey, nameserver, newNode); err != nil {
		ncutils.Log("error updating wireguard config " + err.Error())
		return
	}
	if ifaceDelta { // if a change caused an ifacedelta we need to notify the server to update the peers
		ackErr := publishSignal(&cfg, ncutils.ACK)
		if ackErr != nil {
			ncutils.Log("could not notify server that it received an interface update")
		} else {
			ncutils.Log("signalled acknowledgement of change to server")
		}
		ncutils.Log("applying WG conf to " + file)
		err = wireguard.ApplyConf(&cfg.Node, cfg.Node.Interface, file)
		if err != nil {
			ncutils.Log("error restarting wg after node update " + err.Error())
			return
		}

		time.Sleep(time.Second >> 0)
		if newNode.DNSOn == "yes" {
			for _, server := range newNode.NetworkSettings.DefaultServerAddrs {
				if server.IsLeader {
					go local.SetDNSWithRetry(newNode, server.Address)
					break
				}
			}
		}
		doneErr := publishSignal(&cfg, ncutils.DONE)
		if doneErr != nil {
			ncutils.Log("could not notify server to update peers after interface change")
		} else {
			ncutils.Log("signalled finshed interface update to server")
		}
	}
	//deal with DNS
	if newNode.DNSOn != "yes" && shouldDNSChange && cfg.Node.Interface != "" {
		ncutils.Log("settng DNS off")
		_, err := ncutils.RunCmd("/usr/bin/resolvectl revert "+cfg.Node.Interface, true)
		if err != nil {
			ncutils.Log("error applying dns" + err.Error())
		}
	}
}

// UpdatePeers -- mqtt message handler for peers/<Network>/<NodeID> topic
func UpdatePeers(client mqtt.Client, msg mqtt.Message) {
	var peerUpdate models.PeerUpdate
	var network = parseNetworkFromTopic(msg.Topic())
	var cfg = config.ClientConfig{}
	cfg.Network = network
	cfg.ReadConfig()

	data, dataErr := decryptMsg(&cfg, msg.Payload())
	if dataErr != nil {
		return
	}
	err := json.Unmarshal([]byte(data), &peerUpdate)
	if err != nil {
		ncutils.Log("error unmarshalling peer data")
		return
	}
	// see if cached hit, if so skip
	var currentMessage = read(peerUpdate.Network, lastPeerUpdate)
	if currentMessage == string(data) {
		return
	}
	insert(peerUpdate.Network, lastPeerUpdate, string(data))

	file := ncutils.GetNetclientPathSpecific() + cfg.Node.Interface + ".conf"
	spew.Dump(peerUpdate.Peers)
	err = wireguard.UpdateWgPeers(file, peerUpdate.Peers)
	if err != nil {
		ncutils.Log("error updating wireguard peers" + err.Error())
		return
	}
	//err = wireguard.SyncWGQuickConf(cfg.Node.Interface, file)
	var iface = cfg.Node.Interface
	if ncutils.IsMac() {
		iface, err = local.GetMacIface(cfg.Node.Address)
		if err != nil {
			ncutils.Log("error retrieving mac iface: " + err.Error())
			return
		}
	}
	err = wireguard.SetPeers(iface, cfg.Node.Address, cfg.Node.PersistentKeepalive, peerUpdate.Peers)
	if err != nil {
		ncutils.Log("error syncing wg after peer update: " + err.Error())
		return
	}
}
