package wireguard

import (
	"errors"
	"log"
	"os"
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
	"gopkg.in/ini.v1"
)

const (
	section_interface = "Interface"
	section_peers     = "Peer"
)

// SetPeers - sets peers on a given WireGuard interface
func SetPeers(iface string, keepalive int32, peers []wgtypes.PeerConfig) error {

	var devicePeers []wgtypes.Peer
	var err error
	if ncutils.IsFreeBSD() {
		if devicePeers, err = ncutils.GetPeers(iface); err != nil {
			return err
		}
	} else {
		client, err := wgctrl.New()
		if err != nil {
			ncutils.PrintLog("failed to start wgctrl", 0)
			return err
		}
		defer client.Close()
		device, err := client.Device(iface)
		if err != nil {
			ncutils.PrintLog("failed to parse interface", 0)
			return err
		}
		devicePeers = device.Peers
	}
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
			keepAliveString = "15"
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
	if ncutils.IsMac() {
		err = SetMacPeerRoutes(iface)
		return err
	}

	return nil
}

// Initializes a WireGuard interface
func InitWireguard(node *models.Node, privkey string, peers []wgtypes.PeerConfig, hasGateway bool, gateways []string, syncconf bool) error {

	key, err := wgtypes.ParseKey(privkey)
	if err != nil {
		return err
	}

	wgclient, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wgclient.Close()
	modcfg, err := config.ReadConfig(node.Network)
	if err != nil {
		return err
	}
	nodecfg := modcfg.Node

	if err != nil {
		log.Fatalf("failed to open client: %v", err)
	}

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
	var newConf string
	if node.UDPHolePunch != "yes" {
		newConf, _ = ncutils.CreateWireGuardConf(node, key.String(), strconv.FormatInt(int64(node.ListenPort), 10), peers)
	} else {
		newConf, _ = ncutils.CreateWireGuardConf(node, key.String(), "", peers)
	}
	confPath := ncutils.GetNetclientPathSpecific() + ifacename + ".conf"
	ncutils.PrintLog("writing wg conf file to: "+confPath, 1)
	err = os.WriteFile(confPath, []byte(newConf), 0644)
	if err != nil {
		ncutils.PrintLog("error writing wg conf file to "+confPath+": "+err.Error(), 1)
		return err
	}
	if ncutils.IsWindows() {
		wgConfPath := ncutils.GetWGPathSpecific() + ifacename + ".conf"
		err = os.WriteFile(wgConfPath, []byte(newConf), 0644)
		if err != nil {
			ncutils.PrintLog("error writing wg conf file to "+wgConfPath+": "+err.Error(), 1)
			return err
		}
		confPath = wgConfPath
	}
	// spin up userspace / windows interface + apply the conf file
	var deviceiface string
	if ncutils.IsMac() {
		deviceiface, err = local.GetMacIface(node.Address)
		if err != nil || deviceiface == "" {
			deviceiface = ifacename
		}
	}
	if syncconf {
		err = SyncWGQuickConf(ifacename, confPath)
	} else {
		if !ncutils.IsMac() {
			d, _ := wgclient.Device(deviceiface)
			for d != nil && d.Name == deviceiface {
				RemoveConf(ifacename, false) // remove interface first
				time.Sleep(time.Second >> 2)
				d, _ = wgclient.Device(deviceiface)
			}
		}
		if !ncutils.IsWindows() {
			err = ApplyConf(*node, ifacename, confPath)
			if err != nil {
				ncutils.PrintLog("failed to create wireguard interface", 1)
				return err
			}
		} else {
			var output string
			starttime := time.Now()
			RemoveConf(ifacename, false)
			time.Sleep(time.Second >> 2)
			ncutils.PrintLog("waiting for interface...", 1)
			for !strings.Contains(output, ifacename) && !(time.Now().After(starttime.Add(time.Duration(10) * time.Second))) {
				output, _ = ncutils.RunCmd("wg", false)
				err = ApplyConf(*node, ifacename, confPath)
				time.Sleep(time.Second)
			}
			if !strings.Contains(output, ifacename) {
				return errors.New("could not create wg interface for " + ifacename)
			}
			ip, mask, err := ncutils.GetNetworkIPMask(nodecfg.NetworkSettings.AddressRange)
			if err != nil {
				log.Println(err.Error())
				return err
			}
			ncutils.RunCmd("route add "+ip+" mask "+mask+" "+node.Address, true)
			time.Sleep(time.Second >> 2)
			ncutils.RunCmd("route change "+ip+" mask "+mask+" "+node.Address, true)
		}
	}

	//extra network route setting
	if ncutils.IsFreeBSD() {
		_, _ = ncutils.RunCmd("route add -net "+nodecfg.NetworkSettings.AddressRange+" -interface "+ifacename, true)
	} else if ncutils.IsLinux() {
		_, _ = ncutils.RunCmd("ip -4 route add "+nodecfg.NetworkSettings.AddressRange+" dev "+ifacename, false)
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
	if peerupdate && !ncutils.IsFreeBSD() && !(ncutils.IsLinux() && !ncutils.IsKernel()) {
		var iface string
		iface = nodecfg.Interface
		if ncutils.IsMac() {
			iface, err = local.GetMacIface(nodecfg.Address)
			if err != nil {
				return err
			}
		}
		err = SetPeers(iface, nodecfg.PersistentKeepalive, peers)
	} else if peerupdate {
		err = InitWireguard(&nodecfg, privkey, peers, hasGateway, gateways, true)
	} else {
		err = InitWireguard(&nodecfg, privkey, peers, hasGateway, gateways, false)
	}
	if nodecfg.DNSOn == "yes" {
		_ = local.UpdateDNS(nodecfg.Interface, nodecfg.Network, servercfg.CoreDNSAddr)
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
	case "darwin":
		err = RemoveConfMac(iface)
	default:
		confPath := ncutils.GetNetclientPathSpecific() + iface + ".conf"
		err = RemoveWGQuickConf(confPath, printlog)
	}
	return err
}

// ApplyConf - applys a conf on disk to WireGuard interface
func ApplyConf(node models.Node, ifacename string, confPath string) error {
	os := runtime.GOOS
	var err error
	switch os {
	case "windows":
		_ = ApplyWindowsConf(confPath)
	case "darwin":
		_ = ApplyMacOSConf(node, ifacename, confPath)
	default:
		err = ApplyWGQuickConf(confPath)
	}
	return err
}

// WriteWgConfig - creates a wireguard config file
func WriteWgConfig(cfg config.ClientConfig, privateKey string, peers []wgtypes.Peer) error {
	options := ini.LoadOptions{
		AllowNonUniqueSections: true,
		AllowShadows:           true,
	}
	wireguard := ini.Empty(options)
	wireguard.Section(section_interface).Key("PrivateKey").SetValue(privateKey)
	wireguard.Section(section_interface).Key("ListenPort").SetValue(strconv.Itoa(int(cfg.Node.ListenPort)))
	if cfg.Node.Address != "" {
		wireguard.Section(section_interface).Key("Address").SetValue(cfg.Node.Address)
	}
	if cfg.Node.Address6 != "" {
		wireguard.Section(section_interface).Key("Address").SetValue(cfg.Node.Address6)
	}
	if cfg.Node.DNSOn == "yes" {
		wireguard.Section(section_interface).Key("DNS").SetValue(cfg.Server.CoreDNSAddr)
	}
	if cfg.Node.PostUp != "" {
		wireguard.Section(section_interface).Key("PostUp").SetValue(cfg.Node.PostUp)
	}
	if cfg.Node.PostDown != "" {
		wireguard.Section(section_interface).Key("PostDown").SetValue(cfg.Node.PostDown)
	}
	for i, peer := range peers {
		wireguard.SectionWithIndex(section_peers, i).Key("PublicKey").SetValue(peer.PublicKey.String())
		if peer.PresharedKey.String() != "" {
			wireguard.SectionWithIndex(section_peers, i).Key("PreSharedKey").SetValue(peer.PresharedKey.String())
		}
		if peer.AllowedIPs != nil {
			var allowedIPs string
			for i, ip := range peer.AllowedIPs {
				if i == 0 {
					allowedIPs = ip.String()
				} else {
					allowedIPs = allowedIPs + ", " + ip.String()
				}
			}
			wireguard.SectionWithIndex(section_peers, i).Key("AllowedIps").SetValue(allowedIPs)
		}
		if peer.Endpoint != nil {
			wireguard.SectionWithIndex(section_peers, i).Key("Endpoint").SetValue(peer.Endpoint.String())
		}
	}
	if err := wireguard.SaveTo(ncutils.GetNetclientPathSpecific() + cfg.Node.Interface + ".conf"); err != nil {
		return err
	}
	return nil
}

// UpdateWgPeers - updates the peers of a network
func UpdateWgPeers(wgInterface string, peers []wgtypes.PeerConfig) error {
	file := ncutils.GetNetclientPathSpecific() + wgInterface + ".conf"
	ncutils.Log("updating " + file)
	wireguard, err := ini.ShadowLoad(file)
	if err != nil {
		return err
	}
	//delete the peers sections as they are going to be replaced
	wireguard.DeleteSection(section_peers)
	for i, peer := range peers {
		wireguard.SectionWithIndex(section_peers, i).Key("PublicKey").SetValue(peer.PublicKey.String())
		//if peer.PresharedKey.String() != "" {
		//wireguard.SectionWithIndex(section_peers, i).Key("PreSharedKey").SetValue(peer.PresharedKey.String())
		//}
		if peer.AllowedIPs != nil {
			var allowedIPs string
			for i, ip := range peer.AllowedIPs {
				if i == 0 {
					allowedIPs = ip.String()
				} else {
					allowedIPs = allowedIPs + ", " + ip.String()
				}
			}
			wireguard.SectionWithIndex(section_peers, i).Key("AllowedIps").SetValue(allowedIPs)
		}
		if peer.Endpoint != nil {
			wireguard.SectionWithIndex(section_peers, i).Key("Endpoint").SetValue(peer.Endpoint.String())
		}
	}
	if err := wireguard.SaveTo(file); err != nil {
		return err
	}
	return nil
}

// UpdateWgInterface - updates the interface section of a wireguard config file
func UpdateWgInterface(wgInterface, privateKey, nameserver string, node models.Node) error {
	//update to get path properly
	file := ncutils.GetNetclientPathSpecific() + wgInterface + ".conf"
	wireguard, err := ini.ShadowLoad(file)
	if err != nil {
		return err
	}
	wireguard.Section(section_interface).Key("PrivateKey").SetValue(privateKey)
	wireguard.Section(section_interface).Key("ListenPort").SetValue(strconv.Itoa(int(node.ListenPort)))
	if node.Address != "" {
		wireguard.Section(section_interface).Key("Address").SetValue(node.Address)
	}
	if node.Address6 != "" {
		wireguard.Section(section_interface).Key("Address").SetValue(node.Address6)
	}
	if node.DNSOn == "yes" {
		wireguard.Section(section_interface).Key("DNS").SetValue(nameserver)
	}
	if node.PostUp != "" {
		wireguard.Section(section_interface).Key("PostUp").SetValue(node.PostUp)
	}
	if node.PostDown != "" {
		wireguard.Section(section_interface).Key("PostDown").SetValue(node.PostDown)
	}
	if err := wireguard.SaveTo(file); err != nil {
		return err
	}
	return nil
}

// UpdatePrivateKey - updates the private key of a wireguard config file
func UpdatePrivateKey(wgInterface, privateKey string) error {
	//update to get path properly
	file := ncutils.GetNetclientPathSpecific() + wgInterface + ".conf"
	wireguard, err := ini.ShadowLoad(file)
	if err != nil {
		return err
	}
	wireguard.Section(section_interface).Key("PrivateKey").SetValue(privateKey)
	if err := wireguard.SaveTo(file); err != nil {
		return err
	}
	return nil
}
