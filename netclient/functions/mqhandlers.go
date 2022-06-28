package functions

import (
	"encoding/json"
	"runtime"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"github.com/guumaster/hostctl/pkg/file"
	"github.com/guumaster/hostctl/pkg/parser"
	"github.com/guumaster/hostctl/pkg/types"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// All -- mqtt message hander for all ('#') topics
var All mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	logger.Log(0, "default message handler -- received message but not handling")
	logger.Log(0, "Topic: "+string(msg.Topic()))
	//logger.Log(0, "Message: " + string(msg.Payload()))
}

// NodeUpdate -- mqtt message handler for /update/<NodeID> topic
func NodeUpdate(client mqtt.Client, msg mqtt.Message) {
	var newNode models.Node
	var nodeCfg config.ClientConfig
	var network = parseNetworkFromTopic(msg.Topic())
	nodeCfg.Network = network
	nodeCfg.ReadConfig()

	data, dataErr := decryptMsg(&nodeCfg, msg.Payload())
	if dataErr != nil {
		return
	}
	err := json.Unmarshal([]byte(data), &newNode)
	if err != nil {
		logger.Log(0, "error unmarshalling node update data"+err.Error())
		return
	}

	logger.Log(0, "received message to update node "+newNode.Name)
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
	keepaliveChange := nodeCfg.Node.PersistentKeepalive != newNode.PersistentKeepalive

	nodeCfg.Node = newNode
	switch newNode.Action {
	case models.NODE_DELETE:
		logger.Log(0, "received delete request for %s", nodeCfg.Node.Name)
		unsubscribeNode(client, &nodeCfg)
		if err = LeaveNetwork(nodeCfg.Node.Network); err != nil {
			if !strings.Contains("rpc error", err.Error()) {
				logger.Log(0, "failed to leave, please check that local files for network", nodeCfg.Node.Network, "were removed")
				return
			}
		}
		logger.Log(0, nodeCfg.Node.Name, " was removed")
		return
	case models.NODE_UPDATE_KEY:
		// == get the current key for node ==
		oldPrivateKey, retErr := wireguard.RetrievePrivKey(nodeCfg.Network)
		if retErr != nil {
			break
		}
		if err := UpdateKeys(&nodeCfg, client); err != nil {
			logger.Log(0, "err updating wireguard keys, reusing last key\n", err.Error())
			if key, parseErr := wgtypes.ParseKey(oldPrivateKey); parseErr == nil {
				wireguard.StorePrivKey(key.String(), nodeCfg.Network)
				nodeCfg.Node.PublicKey = key.PublicKey().String()
			}
		}
		ifaceDelta = true
	case models.NODE_FORCE_UPDATE:
		ifaceDelta = true
	case models.NODE_NOOP:
	default:
	}
	// Save new config
	nodeCfg.Node.Action = models.NODE_NOOP
	if err := config.Write(&nodeCfg, nodeCfg.Network); err != nil {
		logger.Log(0, "error updating node configuration: ", err.Error())
	}
	nameserver := nodeCfg.Server.CoreDNSAddr
	privateKey, err := wireguard.RetrievePrivKey(newNode.Network)
	if err != nil {
		logger.Log(0, "error reading PrivateKey "+err.Error())
		return
	}
	file := ncutils.GetNetclientPathSpecific() + nodeCfg.Node.Interface + ".conf"

	if err := wireguard.UpdateWgInterface(file, privateKey, nameserver, newNode); err != nil {
		logger.Log(0, "error updating wireguard config "+err.Error())
		return
	}
	if keepaliveChange {
		wireguard.UpdateKeepAlive(file, newNode.PersistentKeepalive)
	}
	if ifaceDelta { // if a change caused an ifacedelta we need to notify the server to update the peers
		logger.Log(0, "applying WG conf to "+file)
		if ncutils.IsWindows() {
			wireguard.RemoveConfGraceful(nodeCfg.Node.Interface)
		}
		err = wireguard.ApplyConf(&nodeCfg.Node, nodeCfg.Node.Interface, file)
		if err != nil {
			logger.Log(0, "error restarting wg after node update "+err.Error())
			return
		}

		time.Sleep(time.Second >> 0)
		//	if newNode.DNSOn == "yes" {
		//		for _, server := range newNode.NetworkSettings.DefaultServerAddrs {
		//			if server.IsLeader {
		//				go local.SetDNSWithRetry(newNode, server.Address)
		//				break
		//			}
		//		}
		//	}
		doneErr := publishSignal(&nodeCfg, ncutils.DONE)
		if doneErr != nil {
			logger.Log(0, "could not notify server to update peers after interface change")
		} else {
			logger.Log(0, "signalled finished interface update to server")
		}
	} else if hubChange {
		doneErr := publishSignal(&nodeCfg, ncutils.DONE)
		if doneErr != nil {
			logger.Log(0, "could not notify server to update peers after hub change")
		} else {
			logger.Log(0, "signalled finished hub update to server")
		}
	}
	//deal with DNS
	if newNode.DNSOn != "yes" && shouldDNSChange && nodeCfg.Node.Interface != "" {
		logger.Log(0, "settng DNS off")
		if err := removeHostDNS(nodeCfg.Node.Interface, ncutils.IsWindows()); err != nil {
			logger.Log(0, "error removing netmaker profile from /etc/hosts "+err.Error())
		}
		//		_, err := ncutils.RunCmd("/usr/bin/resolvectl revert "+nodeCfg.Node.Interface, true)
		//		if err != nil {
		//			logger.Log(0, "error applying dns" + err.Error())
		//		}
	}
	_ = UpdateLocalListenPort(&nodeCfg)
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
		logger.Log(0, "error unmarshalling peer data")
		return
	}
	// see if cached hit, if so skip
	var currentMessage = read(peerUpdate.Network, lastPeerUpdate)
	if currentMessage == string(data) {
		return
	}
	insert(peerUpdate.Network, lastPeerUpdate, string(data))
	// check version
	if peerUpdate.ServerVersion != ncutils.Version {
		logger.Log(0, "server/client version mismatch server: ", peerUpdate.ServerVersion, " client: ", ncutils.Version)
	}
	if peerUpdate.ServerVersion != cfg.Server.Version {
		logger.Log(1, "updating server version")
		cfg.Server.Version = peerUpdate.ServerVersion
		config.Write(&cfg, cfg.Network)
	}

	file := ncutils.GetNetclientPathSpecific() + cfg.Node.Interface + ".conf"
	err = wireguard.UpdateWgPeers(file, peerUpdate.Peers)
	if err != nil {
		logger.Log(0, "error updating wireguard peers"+err.Error())
		return
	}
	queryAddr := cfg.Node.PrimaryAddress()

	//err = wireguard.SyncWGQuickConf(cfg.Node.Interface, file)
	var iface = cfg.Node.Interface
	if ncutils.IsMac() {
		iface, err = local.GetMacIface(queryAddr)
		if err != nil {
			logger.Log(0, "error retrieving mac iface: "+err.Error())
			return
		}
	}
	err = wireguard.SetPeers(iface, &cfg.Node, peerUpdate.Peers)
	if err != nil {
		logger.Log(0, "error syncing wg after peer update: "+err.Error())
		return
	}
	logger.Log(0, "received peer update for node "+cfg.Node.Name+" "+cfg.Node.Network)
	if cfg.Node.DNSOn == "yes" {
		if err := setHostDNS(peerUpdate.DNS, cfg.Node.Interface, ncutils.IsWindows()); err != nil {
			logger.Log(0, "error updating /etc/hosts "+err.Error())
			return
		}
	} else {
		if err := removeHostDNS(cfg.Node.Interface, ncutils.IsWindows()); err != nil {
			logger.Log(0, "error removing profile from /etc/hosts "+err.Error())
			return
		}
	}
	_ = UpdateLocalListenPort(&cfg)
}

func setHostDNS(dns, iface string, windows bool) error {
	etchosts := "/etc/hosts"
	if windows {
		etchosts = "c:\\windows\\system32\\drivers\\etc\\hosts"
	}
	dnsdata := strings.NewReader(dns)
	profile, err := parser.ParseProfile(dnsdata)
	if err != nil {
		return err
	}
	hosts, err := file.NewFile(etchosts)
	if err != nil {
		return err
	}
	profile.Name = strings.ToLower(iface)
	profile.Status = types.Enabled
	if err := hosts.ReplaceProfile(profile); err != nil {
		return err
	}
	if err := hosts.Flush(); err != nil {
		return err
	}
	return nil
}

func removeHostDNS(iface string, windows bool) error {
	etchosts := "/etc/hosts"
	if windows {
		etchosts = "c:\\windows\\system32\\drivers\\etc\\hosts"
	}
	hosts, err := file.NewFile(etchosts)
	if err != nil {
		return err
	}
	if err := hosts.RemoveProfile(strings.ToLower(iface)); err != nil {
		if err == types.ErrUnknownProfile {
			return nil
		}
		return err
	}
	if err := hosts.Flush(); err != nil {
		return err
	}
	return nil
}
