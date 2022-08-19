package wireguard

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const disconnect_error = "node disconnected"

// ApplyWithoutWGQuick - Function for running the equivalent of "wg-quick up" for linux if wg-quick is missing
func ApplyWithoutWGQuick(node *models.Node, ifacename, confPath string, isConnected bool) error {

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
		node.IsIngressGateway != "yes" {
		conf = wgtypes.Config{
			PrivateKey: &key,
		}
	} else {
		conf = wgtypes.Config{
			PrivateKey: &key,
			ListenPort: &nodeport,
		}
	}
	var address4 string
	var address6 string
	var mask4 string
	var mask6 string
	if node.Address != "" {
		netmaskArr := strings.Split(node.NetworkSettings.AddressRange, "/")
		var netmask = "32"
		if len(netmaskArr) == 2 {
			netmask = netmaskArr[1]
		}
		mask4 = netmask
		address4 = node.Address
	}
	if node.Address6 != "" {
		netmaskArr := strings.Split(node.NetworkSettings.AddressRange6, "/")
		var netmask = "128"
		if len(netmaskArr) == 2 {
			netmask = netmaskArr[1]
		}
		mask6 = netmask
		address6 = node.Address6
	}
	err = setKernelDevice(ifacename, address4, mask4, address6, mask6, isConnected)
	if err != nil {
		if err.Error() == disconnect_error {
			return nil
		}
	}

	_, err = wgclient.Device(ifacename)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.New("Unknown config error: " + err.Error())
		}
	}
	err = wgclient.ConfigureDevice(ifacename, conf)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Log(0, "Could not configure device: ", err.Error())
		}
	}
	if _, err := ncutils.RunCmd(ipExec+" link set down dev "+ifacename, false); err != nil {
		logger.Log(1, "attempted to remove interface before editing")
		return err
	}
	if node.PostDown != "" {
		runcmds := strings.Split(node.PostDown, "; ")
		_ = ncutils.RunCmds(runcmds, false)
	}
	// set MTU of node interface
	if _, err := ncutils.RunCmd(ipExec+" link set mtu "+strconv.Itoa(int(node.MTU))+" up dev "+ifacename, true); err != nil {
		logger.Log(1, "failed to create interface with mtu ", strconv.Itoa(int(node.MTU)), "-", ifacename)
		return err
	}
	if node.PostUp != "" {
		runcmds := strings.Split(node.PostUp, "; ")
		_ = ncutils.RunCmds(runcmds, true)
	}
	if node.Address6 != "" {
		logger.Log(1, "adding address: ", node.Address6)
		netmaskArr := strings.Split(node.NetworkSettings.AddressRange6, "/")
		var netmask = "64"
		if len(netmaskArr) == 2 {
			netmask = netmaskArr[1]
		}
		ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+node.Address6+"/"+netmask, true)
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
		logger.Log(1, "error running command: ", ipExec, " link del ", ifacename)
		logger.Log(1, out)
	}
	network := strings.ReplaceAll(ifacename, "nm-", "")
	nodeconf, err := config.ReadConfig(network)
	if nodeconf != nil && err == nil {
		if nodeconf.Node.PostDown != "" {
			runcmds := strings.Split(nodeconf.Node.PostDown, "; ")
			_ = ncutils.RunCmds(runcmds, false)
		}
	} else if err != nil {
		logger.Log(1, "error retrieving config: ", err.Error())
	}
	return err
}

func setKernelDevice(ifacename, address4, mask4, address6, mask6 string, isConnected bool) error {
	ipExec, err := exec.LookPath("ip")
	if err != nil {
		return err
	}

	// == best effort ==
	ncutils.RunCmd("ip link delete dev "+ifacename, false)
	if !isConnected {
		return fmt.Errorf(disconnect_error)
	}

	ncutils.RunCmd(ipExec+" link add dev "+ifacename+" type wireguard", true)
	if address4 != "" {
		ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+address4+"/"+mask4, true)
	}
	if address6 != "" {
		ncutils.RunCmd(ipExec+" address add dev "+ifacename+" "+address6+"/"+mask6, true)
	}
	return nil
}
