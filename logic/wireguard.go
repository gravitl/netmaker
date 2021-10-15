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

// RemoveConf - removes a configuration for a given WireGuard interface
func RemoveConf(iface string, printlog bool) error {
	var err error
	confPath := ncutils.GetNetclientPathSpecific() + iface + ".conf"
	err = removeWGQuickConf(confPath, printlog)
	return err
}

// == Private Methods ==

func setWGConfig(node models.Node, network string, peerupdate bool) error {

	node.SetID()
	peers, hasGateway, gateways, err := GetServerPeers(node.MacAddress, node.Network, node.IsDualStack == "yes", node.IsIngressGateway == "yes")
	if err != nil {
		return err
	}
	privkey, err := FetchPrivKey(node.ID)
	if err != nil {
		return err
	}
	if peerupdate {
		var iface string
		iface = node.Interface
		err = setServerPeers(iface, node.PersistentKeepalive, peers)
	} else {
		err = initWireguard(&node, privkey, peers, hasGateway, gateways)
	}
	Log("finished setting wg config on server "+node.Name, 1)
	return err
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
		Log("setting kernel device "+ifacename, 2)
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
		if node.UDPHolePunch != "yes" || node.IsServer == "yes" {
			newConf, _ = ncutils.CreateUserSpaceConf(node.Address, key.String(), strconv.FormatInt(int64(node.ListenPort), 10), node.MTU, node.PersistentKeepalive, peers)
		} else {
			newConf, _ = ncutils.CreateUserSpaceConf(node.Address, key.String(), "", node.MTU, node.PersistentKeepalive, peers)
		}
		confPath := ncutils.GetNetclientPathSpecific() + ifacename + ".conf"
		Log("writing wg conf file to: "+confPath, 1)
		err = ioutil.WriteFile(confPath, []byte(newConf), 0644)
		if err != nil {
			Log("error writing wg conf file to "+confPath+": "+err.Error(), 1)
			return err
		}
		// spin up userspace + apply the conf file
		var deviceiface string
		deviceiface = ifacename
		d, _ := wgclient.Device(deviceiface)
		for d != nil && d.Name == deviceiface {
			_ = RemoveConf(ifacename, false) // remove interface first
			time.Sleep(time.Second >> 2)
			d, _ = wgclient.Device(deviceiface)
		}
		err = applyWGQuickConf(confPath)
		if err != nil {
			Log("failed to create wireguard interface", 1)
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
			Log("attempted to remove interface before editing", 2)
			return err
		}

		if node.PostDown != "" {
			runcmds := strings.Split(node.PostDown, "; ")
			_ = ncutils.RunCmds(runcmds, true)
		}
		// set MTU of node interface
		if _, err := ncutils.RunCmd(ipExec+" link set mtu "+strconv.Itoa(int(node.MTU))+" up dev "+ifacename, true); err != nil {
			Log("failed to create interface with mtu "+ifacename, 2)
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
		Log("failed to start wgctrl", 0)
		return err
	}

	device, err := client.Device(iface)
	if err != nil {
		Log("failed to parse interface", 0)
		return err
	}
	devicePeers := device.Peers
	if len(devicePeers) > 1 && len(peers) == 0 {
		Log("no peers pulled", 1)
		return err
	}

	for _, peer := range peers {

		for _, currentPeer := range devicePeers {
			if currentPeer.AllowedIPs[0].String() == peer.AllowedIPs[0].String() &&
				currentPeer.PublicKey.String() != peer.PublicKey.String() {
				_, err := ncutils.RunCmd("wg set "+iface+" peer "+currentPeer.PublicKey.String()+" remove", true)
				if err != nil {
					log.Println("error removing peer", peer.Endpoint.String())
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
			Log("error setting peer "+peer.PublicKey.String(), 1)
		}
	}

	for _, currentPeer := range devicePeers {
		shouldDelete := true
		for _, peer := range peers {
			if peer.AllowedIPs[0].String() == currentPeer.AllowedIPs[0].String() {
				shouldDelete = false
			}
		}
		if shouldDelete {
			output, err := ncutils.RunCmd("wg set "+iface+" peer "+currentPeer.PublicKey.String()+" remove", true)
			if err != nil {
				log.Println(output, "error removing peer", currentPeer.PublicKey.String())
			}
		}
	}

	return nil
}

func setWGKeyConfig(node models.Node) error {

	node.SetID()
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

	return setWGConfig(node, node.Network, false)
}
