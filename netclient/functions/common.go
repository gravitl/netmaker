package functions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os/exec"
	"strings"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/netclientutils"
	"golang.zx2c4.com/wireguard/wgctrl"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var (
	wcclient nodepb.NodeServiceClient
)

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
		err = errors.New("could not find local ip")
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

func GetNode(network string) models.Node {

	modcfg, err := config.ReadConfig(network)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	return modcfg.Node
}

func Uninstall() error {
	networks, err := GetNetworks()
	if err != nil {
		log.Println("unable to retrieve networks: ", err)
		log.Println("continuing uninstall without leaving networks")
	} else {
		for _, network := range networks {
			err = LeaveNetwork(network)
			if err != nil {
				log.Println("Encounter issue leaving network "+network+": ", err)
			}
		}
	}
	// clean up OS specific stuff
	if netclientutils.IsWindows() {
		local.Cleanup()
	}
	return err
}

func LeaveNetwork(network string) error {
	//need to  implement checkin on server side
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	servercfg := cfg.Server
	node := cfg.Node

	var wcclient nodepb.NodeServiceClient
	conn, err := grpc.Dial(cfg.Server.GRPCAddress, 
		netclientutils.GRPCRequestOpts(cfg.Server.GRPCSSL))
	if err != nil {
		log.Printf("Unable to establish client connection to "+servercfg.GRPCAddress+": %v", err)
	} else {
		wcclient = nodepb.NewNodeServiceClient(conn)

		ctx := context.Background()
		ctx, err = auth.SetJWT(wcclient, network)
		if err != nil {
			log.Printf("Failed to authenticate: %v", err)
		} else {
			if netclientutils.IsWindows() {
				local.RemoveWindowsConf(node.Interface)
				log.Println("removed Windows tunnel " + node.Interface)
			}
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
				log.Printf("Encountered error deleting node: %v", err)
				log.Println(err)
			} else {
				log.Println("Removed machine from " + node.Network + " network on remote server")
			}
		}
	}
	return RemoveLocalInstance(cfg, network)
}

func RemoveLocalInstance(cfg *config.ClientConfig, networkName string) error {
	err := local.WipeLocal(networkName)
	if err != nil {
		log.Printf("Unable to wipe local config: %v", err)
	} else {
		log.Println("Removed " + networkName + " network locally")
	}
	if cfg.Daemon != "off" {
		if netclientutils.IsWindows() {
			// TODO: Remove job?
		} else {
			err = local.RemoveSystemDServices(networkName)
		}
	}
	return err
}

func DeleteInterface(ifacename string, postdown string) error {
	var err error
	if netclientutils.IsWindows() {
		err = local.RemoveWindowsConf(ifacename)
	} else {
		ipExec, errN := exec.LookPath("ip")
		err = errN
		if err != nil {
			log.Println(err)
		}
		_, err = local.RunCmd(ipExec + " link del " + ifacename, false)
		if postdown != "" {
			runcmds := strings.Split(postdown, "; ")
			err = local.RunCmds(runcmds, true)
		}
	}
	return err
}

func List() error {

	networks, err := GetNetworks()
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
			log.Println(network + ": " + string(jsoncfg))
		} else {
			log.Println(network + ": Could not retrieve network configuration.")
		}
	}
	return nil
}

func GetNetworks() ([]string, error) {
	var networks []string
	files, err := ioutil.ReadDir(netclientutils.GetNetclientPath())
	if err != nil {
		return networks, err
	}
	for _, f := range files {
		if strings.Contains(f.Name(), "netconfig-") {
			networkname := stringAfter(f.Name(), "netconfig-")
			networks = append(networks, networkname)
		}
	}
	return networks, err
}

func stringAfter(original string, substring string) string {
	position := strings.LastIndex(original, substring)
	if position == -1 {
		return ""
	}
	adjustedPosition := position + len(substring)

	if adjustedPosition >= len(original) {
		return ""
	}
	return original[adjustedPosition:len(original)]
}
