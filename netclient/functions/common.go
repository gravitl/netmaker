package functions

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ListPorts - lists ports of WireGuard devices
func ListPorts() error {
	wgclient, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wgclient.Close()
	devices, err := wgclient.Devices()
	if err != nil {
		return err
	}
	fmt.Println("Here are your ports:")
	for _, i := range devices {
		fmt.Println(i.ListenPort)
	}
	return err
}

func getPrivateAddr() (string, error) {

	var local string
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()

		localAddr := conn.LocalAddr().(*net.UDPAddr)
		localIP := localAddr.IP
		local = localIP.String()
	}
	if local == "" {
		local, err = getPrivateAddrBackup()
	}

	if local == "" {
		err = errors.New("could not find local ip")
	}

	return local, err
}

func getPrivateAddrBackup() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	var local string
	found := false
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if i.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := i.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				if !found {
					ip = v.IP
					local = ip.String()
					found = true
				}
			case *net.IPAddr:
				if !found {
					ip = v.IP
					local = ip.String()
					found = true
				}
			}
		}
	}
	if !found {
		err := errors.New("local ip address not found")
		return "", err
	}
	return local, err
}

// GetNode - gets node locally
func GetNode(network string) models.Node {

	modcfg, err := config.ReadConfig(network)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	return modcfg.Node
}

// Uninstall - uninstalls networks from client
func Uninstall() error {
	networks, err := ncutils.GetSystemNetworks()
	if err != nil {
		ncutils.PrintLog("unable to retrieve networks: "+err.Error(), 1)
		ncutils.PrintLog("continuing uninstall without leaving networks", 1)
	} else {
		for _, network := range networks {
			err = LeaveNetwork(network, true)
			if err != nil {
				ncutils.PrintLog("Encounter issue leaving network "+network+": "+err.Error(), 1)
			}
		}
	}
	// clean up OS specific stuff
	if ncutils.IsWindows() {
		daemon.CleanupWindows()
	} else if ncutils.IsMac() {
		daemon.CleanupMac()
	} else if ncutils.IsLinux() {
		daemon.CleanupLinux()
	} else if ncutils.IsFreeBSD() {
		daemon.CleanupFreebsd()
	} else if !ncutils.IsKernel() {
		ncutils.PrintLog("manual cleanup required", 1)
	}

	return err
}

// LeaveNetwork - client exits a network
func LeaveNetwork(network string, force bool) error {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	servercfg := cfg.Server
	node := cfg.Node
	if node.NetworkSettings.IsComms == "yes" && !force {
		return errors.New("COMMS_NET - You are trying to leave the comms network. This will break network updates. Unless you re-join. If you really want to leave, run with --force=yes.")
	}

	if node.IsServer != "yes" {
		var wcclient nodepb.NodeServiceClient
		conn, err := grpc.Dial(cfg.Server.GRPCAddress,
			ncutils.GRPCRequestOpts(cfg.Server.GRPCSSL))
		if err != nil {
			log.Printf("Unable to establish client connection to "+servercfg.GRPCAddress+": %v", err)
		}
		defer conn.Close()
		wcclient = nodepb.NewNodeServiceClient(conn)

		ctx, err := auth.SetJWT(wcclient, network)
		if err != nil {
			log.Printf("Failed to authenticate: %v", err)
		} else { // handle client side
			var header metadata.MD
			nodeData, err := json.Marshal(&node)
			if err == nil {
				_, err = wcclient.DeleteNode(
					ctx,
					&nodepb.Object{
						Data: string(nodeData),
						Type: nodepb.NODE_TYPE,
					},
					grpc.Header(&header),
				)
				if err != nil {
					ncutils.PrintLog("encountered error deleting node: "+err.Error(), 1)
				} else {
					ncutils.PrintLog("removed machine from "+node.Network+" network on remote server", 1)
				}
			}
		}
	}

	wgClient, wgErr := wgctrl.New()
	if wgErr == nil {
		removeIface := cfg.Node.Interface
		if ncutils.IsMac() {
			var macIface string
			macIface, wgErr = local.GetMacIface(cfg.Node.Address)
			if wgErr == nil && removeIface != "" {
				removeIface = macIface
			}
			wgErr = nil
		}
		dev, devErr := wgClient.Device(removeIface)
		if devErr == nil {
			local.FlushPeerRoutes(removeIface, cfg.Node.Address, dev.Peers[:])
			_, cidr, cidrErr := net.ParseCIDR(cfg.NetworkSettings.AddressRange)
			if cidrErr == nil {
				local.RemoveCIDRRoute(removeIface, cfg.Node.Address, cidr)
			}
		} else {
			ncutils.PrintLog("could not flush peer routes when leaving network, "+cfg.Node.Network, 1)
		}
	}

	err = WipeLocal(node.Network)
	if err != nil {
		ncutils.PrintLog("unable to wipe local config", 1)
	} else {
		ncutils.PrintLog("removed "+node.Network+" network locally", 1)
	}

	currentNets, err := ncutils.GetSystemNetworks()
	if err != nil || len(currentNets) <= 1 {
		daemon.Stop() // stop system daemon if last network
		return RemoveLocalInstance(cfg, network)
	}
	return daemon.Restart()
}

// RemoveLocalInstance - remove all netclient files locally for a network
func RemoveLocalInstance(cfg *config.ClientConfig, networkName string) error {

	if cfg.Daemon != "off" {
		if ncutils.IsWindows() {
			// TODO: Remove job?
		} else if ncutils.IsMac() {
			//TODO: Delete mac daemon
		} else if ncutils.IsFreeBSD() {
			daemon.RemoveFreebsdDaemon()
		} else {
			daemon.RemoveSystemDServices()
		}
	}
	return nil
}

// DeleteInterface - delete an interface of a network
func DeleteInterface(ifacename string, postdown string) error {
	return wireguard.RemoveConf(ifacename, true)
}

// WipeLocal - wipes local instance
func WipeLocal(network string) error {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	nodecfg := cfg.Node
	ifacename := nodecfg.Interface
	if ifacename != "" {
		if err = wireguard.RemoveConf(ifacename, true); err == nil {
			ncutils.PrintLog("removed WireGuard interface: "+ifacename, 1)
		} else if strings.Contains(err.Error(), "does not exist") {
			err = nil
		}
	}

	home := ncutils.GetNetclientPathSpecific()
	if ncutils.FileExists(home + "netconfig-" + network) {
		err = os.Remove(home + "netconfig-" + network)
		if err != nil {
			log.Println("error removing netconfig:")
			log.Println(err.Error())
		}
	}
	if ncutils.FileExists(home + "backup.netconfig-" + network) {
		err = os.Remove(home + "backup.netconfig-" + network)
		if err != nil {
			log.Println("error removing backup netconfig:")
			log.Println(err.Error())
		}
	}
	if ncutils.FileExists(home + "nettoken-" + network) {
		err = os.Remove(home + "nettoken-" + network)
		if err != nil {
			log.Println("error removing nettoken:")
			log.Println(err.Error())
		}
	}
	if ncutils.FileExists(home + "secret-" + network) {
		err = os.Remove(home + "secret-" + network)
		if err != nil {
			log.Println("error removing secret:")
			log.Println(err.Error())
		}
	}
	if ncutils.FileExists(home + "traffic-" + network) {
		err = os.Remove(home + "traffic-" + network)
		if err != nil {
			log.Println("error removing traffic key:")
			log.Println(err.Error())
		}
	}
	if ncutils.FileExists(home + "wgkey-" + network) {
		err = os.Remove(home + "wgkey-" + network)
		if err != nil {
			log.Println("error removing wgkey:")
			log.Println(err.Error())
		}
	}
	if ncutils.FileExists(home + ifacename + ".conf") {
		err = os.Remove(home + ifacename + ".conf")
		if err != nil {
			log.Println("error removing .conf:")
			log.Println(err.Error())
		}
	}
	return err
}
