package functions

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
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
	var nodeCfg config.ClientConfig
	var network = parseNetworkFromTopic(msg.Topic())
	nodeCfg.Network = network
	nodeCfg.ReadConfig()
	var commsCfg = getCommsCfgByNode(&nodeCfg.Node)

	data, dataErr := decryptMsg(&nodeCfg, msg.Payload())
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

	// ensure that OS never changes
	newNode.OS = runtime.GOOS
	// check if interface needs to delta
	ifaceDelta := ncutils.IfaceDelta(&nodeCfg.Node, &newNode)
	shouldDNSChange := nodeCfg.Node.DNSOn != newNode.DNSOn
	hubChange := nodeCfg.Node.IsHub != newNode.IsHub

	nodeCfg.Node = newNode
	switch newNode.Action {
	case models.NODE_DELETE:
		ncutils.PrintLog(fmt.Sprintf("received delete request for %s", nodeCfg.Node.Name), 0)
		unsubscribeNode(client, &nodeCfg)
		if err = LeaveNetwork(nodeCfg.Node.Network, true); err != nil {
			if !strings.Contains("rpc error", err.Error()) {
				ncutils.PrintLog(fmt.Sprintf("failed to leave, please check that local files for network %s were removed", nodeCfg.Node.Network), 0)
				return
			}
		}
		ncutils.PrintLog(fmt.Sprintf("%s was removed", nodeCfg.Node.Name), 0)
		return
	case models.NODE_UPDATE_KEY:
		// == get the current key for node ==
		oldPrivateKey, retErr := wireguard.RetrievePrivKey(nodeCfg.Network)
		if retErr != nil {
			break
		}
		if err := UpdateKeys(&nodeCfg, client); err != nil {
			ncutils.PrintLog("err updating wireguard keys, reusing last key\n"+err.Error(), 0)
			if key, parseErr := wgtypes.ParseKey(oldPrivateKey); parseErr == nil {
				wireguard.StorePrivKey(key.String(), nodeCfg.Network)
				nodeCfg.Node.PublicKey = key.PublicKey().String()
			}
		}
		ifaceDelta = true
	case models.NODE_NOOP:
	default:
	}
	// Save new config
	nodeCfg.Node.Action = models.NODE_NOOP
	if err := config.Write(&nodeCfg, nodeCfg.Network); err != nil {
		ncutils.PrintLog("error updating node configuration: "+err.Error(), 0)
	}
	nameserver := nodeCfg.Server.CoreDNSAddr
	privateKey, err := wireguard.RetrievePrivKey(newNode.Network)
	if err != nil {
		ncutils.Log("error reading PrivateKey " + err.Error())
		return
	}
	file := ncutils.GetNetclientPathSpecific() + nodeCfg.Node.Interface + ".conf"

	if err := wireguard.UpdateWgInterface(file, privateKey, nameserver, newNode); err != nil {
		ncutils.Log("error updating wireguard config " + err.Error())
		return
	}
	if ifaceDelta { // if a change caused an ifacedelta we need to notify the server to update the peers
		ncutils.Log("applying WG conf to " + file)
		err = wireguard.ApplyConf(&nodeCfg.Node, nodeCfg.Node.Interface, file)
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
		doneErr := publishSignal(&commsCfg, &nodeCfg, ncutils.DONE)
		if doneErr != nil {
			ncutils.Log("could not notify server to update peers after interface change")
		} else {
			ncutils.Log("signalled finished interface update to server")
		}
	} else if hubChange {
		doneErr := publishSignal(&commsCfg, &nodeCfg, ncutils.DONE)
		if doneErr != nil {
			ncutils.Log("could not notify server to update peers after hub change")
		} else {
			ncutils.Log("signalled finished hub update to server")
		}
	}
	//deal with DNS
	if newNode.DNSOn != "yes" && shouldDNSChange && nodeCfg.Node.Interface != "" {
		ncutils.Log("settng DNS off")
		_, err := ncutils.RunCmd("/usr/bin/resolvectl revert "+nodeCfg.Node.Interface, true)
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
