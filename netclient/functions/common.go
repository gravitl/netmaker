package functions

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
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
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	localIP := localAddr.IP
	local = localIP.String()
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
			err = LeaveNetwork(network)
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
	} else if !ncutils.IsKernel() {
		ncutils.PrintLog("manual cleanup required", 1)
	}

	return err
}

// LeaveNetwork - client exits a network
func LeaveNetwork(network string) error {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	servercfg := cfg.Server
	node := cfg.Node

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
			_, err = wcclient.DeleteNode(
				ctx,
				&nodepb.Object{
					Data: node.ID,
					Type: nodepb.STRING_TYPE,
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
	//extra network route setting required for freebsd and windows
	if ncutils.IsWindows() {
		ip, mask, err := ncutils.GetNetworkIPMask(node.NetworkSettings.AddressRange)
		if err != nil {
			ncutils.PrintLog(err.Error(), 1)
		}
		_, _ = ncutils.RunCmd("route delete "+ip+" mask "+mask+" "+node.Address, true)
	} else if ncutils.IsFreeBSD() {
		_, _ = ncutils.RunCmd("route del -net "+node.NetworkSettings.AddressRange+" -interface "+node.Interface, true)
	} else if ncutils.IsLinux() {
		_, _ = ncutils.RunCmd("ip -4 route del "+node.NetworkSettings.AddressRange+" dev "+node.Interface, false)
	}
	return RemoveLocalInstance(cfg, network)
}

// RemoveLocalInstance - remove all netclient files locally for a network
func RemoveLocalInstance(cfg *config.ClientConfig, networkName string) error {
	err := WipeLocal(networkName)
	if err != nil {
		ncutils.PrintLog("unable to wipe local config", 1)
	} else {
		ncutils.PrintLog("removed "+networkName+" network locally", 1)
	}
	if cfg.Daemon != "off" {
		if ncutils.IsWindows() {
			// TODO: Remove job?
		} else if ncutils.IsMac() {
			//TODO: Delete mac daemon
		} else {
			err = daemon.RemoveSystemDServices()
		}
	}
	return err
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
		}
	}

	home := ncutils.GetNetclientPathSpecific()
	if ncutils.FileExists(home + "netconfig-" + network) {
		_ = os.Remove(home + "netconfig-" + network)
	}
	if ncutils.FileExists(home + "backup.netconfig-" + network) {
		_ = os.Remove(home + "backup.netconfig-" + network)
	}
	if ncutils.FileExists(home + "nettoken-" + network) {
		_ = os.Remove(home + "nettoken-" + network)
	}
	if ncutils.FileExists(home + "secret-" + network) {
		_ = os.Remove(home + "secret-" + network)
	}
	if ncutils.FileExists(home + "wgkey-" + network) {
		_ = os.Remove(home + "wgkey-" + network)
	}
	if ncutils.FileExists(home + "nm-" + network + ".conf") {
		_ = os.Remove(home + "nm-" + network + ".conf")
	}
	return err
}

func getLocalIP(node models.Node) string {

	var local string

	ifaces, err := net.Interfaces()
	if err != nil {
		return local
	}
	_, localrange, err := net.ParseCIDR(node.LocalRange)
	if err != nil {
		return local
	}

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
			return local
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				if !found {
					ip = v.IP
					local = ip.String()
					if node.IsLocal == "yes" {
						found = localrange.Contains(ip)
					} else {
						found = true
					}
				}
			case *net.IPAddr:
				if !found {
					ip = v.IP
					local = ip.String()
					if node.IsLocal == "yes" {
						found = localrange.Contains(ip)

					} else {
						found = true
					}
				}
			}
		}
	}
	return local
}
