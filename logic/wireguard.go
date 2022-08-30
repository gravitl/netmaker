package logic

import (
	"os"
	"os/exec"
	"strings"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// RemoveConf - removes a configuration for a given WireGuard interface
func RemoveConf(iface string, printlog bool) error {
	var err error
	confPath := ncutils.GetNetclientPathSpecific() + iface + ".conf"
	err = removeWGQuickConf(confPath, printlog)
	return err
}

// HasPeerConnected - checks if a client node has connected over WG
func HasPeerConnected(node *models.Node) bool {
	client, err := wgctrl.New()
	if err != nil {
		return false
	}
	defer client.Close()
	device, err := client.Device(node.Interface)
	if err != nil {
		return false
	}
	for _, peer := range device.Peers {
		if peer.PublicKey.String() == node.PublicKey {
			if peer.Endpoint != nil {
				return true
			}
		}
	}
	return false
}

// IfaceDelta - checks if the new node causes an interface change
func IfaceDelta(currentNode *models.Node, newNode *models.Node) bool {
	// single comparison statements
	if newNode.Endpoint != currentNode.Endpoint ||
		newNode.PublicKey != currentNode.PublicKey ||
		newNode.Address != currentNode.Address ||
		newNode.Address6 != currentNode.Address6 ||
		newNode.IsEgressGateway != currentNode.IsEgressGateway ||
		newNode.IsIngressGateway != currentNode.IsIngressGateway ||
		newNode.IsRelay != currentNode.IsRelay ||
		newNode.UDPHolePunch != currentNode.UDPHolePunch ||
		newNode.IsPending != currentNode.IsPending ||
		newNode.ListenPort != currentNode.ListenPort ||
		newNode.MTU != currentNode.MTU ||
		newNode.PersistentKeepalive != currentNode.PersistentKeepalive ||
		newNode.DNSOn != currentNode.DNSOn ||
		newNode.Connected != currentNode.Connected ||
		len(newNode.AllowedIPs) != len(currentNode.AllowedIPs) {
		return true
	}

	// multi-comparison statements
	if newNode.IsEgressGateway == "yes" {
		if len(currentNode.EgressGatewayRanges) != len(newNode.EgressGatewayRanges) {
			return true
		}
		for _, address := range newNode.EgressGatewayRanges {
			if !StringSliceContains(currentNode.EgressGatewayRanges, address) {
				return true
			}
		}
	}

	if newNode.IsRelay == "yes" {
		if len(currentNode.RelayAddrs) != len(newNode.RelayAddrs) {
			return true
		}
		for _, address := range newNode.RelayAddrs {
			if !StringSliceContains(currentNode.RelayAddrs, address) {
				return true
			}
		}
	}

	for _, address := range newNode.AllowedIPs {
		if !StringSliceContains(currentNode.AllowedIPs, address) {
			return true
		}
	}
	return false
}

// == Private Functions ==

// gets the server peers locally
func getSystemPeers(node *models.Node) (map[string]string, error) {
	peers := make(map[string]string)

	client, err := wgctrl.New()
	if err != nil {
		return peers, err
	}
	defer client.Close()
	device, err := client.Device(node.Interface)
	if err != nil {
		return nil, err
	}
	if device.Peers != nil && len(device.Peers) > 0 {
		for _, peer := range device.Peers {
			if IsBase64(peer.PublicKey.String()) && peer.Endpoint != nil && CheckEndpoint(peer.Endpoint.String()) {
				peers[peer.PublicKey.String()] = peer.Endpoint.String()
			}
		}
	}
	return peers, nil
}
func removeWGQuickConf(confPath string, printlog bool) error {
	if _, err := ncutils.RunCmd("wg-quick down "+confPath, printlog); err != nil {
		return err
	}
	return nil
}

func setWGConfig(node *models.Node, peerupdate bool) error {
	peers, err := GetPeerUpdate(node)
	if err != nil {
		return err
	}
	privkey, err := FetchPrivKey(node.ID)
	if err != nil {
		return err
	}
	if peerupdate {
		if err := wireguard.SetPeers(node.Interface, node, peers.Peers); err != nil {
			logger.Log(0, "error updating peers", err.Error())
		}
		logger.Log(2, "updated peers on server", node.Name)
	} else {
		err = wireguard.InitWireguard(node, privkey, peers.Peers)
		logger.Log(3, "finished setting wg config on server", node.Name)
	}
	return err
}

func setWGKeyConfig(node *models.Node) error {

	privatekey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return err
	}
	privkeystring := privatekey.String()
	publickey := privatekey.PublicKey()
	node.PublicKey = publickey.String()

	err = StorePrivKey(node.ID, privkeystring)
	if err != nil {
		return err
	}
	if node.Action == models.NODE_UPDATE_KEY {
		node.Action = models.NODE_NOOP
	}

	return setWGConfig(node, false)
}

func removeLocalServer(node *models.Node) error {

	var err error
	var ifacename = node.Interface
	if err = RemovePrivKey(node.ID); err != nil {
		logger.Log(1, "failed to remove server conf from db", node.ID)
	}
	if ifacename != "" {
		if !ncutils.IsKernel() {
			if err = RemoveConf(ifacename, true); err == nil {
				logger.Log(1, "removed WireGuard interface:", ifacename)
			}
		} else {
			ipExec, err := exec.LookPath("ip")
			if err != nil {
				return err
			}
			out, err := ncutils.RunCmd(ipExec+" link del "+ifacename, false)
			dontprint := strings.Contains(out, "does not exist") || strings.Contains(out, "Cannot find device")
			if err != nil && !dontprint {
				logger.Log(1, "error running command:", ipExec, "link del", ifacename)
				logger.Log(1, out)
			}
			if node.PostDown != "" {
				runcmds := strings.Split(node.PostDown, "; ")
				_ = ncutils.RunCmds(runcmds, false)
			}
		}
	}
	home := ncutils.GetNetclientPathSpecific()
	if ncutils.FileExists(home + "netconfig-" + node.Network) {
		_ = os.Remove(home + "netconfig-" + node.Network)
	}
	if ncutils.FileExists(home + "nettoken-" + node.Network) {
		_ = os.Remove(home + "nettoken-" + node.Network)
	}
	if ncutils.FileExists(home + "secret-" + node.Network) {
		_ = os.Remove(home + "secret-" + node.Network)
	}
	if ncutils.FileExists(home + "wgkey-" + node.Network) {
		_ = os.Remove(home + "wgkey-" + node.Network)
	}
	if ncutils.FileExists(home + "nm-" + node.Network + ".conf") {
		_ = os.Remove(home + "nm-" + node.Network + ".conf")
	}
	return err
}
