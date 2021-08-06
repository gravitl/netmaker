package wireguard

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/server"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	//homedir "github.com/mitchellh/go-homedir"
)

func InitGRPCWireguard(client models.IntClient) error {

	key, err := wgtypes.ParseKey(client.PrivateKey)
	if err != nil {
		return err
	}
	serverkey, err := wgtypes.ParseKey(client.ServerKey)
	if err != nil {
		return err
	}
	serverport, err := strconv.Atoi(client.ServerWGPort)
	if err != nil {
		return err
	}

	wgclient, err := wgctrl.New()
	if err != nil {
		log.Fatalf("failed to open client: %v", err)
	}
	defer wgclient.Close()

	ifacename := "grpc-wg-001"
	if client.Address6 == "" && client.Address == "" {
		return errors.New("no address to configure")
	}
	currentiface, err := net.InterfaceByName(ifacename)
	if err != nil {
		_, err = local.RunCmd("ip link add dev " + ifacename + " type wireguard")
		if err != nil && !strings.Contains(err.Error(), "exists") {
			log.Println("Error creating interface")
		}
	}
	match := false
	match6 := false
	addrs, _ := currentiface.Addrs()

	//Add IPv4Address (make into separate function)
	for _, a := range addrs {
		if strings.Contains(a.String(), client.Address) {
			match = true
		}
		if strings.Contains(a.String(), client.Address6) {
			match6 = true
		}
	}
	if !match && client.Address != "" {
		_, err = local.RunCmd("ip address add dev " + ifacename + " " + client.Address + "/24")
		if err != nil {
			log.Println("Error adding ipv4 address")
			fmt.Println(err)
		}
	}
	if !match6 && client.Address6 != "" {
		_, err = local.RunCmd("ip address add dev" + ifacename + " " + client.Address6 + "/64")
		if err != nil {
			log.Println("Error adding ipv6 address")
			fmt.Println(err)
		}
	}
	var peers []wgtypes.PeerConfig
	var peeraddr = net.IPNet{
		IP:   net.ParseIP(client.ServerPrivateAddress),
		Mask: net.CIDRMask(32, 32),
	}
	var allowedips []net.IPNet
	allowedips = append(allowedips, peeraddr)
	net.ParseIP(client.ServerPublicEndpoint)
	peer := wgtypes.PeerConfig{
		PublicKey: serverkey,
		Endpoint: &net.UDPAddr{
			IP:   net.ParseIP(client.ServerPublicEndpoint),
			Port: serverport,
		},
		ReplaceAllowedIPs: true,
		AllowedIPs:        allowedips,
	}
	peers = append(peers, peer)
	conf := wgtypes.Config{
		PrivateKey:   &key,
		ReplacePeers: true,
		Peers:        peers,
	}
	_, err = wgclient.Device(ifacename)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Device does not exist: ")
			log.Println(err)
		} else {
			return err
		}
	}
	err = wgclient.ConfigureDevice(ifacename, conf)

	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Device does not exist: ")
			log.Println(err)
		} else {
			log.Printf("This is inconvenient: %v", err)
		}
	}

	_, err = local.RunCmd("ip link set up dev " + ifacename)
	_, err = local.RunCmd("ip link set down dev " + ifacename)
	if err != nil {
		return err
	}

	return err
}

func InitWireguard(node *models.Node, privkey string, peers []wgtypes.PeerConfig, hasGateway bool, gateways []string) error {

	ipExec, err := exec.LookPath("ip")
	if err != nil {
		return err
	}
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

	ifacename := node.Interface
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

	_, delErr := local.RunCmd("ip link delete dev " + ifacename)
	_, addLinkErr := local.RunCmd(ipExec + " link add dev " + ifacename + " type wireguard")
	_, addErr := local.RunCmd(ipExec + " address add dev " + ifacename + " " + node.Address + "/24")
	if delErr != nil {
		log.Println(delErr)
	}
	if addLinkErr != nil {
		log.Println(addLinkErr)
	}
	if addErr != nil {
		log.Println(addErr)
	}
	var nodeport int
	nodeport = int(node.ListenPort)

	conf := wgtypes.Config{}
	if nodecfg.UDPHolePunch == "yes" && nodecfg.Name != "netmaker" {
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

	cmdIPLinkUp := &exec.Cmd{
		Path:   ipExec,
		Args:   []string{ipExec, "link", "set", "up", "dev", ifacename},
		Stdout: os.Stdout,
		Stderr: os.Stdout,
	}

	cmdIPLinkDown := &exec.Cmd{
		Path:   ipExec,
		Args:   []string{ipExec, "link", "set", "down", "dev", ifacename},
		Stdout: os.Stdout,
		Stderr: os.Stdout,
	}
	err = cmdIPLinkDown.Run()
	if nodecfg.PostDown != "" {
		runcmds := strings.Split(nodecfg.PostDown, "; ")
		err = local.RunCmds(runcmds)
		if err != nil {
			fmt.Println("Error encountered running PostDown: " + err.Error())
		}
	}

	err = cmdIPLinkUp.Run()
	if err != nil {
		return err
	}

	if nodecfg.PostUp != "" {
		runcmds := strings.Split(nodecfg.PostUp, "; ")
		err = local.RunCmds(runcmds)
		if err != nil {
			fmt.Println("Error encountered running PostUp: " + err.Error())
		}
	}
	if hasGateway {
		for _, gateway := range gateways {
			out, err := local.RunCmd(ipExec + " -4 route add " + gateway + " dev " + ifacename)
			fmt.Println(string(out))
			if err != nil {
				fmt.Println("Error encountered adding gateway: " + err.Error())
			}
		}
	}
	if node.Address6 != "" && node.IsDualStack == "yes" {
		fmt.Println("Adding address: " + node.Address6)
		out, err := local.RunCmd(ipExec + " address add dev " + ifacename + " " + node.Address6 + "/64")
		if err != nil {
			fmt.Println(out)
			fmt.Println("Error encountered adding ipv6: " + err.Error())
		}
	}

	return err
}

func SetWGKeyConfig(network string, serveraddr string) error {

	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}

	node := config.GetNode(network)

	privatekey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return err
	}
	privkeystring := privatekey.String()
	publickey := privatekey.PublicKey()

	node.PublicKey = publickey.String()

	err = StorePrivKey(privkeystring, network)
	if err != nil {
		return err
	}
	if node.Action == models.NODE_UPDATE_KEY {
		node.Action = models.NODE_NOOP
	}
	err = config.ModConfig(&node)
	if err != nil {
		return err
	}

	err = SetWGConfig(network, false)
	if err != nil {
		return err
	}

	return err
}

func SetWGConfig(network string, peerupdate bool) error {

	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	servercfg := cfg.Server
	nodecfg := cfg.Node
	node := config.GetNode(network)

	peers, hasGateway, gateways, err := server.GetPeers(node.MacAddress, nodecfg.Network, servercfg.GRPCAddress, node.IsDualStack == "yes", node.IsIngressGateway == "yes")
	if err != nil {
		return err
	}
	privkey, err := RetrievePrivKey(network)
	if err != nil {
		return err
	}
	if peerupdate {
		SetPeers(node.Interface, node.PersistentKeepalive, peers)
	} else {
		err = InitWireguard(&node, privkey, peers, hasGateway, gateways)
	}
	if err != nil {
		return err
	}

	return err
}

func SetPeers(iface string, keepalive int32, peers []wgtypes.PeerConfig) {
	client, err := wgctrl.New()
	if err != nil {
		log.Println("failed to start wgctrl")
		return
	}
	device, err := client.Device(iface)
	if err != nil {
		log.Println("failed to parse interface")
		return
	}
	for _, peer := range peers {

		for _, currentPeer := range device.Peers {
			if currentPeer.AllowedIPs[0].String() == peer.AllowedIPs[0].String() &&
				currentPeer.PublicKey.String() == peer.PublicKey.String() {
				_, err := local.RunCmd("wg set " + iface + " peer " + currentPeer.PublicKey.String() + " delete")
				if err != nil {
					log.Println("error setting peer", peer.Endpoint.String())
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

		_, err = local.RunCmd("wg set " + iface + " peer " + peer.PublicKey.String() +
			" endpoint " + udpendpoint +
			" persistent-keepalive " + string(keepalive) +
			" allowed-ips " + allowedips)
		if err != nil {
			log.Println("error setting peer", peer.Endpoint.String())
		}
	}
}

func StorePrivKey(key string, network string) error {
	d1 := []byte(key)
	err := ioutil.WriteFile("/etc/netclient/wgkey-"+network, d1, 0644)
	return err
}

func RetrievePrivKey(network string) (string, error) {
	dat, err := ioutil.ReadFile("/etc/netclient/wgkey-" + network)
	return string(dat), err
}
