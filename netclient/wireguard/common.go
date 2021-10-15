package wireguard

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/server"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// SetPeers - sets peers on a given WireGuard interface
func SetPeers(iface string, keepalive int32, peers []wgtypes.PeerConfig) error {

	client, err := wgctrl.New()
	if err != nil {
		ncutils.PrintLog("failed to start wgctrl", 0)
		return err
	}

	device, err := client.Device(iface)
	if err != nil {
		ncutils.PrintLog("failed to parse interface", 0)
		return err
	}
	devicePeers := device.Peers
	if len(devicePeers) > 1 && len(peers) == 0 {
		ncutils.PrintLog("no peers pulled", 1)
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
			log.Println("error setting peer", peer.PublicKey.String())
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

// Initializes a WireGuard interface
func InitWireguard(node *models.Node, privkey string, peers []wgtypes.PeerConfig, hasGateway bool, gateways []string) error {

	key, err := wgtypes.ParseKey(privkey)
	if err != nil {
		return err
	}

	wgclient, err := wgctrl.New()
	if err != nil {
		return err
	}
	modcfg, err := config.ReadConfig(node.Network)
	if err != nil {
		return err
	}
	nodecfg := modcfg.Node
	servercfg := modcfg.Server

	if err != nil {
		log.Fatalf("failed to open client: %v", err)
	}
	defer wgclient.Close()

	var ifacename string
	if nodecfg.Interface != "" {
		ifacename = nodecfg.Interface
	} else if node.Interface != "" {
		ifacename = node.Interface
	} else {
		log.Fatal("no interface to configure")
	}
	if node.Address == "" {
		log.Fatal("no address to configure")
	}

	nameserver := servercfg.CoreDNSAddr
	network := node.Network
	if nodecfg.Network != "" {
		network = nodecfg.Network
	} else if node.Network != "" {
		network = node.Network
	}

	if ncutils.IsKernel() {
		setKernelDevice(ifacename, node.Address)
	}

	nodeport := int(node.ListenPort)
	conf := wgtypes.Config{}
	if nodecfg.UDPHolePunch == "yes" &&
		nodecfg.IsServer == "no" &&
		nodecfg.IsIngressGateway != "yes" &&
		nodecfg.IsStatic != "yes" {
		conf = wgtypes.Config{
			PrivateKey:   &key,
			ReplacePeers: true,
			Peers:        peers,
		}
	} else {
		conf = wgtypes.Config{
			PrivateKey:   &key,
			ListenPort:   &nodeport,
			ReplacePeers: true,
			Peers:        peers,
		}
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
		// spin up userspace / windows interface + apply the conf file
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
		err = ApplyConf(confPath)
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
				log.Fatalf("Unknown config error: %v", err)
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

		//=========DNS Setup==========\\
		if nodecfg.DNSOn == "yes" {
			_ = local.UpdateDNS(ifacename, network, nameserver)
		}
		//=========End DNS Setup=======\\
		if _, err := ncutils.RunCmd(ipExec+" link set down dev "+ifacename, false); err != nil {
			ncutils.Log("attempted to remove interface before editing")
			return err
		}

		if nodecfg.PostDown != "" {
			runcmds := strings.Split(nodecfg.PostDown, "; ")
			_ = ncutils.RunCmds(runcmds, true)
		}
		// set MTU of node interface
		if _, err := ncutils.RunCmd(ipExec+" link set mtu "+strconv.Itoa(int(nodecfg.MTU))+" up dev "+ifacename, true); err != nil {
			ncutils.Log("failed to create interface with mtu " + ifacename)
			return err
		}

		if nodecfg.PostUp != "" {
			runcmds := strings.Split(nodecfg.PostUp, "; ")
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

// SetWGConfig - sets the WireGuard Config of a given network and checks if it needs a peer update
func SetWGConfig(network string, peerupdate bool) error {

	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	servercfg := cfg.Server
	nodecfg := cfg.Node

	peers, hasGateway, gateways, err := server.GetPeers(nodecfg.MacAddress, nodecfg.Network, servercfg.GRPCAddress, nodecfg.IsDualStack == "yes", nodecfg.IsIngressGateway == "yes", nodecfg.IsServer == "yes")
	if err != nil {
		return err
	}
	privkey, err := RetrievePrivKey(network)
	if err != nil {
		return err
	}
	if peerupdate {
		var iface string
		iface = nodecfg.Interface
		if ncutils.IsMac() {
			iface, err = local.GetMacIface(nodecfg.Address)
			if err != nil {
				return err
			}
		}
		err = SetPeers(iface, nodecfg.PersistentKeepalive, peers)
	} else {
		err = InitWireguard(&nodecfg, privkey, peers, hasGateway, gateways)
	}
	return err
}

// RemoveConf - removes a configuration for a given WireGuard interface
func RemoveConf(iface string, printlog bool) error {
	os := runtime.GOOS
	var err error
	switch os {
	case "windows":
		err = RemoveWindowsConf(iface, printlog)
	default:
		confPath := ncutils.GetNetclientPathSpecific() + iface + ".conf"
		err = RemoveWGQuickConf(confPath, printlog)
	}
	return err
}

// ApplyConf - applys a conf on disk to WireGuard interface
func ApplyConf(confPath string) error {
	os := runtime.GOOS
	var err error
	switch os {
	case "windows":
		_ = ApplyWindowsConf(confPath)
	default:
		err = ApplyWGQuickConf(confPath)
	}
	return err
}
