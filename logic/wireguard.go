package logic

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/local"
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

func initWireguard(node *models.Node, privkey string, peers []wgtypes.PeerConfig, hasGateway bool, gateways []string) error {

	key, err := wgtypes.ParseKey(privkey)
	if err != nil {
		return err
	}

	wgclient, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wgclient.Close()

	var ifacename string
	if node.Interface != "" {
		ifacename = node.Interface
	} else {
		logger.Log(2, "no server interface provided to configure")
	}
	if node.Address == "" {
		logger.Log(2, "no server address to provided configure")
	}

	if ncutils.IsKernel() {
		logger.Log(2, "setting kernel device", ifacename)
		network, err := GetNetwork(node.Network)
		if err != nil {
			logger.Log(0, "failed to get network"+err.Error())
			return err
		}
		var address4 string
		var address6 string
		var mask4 string
		var mask6 string
		if network.AddressRange != "" {
			net := strings.Split(network.AddressRange, "/")
			mask4 = net[len(net)-1]
			address4 = node.Address
		}
		if network.AddressRange6 != "" {
			net := strings.Split(network.AddressRange6, "/")
			mask6 = net[len(net)-1]
			address6 = node.Address6
		}

		setKernelDevice(ifacename, address4, mask4, address6, mask6)
	}

	nodeport := int(node.ListenPort)
	var conf = wgtypes.Config{
		PrivateKey:   &key,
		ListenPort:   &nodeport,
		ReplacePeers: true,
		Peers:        peers,
	}

	if !ncutils.IsKernel() {
		if err := wireguard.WriteWgConfig(node, key.String(), peers); err != nil {
			logger.Log(1, "error writing wg conf file: ", err.Error())
			return err
		}
		// spin up userspace + apply the conf file
		var deviceiface = ifacename
		confPath := ncutils.GetNetclientPathSpecific() + ifacename + ".conf"
		d, _ := wgclient.Device(deviceiface)
		for d != nil && d.Name == deviceiface {
			_ = RemoveConf(ifacename, false) // remove interface first
			time.Sleep(time.Second >> 2)
			d, _ = wgclient.Device(deviceiface)
		}
		time.Sleep(time.Second >> 2)
		err = applyWGQuickConf(confPath)
		if err != nil {
			logger.Log(1, "failed to create wireguard interface")
			return err
		}
	} else {
		ipExec, err := exec.LookPath("ip")
		if err != nil {
			return err
		}

		_, err = wgclient.Device(ifacename)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("Device does not exist: ")
				fmt.Println(err)
			} else {
				return errors.New("Unknown config error: " + err.Error())
			}
		}

		err = wgclient.ConfigureDevice(ifacename, conf)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("Device does not exist: ")
				fmt.Println(err)
			} else {
				fmt.Printf("This is inconvenient: %v", err)
			}
		}

		if _, err := ncutils.RunCmd(ipExec+" link set down dev "+ifacename, false); err != nil {
			logger.Log(2, "attempted to remove interface before editing")
			return err
		}

		if node.PostDown != "" {
			runcmds := strings.Split(node.PostDown, "; ")
			_ = ncutils.RunCmds(runcmds, false)
		}
		// set MTU of node interface
		if _, err := ncutils.RunCmd(ipExec+" link set mtu "+strconv.Itoa(int(node.MTU))+" up dev "+ifacename, true); err != nil {
			logger.Log(2, "failed to create interface with mtu", strconv.Itoa(int(node.MTU)), "-", ifacename)
			return err
		}

		if node.PostUp != "" {
			runcmds := strings.Split(node.PostUp, "; ")
			_ = ncutils.RunCmds(runcmds, true)
		}
		if hasGateway {
			for _, gateway := range gateways {
				_, _ = ncutils.RunCmd(ipExec+" -4 route add "+gateway+" dev "+ifacename, true)
			}
		}
		if node.Address != "" {
			logger.Log(1, "adding address:", node.Address)
			_, _ = ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+node.Address+"/32", true)
		}
		if node.Address6 != "" {
			logger.Log(1, "adding address6:", node.Address6)
			_, _ = ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+node.Address6+"/128", true)
		}
		wireguard.SetPeers(ifacename, node, peers)
	}

	if node.IsServer == "yes" {
		setServerRoutes(node.Interface, node.Network)
	}

	return err
}

func setKernelDevice(ifacename, address4, mask4, address6, mask6 string) error {
	ipExec, err := exec.LookPath("ip")
	if err != nil {
		return err
	}

	// == best effort ==
	ncutils.RunCmd("ip link delete dev "+ifacename, false)
	ncutils.RunCmd(ipExec+" link add dev "+ifacename+" type wireguard", true)
	if address4 != "" {
		ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+address4+"/"+mask4, true)
	}
	if address6 != "" {
		ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+address6+"/"+mask6, true)
	}

	return nil
}

func applyWGQuickConf(confPath string) error {
	if _, err := ncutils.RunCmd("wg-quick up "+confPath, true); err != nil {
		return err
	}
	return nil
}

func removeWGQuickConf(confPath string, printlog bool) error {
	if _, err := ncutils.RunCmd("wg-quick down "+confPath, printlog); err != nil {
		return err
	}
	return nil
}

func setWGConfig(node *models.Node, peerupdate bool) error {

	peers, hasGateway, gateways, err := GetServerPeers(node)
	if err != nil {
		return err
	}
	privkey, err := FetchPrivKey(node.ID)
	if err != nil {
		return err
	}
	if peerupdate {
		if err := wireguard.SetPeers(node.Interface, node, peers); err != nil {
			logger.Log(0, "error updating peers", err.Error())
		}
		logger.Log(2, "updated peers on server", node.Name)
	} else {
		err = initWireguard(node, privkey, peers[:], hasGateway, gateways[:])
		logger.Log(3, "finished setting wg config on server", node.Name)
	}
	peers = nil
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

func setServerRoutes(iface, network string) {
	parentNetwork, err := GetParentNetwork(network)
	if err == nil {
		if parentNetwork.AddressRange != "" {
			ip, cidr, err := net.ParseCIDR(parentNetwork.AddressRange)
			if err == nil {
				local.SetCIDRRoute(iface, ip.String(), cidr)
			}
		}
		if parentNetwork.AddressRange6 != "" {
			ip, cidr, err := net.ParseCIDR(parentNetwork.AddressRange6)
			if err == nil {
				local.SetCIDRRoute(iface, ip.String(), cidr)
			}
		}
	}
}
