package wireguard

import (
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gopkg.in/ini.v1"
)

const (
	section_interface = "Interface"
	section_peers     = "Peer"
)

// SetPeers - sets peers on a given WireGuard interface
func SetPeers(iface string, node *models.Node, peers []wgtypes.PeerConfig) error {
	var devicePeers []wgtypes.Peer
	var keepalive = node.PersistentKeepalive
	var oldPeerAllowedIps = make(map[string]bool, len(peers))
	var err error
	devicePeers, err = GetDevicePeers(iface)
	if err != nil {
		return err
	}

	if len(devicePeers) > 1 && len(peers) == 0 {
		logger.Log(1, "no peers pulled")
		return err
	}
	for _, peer := range peers {
		// make sure peer has AllowedIP's before comparison
		hasPeerIP := len(peer.AllowedIPs) > 0
		for _, currentPeer := range devicePeers {
			// make sure currenPeer has AllowedIP's before comparison
			hascurrentPeerIP := len(currentPeer.AllowedIPs) > 0

			if hasPeerIP && hascurrentPeerIP &&
				currentPeer.AllowedIPs[0].String() == peer.AllowedIPs[0].String() &&
				currentPeer.PublicKey.String() != peer.PublicKey.String() {
				_, err := ncutils.RunCmd("wg set "+iface+" peer "+currentPeer.PublicKey.String()+" remove", true)
				if err != nil {
					logger.Log(0, "error removing peer", peer.Endpoint.String())
				}
			}
		}
		udpendpoint := peer.Endpoint.String()
		var allowedips string
		var iparr []string
		for _, ipaddr := range peer.AllowedIPs {
			if hasPeerIP {
				iparr = append(iparr, ipaddr.String())
			}
		}
		if len(iparr) > 0 {
			allowedips = strings.Join(iparr, ",")
		}
		keepAliveString := strconv.Itoa(int(keepalive))
		if keepAliveString == "0" {
			keepAliveString = "15"
		}
		if node.IsServer == "yes" || peer.Endpoint == nil {
			_, err = ncutils.RunCmd("wg set "+iface+" peer "+peer.PublicKey.String()+
				" persistent-keepalive "+keepAliveString+
				" allowed-ips "+allowedips, true)
		} else {
			_, err = ncutils.RunCmd("wg set "+iface+" peer "+peer.PublicKey.String()+
				" endpoint "+udpendpoint+
				" persistent-keepalive "+keepAliveString+
				" allowed-ips "+allowedips, true)
		}
		if err != nil {
			logger.Log(0, "error setting peer", peer.PublicKey.String())
		}
	}
	if len(devicePeers) > 0 {
		for _, currentPeer := range devicePeers {
			shouldDelete := true
			if len(peers) > 0 {
				for _, peer := range peers {

					if len(peer.AllowedIPs) > 0 && len(currentPeer.AllowedIPs) > 0 &&
						peer.AllowedIPs[0].String() == currentPeer.AllowedIPs[0].String() {
						shouldDelete = false
					}
					// re-check this if logic is not working, added in case of allowedips not working
					if peer.PublicKey.String() == currentPeer.PublicKey.String() {
						shouldDelete = false
					}
				}
				if shouldDelete {
					output, err := ncutils.RunCmd("wg set "+iface+" peer "+currentPeer.PublicKey.String()+" remove", true)
					if err != nil {
						logger.Log(0, output, "error removing peer", currentPeer.PublicKey.String())
					}
				}
				for _, ip := range currentPeer.AllowedIPs {
					oldPeerAllowedIps[ip.String()] = true
				}
			}
		}
	}
	if ncutils.IsMac() {
		err = SetMacPeerRoutes(iface)
		return err
	} else if ncutils.IsLinux() {
		if len(peers) > 0 {
			local.SetPeerRoutes(iface, oldPeerAllowedIps, peers)
		}
	}

	return nil
}

// Initializes a WireGuard interface
func InitWireguard(node *models.Node, privkey string, peers []wgtypes.PeerConfig) error {

	key, err := wgtypes.ParseKey(privkey)
	if err != nil {
		return err
	}

	wgclient, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wgclient.Close()
	//nodecfg := modcfg.Node
	var ifacename string
	if node.Interface != "" {
		ifacename = node.Interface
	} else {
		return fmt.Errorf("no interface to configure")
	}
	if node.PrimaryAddress() == "" {
		return fmt.Errorf("no address to configure")
	}
	if err := WriteWgConfig(node, key.String(), peers); err != nil {
		logger.Log(1, "error writing wg conf file: ", err.Error())
		return err
	}
	// spin up userspace / windows interface + apply the conf file
	confPath := ncutils.GetNetclientPathSpecific() + ifacename + ".conf"
	var deviceiface = ifacename
	var mErr error
	if ncutils.IsMac() { // if node is Mac (Darwin) get the tunnel name first
		deviceiface, mErr = local.GetMacIface(node.PrimaryAddress())
		if mErr != nil || deviceiface == "" {
			deviceiface = ifacename
		}
	}
	// ensure you clear any existing interface first
	RemoveConfGraceful(deviceiface)
	ApplyConf(node, ifacename, confPath)      // Apply initially
	logger.Log(1, "waiting for interface...") // ensure interface is created
	output, _ := ncutils.RunCmd("wg", false)
	starttime := time.Now()
	ifaceReady := strings.Contains(output, deviceiface)
	for !ifaceReady && !(time.Now().After(starttime.Add(time.Second << 4))) {
		if ncutils.IsMac() { // if node is Mac (Darwin) get the tunnel name first
			deviceiface, mErr = local.GetMacIface(node.PrimaryAddress())
			if mErr != nil || deviceiface == "" {
				deviceiface = ifacename
			}
		}
		output, _ = ncutils.RunCmd("wg", false)
		err = ApplyConf(node, node.Interface, confPath)
		time.Sleep(time.Second)
		ifaceReady = strings.Contains(output, deviceiface)
	}
	//wgclient does not work well on freebsd
	if node.OS == "freebsd" {
		if !ifaceReady {
			return fmt.Errorf("could not reliably create interface, please check wg installation and retry")
		}
	} else {
		_, devErr := wgclient.Device(deviceiface)
		if !ifaceReady || devErr != nil {
			fmt.Printf("%v\n", devErr)
			return fmt.Errorf("could not reliably create interface, please check wg installation and retry")
		}
	}
	logger.Log(1, "interface ready - netclient.. ENGAGE")

	if !ncutils.HasWgQuick() && ncutils.IsLinux() {
		err = SetPeers(ifacename, node, peers)
		if err != nil {
			logger.Log(1, "error setting peers: ", err.Error())
		}

		time.Sleep(time.Second)
	}

	//ipv4
	if node.Address != "" {
		_, cidr, cidrErr := net.ParseCIDR(node.NetworkSettings.AddressRange)
		if cidrErr == nil {
			local.SetCIDRRoute(ifacename, node.Address, cidr)
		} else {
			logger.Log(1, "could not set cidr route properly: ", cidrErr.Error())
		}
		local.SetCurrentPeerRoutes(ifacename, node.Address, peers)
	}
	if node.Address6 != "" {
		//ipv6
		_, cidr, cidrErr := net.ParseCIDR(node.NetworkSettings.AddressRange6)
		if cidrErr == nil {
			local.SetCIDRRoute(ifacename, node.Address6, cidr)
		} else {
			logger.Log(1, "could not set cidr route properly: ", cidrErr.Error())
		}
		local.SetCurrentPeerRoutes(ifacename, node.Address6, peers)
	}
	return err
}

// SetWGConfig - sets the WireGuard Config of a given network and checks if it needs a peer update
func SetWGConfig(network string, peerupdate bool, peers []wgtypes.PeerConfig) error {

	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	privkey, err := RetrievePrivKey(network)
	if err != nil {
		return err
	}
	if peerupdate && !ncutils.IsFreeBSD() && !(ncutils.IsLinux() && !ncutils.IsKernel()) {
		var iface string
		iface = cfg.Node.Interface
		if ncutils.IsMac() {
			iface, err = local.GetMacIface(cfg.Node.PrimaryAddress())
			if err != nil {
				return err
			}
		}
		err = SetPeers(iface, &cfg.Node, peers)
	} else {
		err = InitWireguard(&cfg.Node, privkey, peers)
	}
	return err
}

// RemoveConf - removes a configuration for a given WireGuard interface
func RemoveConf(iface string, printlog bool) error {
	os := runtime.GOOS
	if ncutils.IsLinux() && !ncutils.HasWgQuick() {
		os = "nowgquick"
	}
	var err error
	switch os {
	case "nowgquick":
		err = RemoveWithoutWGQuick(iface)
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
func ApplyConf(node *models.Node, ifacename string, confPath string) error {
	os := runtime.GOOS
	if ncutils.IsLinux() && !ncutils.HasWgQuick() {
		os = "nowgquick"
	}
	var isConnected = node.Connected != "no"
	var err error
	switch os {
	case "windows":
		ApplyWindowsConf(confPath, isConnected)
	case "darwin":
		ApplyMacOSConf(node, ifacename, confPath, isConnected)
	case "nowgquick":
		ApplyWithoutWGQuick(node, ifacename, confPath, isConnected)
	default:
		ApplyWGQuickConf(confPath, ifacename, isConnected)
	}

	var nodeCfg config.ClientConfig
	nodeCfg.Network = node.Network
	if !(node.IsServer == "yes") {
		nodeCfg.ReadConfig()
		if nodeCfg.NetworkSettings.AddressRange != "" {
			ip, cidr, err := net.ParseCIDR(nodeCfg.NetworkSettings.AddressRange)
			if err == nil {
				local.SetCIDRRoute(node.Interface, ip.String(), cidr)
			}
		}
		if nodeCfg.NetworkSettings.AddressRange6 != "" {
			ip, cidr, err := net.ParseCIDR(nodeCfg.NetworkSettings.AddressRange6)
			if err == nil {
				local.SetCIDRRoute(node.Interface, ip.String(), cidr)
			}
		}
	}
	return err
}

// WriteWgConfig - creates a wireguard config file
func WriteWgConfig(node *models.Node, privateKey string, peers []wgtypes.PeerConfig) error {
	options := ini.LoadOptions{
		AllowNonUniqueSections: true,
		AllowShadows:           true,
	}
	wireguard := ini.Empty(options)
	wireguard.Section(section_interface).Key("PrivateKey").SetValue(privateKey)
	if node.ListenPort > 0 && node.UDPHolePunch != "yes" {
		wireguard.Section(section_interface).Key("ListenPort").SetValue(strconv.Itoa(int(node.ListenPort)))
	}
	addrString := node.Address
	if node.Address6 != "" {
		if addrString != "" {
			addrString += ","
		}
		addrString += node.Address6
	}
	wireguard.Section(section_interface).Key("Address").SetValue(addrString)
	// need to figure out DNS
	//if node.DNSOn == "yes" {
	//	wireguard.Section(section_interface).Key("DNS").SetValue(cfg.Server.CoreDNSAddr)
	//}
	//need to split postup/postdown because ini lib adds a ` and the ` breaks freebsd
	if node.PostUp != "" {
		parts := strings.Split(node.PostUp, " ; ")
		for i, part := range parts {
			if i == 0 {
				wireguard.Section(section_interface).Key("PostUp").SetValue(part)
			}
			wireguard.Section(section_interface).Key("PostUp").AddShadow(part)
		}
	}
	if node.PostDown != "" {
		parts := strings.Split(node.PostDown, " ; ")
		for i, part := range parts {
			if i == 0 {
				wireguard.Section(section_interface).Key("PostDown").SetValue(part)
			}
			wireguard.Section(section_interface).Key("PostDown").AddShadow(part)
		}
	}
	if node.MTU != 0 {
		wireguard.Section(section_interface).Key("MTU").SetValue(strconv.FormatInt(int64(node.MTU), 10))
	}
	for i, peer := range peers {
		wireguard.SectionWithIndex(section_peers, i).Key("PublicKey").SetValue(peer.PublicKey.String())
		if peer.PresharedKey != nil {
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

		if peer.PersistentKeepaliveInterval != nil && peer.PersistentKeepaliveInterval.Seconds() > 0 {
			wireguard.SectionWithIndex(section_peers, i).Key("PersistentKeepalive").SetValue(strconv.FormatInt((int64)(peer.PersistentKeepaliveInterval.Seconds()), 10))
		}
	}
	if err := wireguard.SaveTo(ncutils.GetNetclientPathSpecific() + node.Interface + ".conf"); err != nil {
		return err
	}
	return nil
}

// UpdateWgPeers - updates the peers of a network
func UpdateWgPeers(file string, peers []wgtypes.PeerConfig) (*net.UDPAddr, error) {
	var internetGateway *net.UDPAddr
	options := ini.LoadOptions{
		AllowNonUniqueSections: true,
		AllowShadows:           true,
	}
	wireguard, err := ini.LoadSources(options, file)
	if err != nil {
		return internetGateway, err
	}
	//delete the peers sections as they are going to be replaced
	wireguard.DeleteSection(section_peers)
	for i, peer := range peers {
		wireguard.SectionWithIndex(section_peers, i).Key("PublicKey").SetValue(peer.PublicKey.String())
		if peer.PresharedKey != nil {
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
			if strings.Contains(allowedIPs, "0.0.0.0/0") || strings.Contains(allowedIPs, "::/0") {
				internetGateway = peer.Endpoint
			}
		}
		if peer.Endpoint != nil {
			wireguard.SectionWithIndex(section_peers, i).Key("Endpoint").SetValue(peer.Endpoint.String())
		}
		if peer.PersistentKeepaliveInterval != nil && peer.PersistentKeepaliveInterval.Seconds() > 0 {
			wireguard.SectionWithIndex(section_peers, i).Key("PersistentKeepalive").SetValue(strconv.FormatInt((int64)(peer.PersistentKeepaliveInterval.Seconds()), 10))
		}
	}
	if err := wireguard.SaveTo(file); err != nil {
		return internetGateway, err
	}
	return internetGateway, nil
}

// UpdateWgInterface - updates the interface section of a wireguard config file
func UpdateWgInterface(file, privateKey, nameserver string, node models.Node) error {
	options := ini.LoadOptions{
		AllowNonUniqueSections: true,
		AllowShadows:           true,
	}
	wireguard, err := ini.LoadSources(options, file)
	if err != nil {
		return err
	}
	if node.UDPHolePunch == "yes" {
		node.ListenPort = 0
	}
	wireguard.DeleteSection(section_interface)
	wireguard.Section(section_interface).Key("PrivateKey").SetValue(privateKey)
	wireguard.Section(section_interface).Key("ListenPort").SetValue(strconv.Itoa(int(node.ListenPort)))
	addrString := node.Address
	if node.Address6 != "" {
		if addrString != "" {
			addrString += ","
		}
		addrString += node.Address6
	}
	wireguard.Section(section_interface).Key("Address").SetValue(addrString)
	//if node.DNSOn == "yes" {
	//	wireguard.Section(section_interface).Key("DNS").SetValue(nameserver)
	//}
	//need to split postup/postdown because ini lib adds a quotes which breaks freebsd
	if node.PostUp != "" {
		parts := strings.Split(node.PostUp, " ; ")
		for i, part := range parts {
			if i == 0 {
				wireguard.Section(section_interface).Key("PostUp").SetValue(part)
			}
			wireguard.Section(section_interface).Key("PostUp").AddShadow(part)
		}
	}
	if node.PostDown != "" {
		parts := strings.Split(node.PostDown, " ; ")
		for i, part := range parts {
			if i == 0 {
				wireguard.Section(section_interface).Key("PostDown").SetValue(part)
			}
			wireguard.Section(section_interface).Key("PostDown").AddShadow(part)
		}
	}
	if node.MTU != 0 {
		wireguard.Section(section_interface).Key("MTU").SetValue(strconv.FormatInt(int64(node.MTU), 10))
	}
	if err := wireguard.SaveTo(file); err != nil {
		return err
	}
	return nil
}

// UpdatePrivateKey - updates the private key of a wireguard config file
func UpdatePrivateKey(file, privateKey string) error {
	options := ini.LoadOptions{
		AllowNonUniqueSections: true,
		AllowShadows:           true,
	}
	wireguard, err := ini.LoadSources(options, file)
	if err != nil {
		return err
	}
	wireguard.Section(section_interface).Key("PrivateKey").SetValue(privateKey)
	if err := wireguard.SaveTo(file); err != nil {
		return err
	}
	return nil
}

// UpdateKeepAlive - updates the persistentkeepalive of all peers
func UpdateKeepAlive(file string, keepalive int32) error {
	options := ini.LoadOptions{
		AllowNonUniqueSections: true,
		AllowShadows:           true,
	}
	wireguard, err := ini.LoadSources(options, file)
	if err != nil {
		return err
	}
	peers, err := wireguard.SectionsByName(section_peers)
	if err != nil {
		return err
	}
	newvalue := strconv.Itoa(int(keepalive))
	for i := range peers {
		wireguard.SectionWithIndex(section_peers, i).Key("PersistentKeepALive").SetValue(newvalue)
	}
	if err := wireguard.SaveTo(file); err != nil {
		return err
	}
	return nil
}

// RemoveConfGraceful - Run remove conf and wait for it to actually be gone before proceeding
func RemoveConfGraceful(ifacename string) {
	// ensure you clear any existing interface first
	wgclient, err := wgctrl.New()
	if err != nil {
		logger.Log(0, "could not create wgclient")
		return
	}
	defer wgclient.Close()
	d, _ := wgclient.Device(ifacename)
	startTime := time.Now()
	for d != nil && d.Name == ifacename {
		if err = RemoveConf(ifacename, false); err != nil { // remove interface first
			if strings.Contains(err.Error(), "does not exist") {
				err = nil
				break
			}
		}
		time.Sleep(time.Second >> 2)
		d, _ = wgclient.Device(ifacename)
		if time.Now().After(startTime.Add(time.Second << 4)) {
			break
		}
	}
	time.Sleep(time.Second << 1)
}

// GetDevicePeers - gets the current device's peers
func GetDevicePeers(iface string) ([]wgtypes.Peer, error) {
	if ncutils.IsFreeBSD() {
		if devicePeers, err := ncutils.GetPeers(iface); err != nil {
			return nil, err
		} else {
			return devicePeers, nil
		}
	} else {
		client, err := wgctrl.New()
		if err != nil {
			logger.Log(0, "failed to start wgctrl")
			return nil, err
		}
		defer client.Close()
		device, err := client.Device(iface)
		if err != nil {
			logger.Log(0, "failed to parse interface", iface)
			return nil, err
		}
		return device.Peers, nil
	}
}
