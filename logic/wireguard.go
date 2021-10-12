package logic

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/local"
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
		Log("no interface to configure", 0)
	}
	if node.Address == "" {
		Log("no address to configure", 0)
	}

	if ncutils.IsKernel() {
		setKernelDevice(ifacename, node.Address)
	}

	nodeport := int(node.ListenPort)
	var conf wgtypes.Config
	conf = wgtypes.Config{
		PrivateKey:   &key,
		ListenPort:   &nodeport,
		ReplacePeers: true,
		Peers:        peers,
	}

	if !ncutils.IsKernel() {
		var newConf string
		if node.UDPHolePunch != "yes" {
			newConf, _ = ncutils.CreateUserSpaceConf(node.Address, key.String(), strconv.FormatInt(int64(node.ListenPort), 10), node.MTU, node.PersistentKeepalive, peers)
		} else {
			newConf, _ = ncutils.CreateUserSpaceConf(node.Address, key.String(), "", node.MTU, node.PersistentKeepalive, peers)
		}
		confPath := ncutils.GetNetclientPathSpecific() + ifacename + ".conf"
		ncutils.PrintLog("writing wg conf file to: "+confPath, 1)
		err = ioutil.WriteFile(confPath, []byte(newConf), 0644)
		if err != nil {
			ncutils.PrintLog("error writing wg conf file to "+confPath+": "+err.Error(), 1)
			return err
		}
		// spin up userspace + apply the conf file
		var deviceiface string
		if ncutils.IsMac() {
			deviceiface, err = local.GetMacIface(node.Address)
			if err != nil || deviceiface == "" {
				deviceiface = ifacename
			}
		}
		d, _ := wgclient.Device(deviceiface)
		for d != nil && d.Name == deviceiface {
			_ = RemoveConf(ifacename, false) // remove interface first
			time.Sleep(time.Second >> 2)
			d, _ = wgclient.Device(deviceiface)
		}
		err = applyWGQuickConf(confPath)
		if err != nil {
			ncutils.PrintLog("failed to create wireguard interface", 1)
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
			ncutils.Log("attempted to remove interface before editing")
			return err
		}

		if node.PostDown != "" {
			runcmds := strings.Split(node.PostDown, "; ")
			_ = ncutils.RunCmds(runcmds, true)
		}
		// set MTU of node interface
		if _, err := ncutils.RunCmd(ipExec+" link set mtu "+strconv.Itoa(int(node.MTU))+" up dev "+ifacename, true); err != nil {
			ncutils.Log("failed to create interface with mtu " + ifacename)
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
			log.Println("[netclient] adding address: "+node.Address6, 1)
			_, _ = ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+node.Address6+"/64", true)
		}
	}

	return err
}

// RemoveConf - removes a configuration for a given WireGuard interface
func RemoveConf(iface string, printlog bool) error {
	var err error
	confPath := ncutils.GetNetclientPathSpecific() + iface + ".conf"
	err = removeWGQuickConf(confPath, printlog)
	return err
}

// == Private Methods ==

func setKernelDevice(ifacename string, address string) error {
	ipExec, err := exec.LookPath("ip")
	if err != nil {
		return err
	}

	_, _ = ncutils.RunCmd("ip link delete dev "+ifacename, false)
	_, _ = ncutils.RunCmd(ipExec+" link add dev "+ifacename+" type wireguard", true)
	_, _ = ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+address+"/24", true)

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
