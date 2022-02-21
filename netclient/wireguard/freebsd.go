package wireguard

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// ApplyWithoutWGQuick - Function for running the equivalent of "wg-quick up" for linux if wg-quick is missing
func ApplyWithoutWGQuickFreeBSD(node *models.Node, ifacename string, confPath string) error {

	netmaskArr := strings.Split(node.NetworkSettings.AddressRange, "/")
	var netmask = "32"
	if len(netmaskArr) == 2 {
		netmask = netmaskArr[1]
	}
	setKernelDeviceFreeBSD(ifacename, node.Address, netmask)
	setConfFreeBSD(ifacename, confPath)
	addAddressFreeBSD(ifacename, node.Address6+"/64", node.Address+"/"+netmask)
	if _, err := ncutils.RunCmd("ifconfig "+ifacename+" mtu "+strconv.Itoa(int(node.MTU))+" up", true); err != nil {
		logger.Log(2, "failed to create interface with mtu", strconv.Itoa(int(node.MTU)), "-", ifacename)
		return err
	}
	if node.PostUp != "" {
		runcmds := strings.Split(node.PostUp, "; ")
		_ = ncutils.RunCmds(runcmds, true)
	}
	return nil
}

// RemoveWithoutWGQuickFreeBSD - Function for running the equivalent of "wg-quick down" for linux if wg-quick is missing
func RemoveWithoutWGQuickFreeBSD(ifacename string) error {
	delInterface(ifacename)
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

func setKernelDeviceFreeBSD(ifacename, address, mask string) error {
	// == best effort ==
	delInterface(ifacename)
	addInterfaceFreeBSD(ifacename)
	return nil
}

func delInterface(ifacename string) {
	ncutils.RunCmd("rm -f /var/run/wireguard/"+ifacename+".sock", false)
	ncutils.RunCmd("ifconfig "+ifacename+" destroy", false)
	output, _ := ncutils.RunCmd("wg", false)
	starttime := time.Now()
	ifaceGone := !strings.Contains(output, ifacename)
	for !ifaceGone && !(time.Now().After(starttime.Add(time.Second << 4))) {
		output, _ = ncutils.RunCmd("wg", false)
		time.Sleep(time.Second)
		ifaceGone = !strings.Contains(output, ifacename)
	}
}

func addInterfaceFreeBSD(ifacename string) {
	ncutils.RunCmd("ifconfig wg create name "+ifacename, false)
	output, _ := ncutils.RunCmd("wg", false)
	starttime := time.Now()
	ifaceReady := strings.Contains(output, ifacename)
	for !ifaceReady && !(time.Now().After(starttime.Add(time.Second << 4))) {
		output, _ = ncutils.RunCmd("wg", false)
		time.Sleep(time.Second)
		ifaceReady = strings.Contains(output, ifacename)
	}
}

func addAddressFreeBSD(ifacename, inet6, inet string) {
	if inet6 != "" && inet6[0:1] != "/" {
		ncutils.RunCmd("ifconfig "+ifacename+" inet6 "+inet6+" alias", false)
	}
	if inet != "" && inet[0:1] != "/" {
		ncutils.RunCmd("ifconfig "+ifacename+" inet "+inet+" alias", false)

	}
}

func setConfFreeBSD(iface string, confPath string) error {
	var tmpConf = confPath + ".sync.tmp"
	var confCmd = "wg-quick strip "
	if ncutils.IsMac() {
		confCmd = "grep -v -e Address -e MTU -e PostUp -e PostDown "
	}
	confRaw, err := ncutils.RunCmd(confCmd+confPath, false)
	if err != nil {
		return err
	}
	regex := regexp.MustCompile(".*Warning.*\n")
	conf := regex.ReplaceAllString(confRaw, "")
	err = os.WriteFile(tmpConf, []byte(conf), 0600)
	if err != nil {
		return err
	}
	_, err = ncutils.RunCmd("wg setconf "+iface+" "+tmpConf, true)
	errN := os.Remove(tmpConf)
	if errN != nil {
		ncutils.Log(errN.Error())
	}
	return err
}
