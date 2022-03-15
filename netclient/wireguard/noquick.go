package wireguard

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// ApplyWithoutWGQuick - Function for running the equivalent of "wg-quick up" for linux if wg-quick is missing
func ApplyWithoutWGQuick(node *models.Node, ifacename string, confPath string) error {

	ipExec, err := exec.LookPath("ip")
	if err != nil {
		return err
	}
	wgclient, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wgclient.Close()

	privkey, err := RetrievePrivKey(node.Network)
	if err != nil {
		return err
	}
	key, err := wgtypes.ParseKey(privkey)
	if err != nil {
		return err
	}
	conf := wgtypes.Config{}
	nodeport := int(node.ListenPort)
	if node.UDPHolePunch == "yes" &&
		node.IsServer == "no" &&
		node.IsIngressGateway != "yes" &&
		node.IsStatic != "yes" {
		conf = wgtypes.Config{
			PrivateKey: &key,
		}
	} else {
		conf = wgtypes.Config{
			PrivateKey: &key,
			ListenPort: &nodeport,
		}
	}

	netmaskArr := strings.Split(node.NetworkSettings.AddressRange, "/")
	var netmask = "32"
	if len(netmaskArr) == 2 {
		netmask = netmaskArr[1]
	}
	setKernelDevice(ifacename, node.Address, netmask)

	_, err = wgclient.Device(ifacename)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.New("Unknown config error: " + err.Error())
		}
	}
	err = wgclient.ConfigureDevice(ifacename, conf)
	if err != nil {
		if os.IsNotExist(err) {
			ncutils.PrintLog("Could not configure device: "+err.Error(), 0)
		}
	}
	if _, err := ncutils.RunCmd(ipExec+" link set down dev "+ifacename, false); err != nil {
		ncutils.PrintLog("attempted to remove interface before editing", 1)
		return err
	}
	if node.PostDown != "" {
		runcmds := strings.Split(node.PostDown, "; ")
		_ = ncutils.RunCmds(runcmds, false)
	}
	// set MTU of node interface
	if _, err := ncutils.RunCmd(ipExec+" link set mtu "+strconv.Itoa(int(node.MTU))+" up dev "+ifacename, true); err != nil {
		ncutils.PrintLog("failed to create interface with mtu "+strconv.Itoa(int(node.MTU))+"-"+ifacename, 1)
		return err
	}
	if node.PostUp != "" {
		runcmds := strings.Split(node.PostUp, "; ")
		_ = ncutils.RunCmds(runcmds, true)
	}
	if node.Address6 != "" && node.IsDualStack == "yes" {
		ncutils.PrintLog("adding address: "+node.Address6, 1)
		_, _ = ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+node.Address6+"/64", true)
	}
	return nil
}

// RemoveWithoutWGQuick - Function for running the equivalent of "wg-quick down" for linux if wg-quick is missing
func RemoveWithoutWGQuick(ifacename string) error {
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
	network := strings.ReplaceAll(ifacename, "nm-", "")
	nodeconf, err := config.ReadConfig(network)
	if nodeconf != nil && err == nil {
		if nodeconf.Node.PostDown != "" {
			runcmds := strings.Split(nodeconf.Node.PostDown, "; ")
			_ = ncutils.RunCmds(runcmds, false)
		}
	} else if err != nil {
		ncutils.PrintLog("error retrieving config: "+err.Error(), 1)
	}
	return err
}

func setKernelDevice(ifacename, address, mask string) error {
	ipExec, err := exec.LookPath("ip")
	if err != nil {
		return err
	}

	// == best effort ==
	ncutils.RunCmd("ip link delete dev "+ifacename, false)
	ncutils.RunCmd(ipExec+" link add dev "+ifacename+" type wireguard", true)
	ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+address+"/"+mask, true) // this was a bug waiting to happen

	return nil
}
