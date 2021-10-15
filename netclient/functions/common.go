package functions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/logic"
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

var (
	wcclient nodepb.NodeServiceClient
)

// ListPorts - lists ports of WireGuard devices
func ListPorts() error {
	wgclient, err := wgctrl.New()
	if err != nil {
		return err
	}
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
		err := errors.New("Local Address Not Found.")
		return "", err
	}
	return local, err
}

func needInterfaceUpdate(ctx context.Context, mac string, network string, iface string) (bool, string, error) {
	var header metadata.MD
	req := &nodepb.Object{
		Data: mac + "###" + network,
		Type: nodepb.STRING_TYPE,
	}
	readres, err := wcclient.ReadNode(ctx, req, grpc.Header(&header))
	if err != nil {
		return false, "", err
	}
	var resNode models.Node
	if err := json.Unmarshal([]byte(readres.Data), &resNode); err != nil {
		return false, iface, err
	}
	oldiface := resNode.Interface

	return iface != oldiface, oldiface, err
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
			node.SetID()
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
	} else { // handle server side
		node.SetID()
		if err = logic.DeleteNode(node.ID, true); err != nil {
			ncutils.PrintLog("error removing server on network "+node.Network, 1)
		} else {
			ncutils.PrintLog("removed netmaker server instance on  "+node.Network, 1)
		}
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
			err = daemon.RemoveSystemDServices(networkName)
		}
	}
	return err
}

// DeleteInterface - delete an interface of a network
func DeleteInterface(ifacename string, postdown string) error {
	var err error
	if !ncutils.IsKernel() {
		err = wireguard.RemoveConf(ifacename, true)
	} else {
		ipExec, errN := exec.LookPath("ip")
		err = errN
		if err != nil {
			ncutils.PrintLog(err.Error(), 1)
		}
		_, err = ncutils.RunCmd(ipExec+" link del "+ifacename, false)
		if postdown != "" {
			runcmds := strings.Split(postdown, "; ")
			err = ncutils.RunCmds(runcmds, true)
		}
	}
	return err
}

// List - lists all networks on local machine
func List() error {

	networks, err := ncutils.GetSystemNetworks()
	if err != nil {
		return err
	}
	for _, network := range networks {
		cfg, err := config.ReadConfig(network)
		if err == nil {
			jsoncfg, _ := json.Marshal(
				map[string]string{
					"Name":           cfg.Node.Name,
					"Interface":      cfg.Node.Interface,
					"PrivateIPv4":    cfg.Node.Address,
					"PrivateIPv6":    cfg.Node.Address6,
					"PublicEndpoint": cfg.Node.Endpoint,
				})
			fmt.Println(network + ": " + string(jsoncfg))
		} else {
			ncutils.PrintLog(network+": Could not retrieve network configuration.", 1)
		}
	}
	return nil
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
		if !ncutils.IsKernel() {
			if err = wireguard.RemoveConf(ifacename, true); err == nil {
				ncutils.PrintLog("removed WireGuard interface: "+ifacename, 1)
			}
		} else {
			ipExec, err := exec.LookPath("ip")
			if err != nil {
				return err
			}
			out, err := ncutils.RunCmd(ipExec+" link del "+ifacename, false)
			dontprint := strings.Contains(out, "does not exist") || strings.Contains(out, "Cannot find device")
			if err != nil && !dontprint {
				ncutils.PrintLog("error running command: "+ipExec+" link del "+ifacename, 1)
				ncutils.PrintLog(out, 1)
			}
			if nodecfg.PostDown != "" {
				runcmds := strings.Split(nodecfg.PostDown, "; ")
				_ = ncutils.RunCmds(runcmds, false)
			}
		}
	}
	home := ncutils.GetNetclientPathSpecific()
	if ncutils.FileExists(home + "netconfig-" + network) {
		_ = os.Remove(home + "netconfig-" + network)
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
