package wireguard

import (
	"fmt"
	"io/ioutil"
	"log"
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
	addLinkOut, addLinkErr := local.RunCmd(ipExec + " link add dev " + ifacename + " type wireguard")
	addOut, addErr := local.RunCmd(ipExec + " address add dev " + ifacename + " " + node.Address + "/24")
	if delErr != nil {
		// not displaying error
		// log.Println(delOut, delErr)
	}
	if addLinkErr != nil {
		log.Println(addLinkOut, addLinkErr)
	}
	if addErr != nil {
		log.Println(addOut, addErr)
	}
	var nodeport int
	nodeport = int(node.ListenPort)

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
	if ipLinkDownOut, err := local.RunCmd(ipExec + " link set down dev " + ifacename); err != nil {
		log.Println(ipLinkDownOut, err)
		return err
	}

	if nodecfg.PostDown != "" {
		runcmds := strings.Split(nodecfg.PostDown, "; ")
		err = local.RunCmds(runcmds)
		if err != nil {
			fmt.Println("Error encountered running PostDown: " + err.Error())
		}
	}

	if ipLinkUpOut, err := local.RunCmd(ipExec + " link set up dev " + ifacename); err != nil {
		log.Println(ipLinkUpOut, err)
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
				fmt.Println("error encountered adding gateway: " + err.Error())
			}
		}
	}
	if node.Address6 != "" && node.IsDualStack == "yes" {
		fmt.Println("adding address: " + node.Address6)
		out, err := local.RunCmd(ipExec + " address add dev " + ifacename + " " + node.Address6 + "/64")
		if err != nil {
			fmt.Println(out)
			fmt.Println("error encountered adding ipv6: " + err.Error())
		}
	}

	return err
}

func SetWGKeyConfig(network string, serveraddr string) error {

	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}

	node := cfg.Node

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

	peers, hasGateway, gateways, err := server.GetPeers(nodecfg.MacAddress, nodecfg.Network, servercfg.GRPCAddress, nodecfg.IsDualStack == "yes", nodecfg.IsIngressGateway == "yes")
	if err != nil {
		return err
	}
	privkey, err := RetrievePrivKey(network)
	if err != nil {
		return err
	}
	if peerupdate {
		err = SetPeers(nodecfg.Interface, nodecfg.PersistentKeepalive, peers)
	} else {
		err = InitWireguard(&nodecfg, privkey, peers, hasGateway, gateways)
	}
	if err != nil {
		return err
	}

	return err
}

func SetPeers(iface string, keepalive int32, peers []wgtypes.PeerConfig) error {

	client, err := wgctrl.New()
	if err != nil {
		log.Println("failed to start wgctrl")
		return err
	}
	device, err := client.Device(iface)
	if err != nil {
		log.Println("failed to parse interface")
		return err
	}
	devicePeers := device.Peers
	if len(devicePeers) > 1 && len(peers) == 0 {
		log.Println("no peers pulled")
		return err
	}

	for _, peer := range peers {

		for _, currentPeer := range devicePeers {
			if currentPeer.AllowedIPs[0].String() == peer.AllowedIPs[0].String() &&
				currentPeer.PublicKey.String() != peer.PublicKey.String() {
				output, err := local.RunCmd("wg set " + iface + " peer " + currentPeer.PublicKey.String() + " remove")
				if err != nil {
					log.Println(output, "error removing peer", peer.Endpoint.String())
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
		var output string
		if peer.Endpoint != nil {
			output, err = local.RunCmd("wg set " + iface + " peer " + peer.PublicKey.String() +
				" endpoint " + udpendpoint +
				" persistent-keepalive " + keepAliveString +
				" allowed-ips " + allowedips)
		} else {
			output, err = local.RunCmd("wg set " + iface + " peer " + peer.PublicKey.String() +
				" persistent-keepalive " + keepAliveString +
				" allowed-ips " + allowedips)
		}
		if err != nil {
			log.Println(output, "error setting peer", peer.PublicKey.String(), err)
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
			output, err := local.RunCmd("wg set " + iface + " peer " + currentPeer.PublicKey.String() + " remove")
			if err != nil {
				log.Println(output, "error removing peer", currentPeer.PublicKey.String())
			} else {
				log.Println("removed peer " + currentPeer.PublicKey.String())
			}
		}
	}

	return nil
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
