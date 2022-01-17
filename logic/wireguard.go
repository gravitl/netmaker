package logic

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// GetSystemPeers - gets the server peers
func GetSystemPeers(node *models.Node) (map[string]string, error) {
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
	for _, peer := range device.Peers {
		if IsBase64(peer.PublicKey.String()) && peer.Endpoint != nil && CheckEndpoint(peer.Endpoint.String()) {
			peers[peer.PublicKey.String()] = peer.Endpoint.String()
		}
	}
	return peers, nil
}

// RemoveConf - removes a configuration for a given WireGuard interface
func RemoveConf(iface string, printlog bool) error {
	var err error
	confPath := ncutils.GetNetclientPathSpecific() + iface + ".conf"
	err = removeWGQuickConf(confPath, printlog)
	return err
}

// == Private Functions ==

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
		setKernelDevice(ifacename, node.Address)
	}

	nodeport := int(node.ListenPort)
	var conf = wgtypes.Config{
		PrivateKey:   &key,
		ListenPort:   &nodeport,
		ReplacePeers: true,
		Peers:        peers,
	}

	if !ncutils.IsKernel() {
		var newConf string
		newConf, _ = ncutils.CreateWireGuardConf(node, key.String(), strconv.FormatInt(int64(node.ListenPort), 10), peers)
		confPath := ncutils.GetNetclientPathSpecific() + ifacename + ".conf"
		logger.Log(1, "writing wg conf file to:", confPath)
		err = os.WriteFile(confPath, []byte(newConf), 0644)
		if err != nil {
			logger.Log(1, "error writing wg conf file to", confPath, ":", err.Error())
			return err
		}
		if ncutils.IsWindows() {
			wgConfPath := ncutils.GetWGPathSpecific() + ifacename + ".conf"
			logger.Log(1, "writing wg conf file to:", confPath)
			err = os.WriteFile(wgConfPath, []byte(newConf), 0644)
			if err != nil {
				logger.Log(1, "error writing wg conf file to", wgConfPath, ":", err.Error())
				return err
			}
			confPath = wgConfPath
		}
		// spin up userspace + apply the conf file
		var deviceiface = ifacename
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
		if node.Address6 != "" && node.IsDualStack == "yes" {
			logger.Log(1, "adding address:", node.Address6)
			_, _ = ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+node.Address6+"/64", true)
		}
	}

	return err
}

func setKernelDevice(ifacename string, address string) error {
	ipExec, err := exec.LookPath("ip")
	if err != nil {
		return err
	}

	_, _ = ncutils.RunCmd("ip link delete dev "+ifacename, false)
	_, _ = ncutils.RunCmd(ipExec+" link add dev "+ifacename+" type wireguard", true)
	_, _ = ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+address+"/24", true) // this is a bug waiting to happen

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

func setServerPeers(iface string, keepalive int32, peers []wgtypes.PeerConfig) error {

	client, err := wgctrl.New()
	if err != nil {
		logger.Log(0, "failed to start wgctrl")
		return err
	}
	defer client.Close()

	device, err := client.Device(iface)
	if err != nil {
		logger.Log(1, "failed to parse interface")
		return err
	}
	devicePeers := device.Peers
	if len(devicePeers) > 1 && len(peers) == 0 {
		logger.Log(1, "no peers pulled")
		return err
	}

	for _, peer := range peers {
		if len(peer.AllowedIPs) > 0 {
			for _, currentPeer := range devicePeers {
				if len(currentPeer.AllowedIPs) > 0 && currentPeer.AllowedIPs[0].String() == peer.AllowedIPs[0].String() &&
					currentPeer.PublicKey.String() != peer.PublicKey.String() {
					_, err := ncutils.RunCmd("wg set "+iface+" peer "+currentPeer.PublicKey.String()+" remove", true)
					if err != nil {
						logger.Log(0, "error removing peer", peer.Endpoint.String())
					}
				}
			}
		}
		udpendpoint := peer.Endpoint.String()
		var allowedips string
		var iparr []string
		for _, ipaddr := range peer.AllowedIPs {
			iparr = append(iparr, ipaddr.String())
		}
		allowedips = strings.Join(iparr, ",")
		keepAliveString := strconv.Itoa(int(keepalive))
		if keepAliveString == "0" {
			keepAliveString = "5"
		}
		if peer.Endpoint != nil {
			_, err = ncutils.RunCmd("wg set "+iface+" peer "+peer.PublicKey.String()+
				" endpoint "+udpendpoint+
				" persistent-keepalive "+keepAliveString+
				" allowed-ips "+allowedips, true)
		} else {
			_, err = ncutils.RunCmd("wg set "+iface+" peer "+peer.PublicKey.String()+
				" persistent-keepalive "+keepAliveString+
				" allowed-ips "+allowedips, true)
		}
		if err != nil {
			logger.Log(2, "error setting peer", peer.PublicKey.String())
		}
	}

	for _, currentPeer := range devicePeers {
		if len(currentPeer.AllowedIPs) > 0 {
			shouldDelete := true
			for _, peer := range peers {
				if len(peer.AllowedIPs) > 0 && peer.AllowedIPs[0].String() == currentPeer.AllowedIPs[0].String() {
					shouldDelete = false
				}
			}
			if shouldDelete {
				output, err := ncutils.RunCmd("wg set "+iface+" peer "+currentPeer.PublicKey.String()+" remove", true)
				if err != nil {
					logger.Log(0, output, "error removing peer", currentPeer.PublicKey.String())
				}
			}
		}
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
		err = setServerPeers(node.Interface, node.PersistentKeepalive, peers[:])
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
	var ifacename = node.Interface
	var err error
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
