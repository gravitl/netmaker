package wireguard

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// WgQuickDownMac - bring down mac interface, remove routes, and run post-down commands
func WgQuickDownMac(node *models.Node, iface string) error {
	if err := RemoveConfMac(iface); err != nil {
		return err
	}
	if node.PostDown != "" {
		runcmds := strings.Split(node.PostDown, "; ")
		ncutils.RunCmds(runcmds, true)
	}
	return nil
}

// RemoveConfMac - bring down mac interface and remove routes
func RemoveConfMac(iface string) error {
	realIface, err := getRealIface(iface)
	if realIface != "" {
		err = deleteInterface(iface, realIface)
	}
	return err
}

// WgQuickUpMac - bring up mac interface and set routes
func WgQuickUpMac(node *models.Node, iface string, confPath string) error {
	var err error
	var realIface string
	realIface, err = getRealIface(iface)
	if realIface != "" && err == nil {
		deleteInterface(iface, realIface)
		deleteRoutes(realIface)
	}
	realIface, err = addInterface(iface)
	if err != nil {
		logger.Log(1, "error creating wg interface")
		return err
	}
	time.Sleep(time.Second / 2)

	err = setConfig(realIface, confPath)
	if err != nil {
		logger.Log(1, "error setting config for ", realIface)
		return err
	}
	var ips = append(node.AllowedIPs, node.Address, node.Address6)
	for _, i := range ips {
		if i != "" {
			err = addAddress(realIface, i)
			if err != nil {
				logger.Log(1, "error adding address ", i, " on interface ", realIface)
				return err
			}
		}
	}
	setMTU(realIface, int(node.MTU))
	err = upInterface(realIface)
	if err != nil {
		logger.Log(1, "error turning on interface ", iface)
		return err
	}
	peerIPs := getPeerIPs(realIface)
	for _, i := range peerIPs {
		if i != "" {
			err = addRoute(i, realIface)
			if err != nil {
				logger.Log(1, "error adding route to ", realIface, " for ", i)
				return err
			}
		}
	}
	//next, wg-quick runs set_endpoint_direct_route
	//next, wg-quick runs monitor_daemon
	time.Sleep(time.Second / 2)
	if node.PostUp != "" {
		runcmds := strings.Split(node.PostUp, "; ")
		ncutils.RunCmds(runcmds, true)
	}
	return err
}

// addInterface - adds mac interface and creates reference file to match iface name with tun iface
func addInterface(iface string) (string, error) {
	ncutils.RunCmd("mkdir -p /var/run/wireguard/", true)
	ncutils.RunCmd("wireguard-go utun", true)
	realIface, err := ncutils.GetNewIface("/var/run/wireguard/")
	if iface != "" && err == nil {
		ifacePath := "/var/run/wireguard/" + iface + ".name"
		err = os.WriteFile(ifacePath, []byte(realIface), 0600)
	}
	return realIface, err
}

// getRealIface - retrieves tun iface based on reference iface name from config file
func getRealIface(iface string) (string, error) {
	ncutils.RunCmd("wg show interfaces", false)
	ifacePath := "/var/run/wireguard/" + iface + ".name"
	if !(ncutils.FileExists(ifacePath)) {
		return "", errors.New(ifacePath + " does not exist")
	}
	realIfaceName, err := ncutils.GetFileAsString(ifacePath)
	if err != nil {
		return "", err
	}
	if !(ncutils.FileExists("/var/run/wireguard/" + realIfaceName + ".sock")) {
		return "", errors.New("interface file does not exist")
	}
	return realIfaceName, nil
}

// deleteRoutes - deletes network routes associated with interface
func deleteRoutes(iface string) error {
	realIface, err := getRealIface(iface)
	if err != nil {
		return err
	}
	var inets = [2]string{"inet", "inet6"}
	for _, inet := range inets {
		ifaceList, err := ncutils.RunCmd("netstat -nr -f "+inet+" | grep -e "+realIface+" | awk '{print $1}'", true)
		if err != nil {
			return err
		}
		destinations := strings.Split(ifaceList, "\n")

		for _, i := range destinations {
			ncutils.RunCmd("route -q -n delete -"+inet+" "+i, true)
		}
	}
	// wg-quick deletes ENDPOINTS here (runs 'route -q delete' for each peer endpoint on the interface.)
	// We don't believe this is necessary.
	return nil
}

// deleteInterface - deletes the real interface and the referance file
func deleteInterface(iface string, realIface string) error {
	var err error
	var out string
	if iface != "" {
		os.Remove("/var/run/wireguard/" + realIface + ".sock")
		os.Remove("/var/run/wireguard/" + iface + ".name")
	}
	out, err = ncutils.RunCmd("ifconfig "+realIface+" down", false)
	if err != nil && strings.Contains(err.Error(), "does not exist") {
		err = nil
	} else if err != nil && out != "" {
		err = errors.New(out)
	}
	return err
}

// upInterface - bring up the interface with ifconfig
func upInterface(iface string) error {
	var err error
	_, err = ncutils.RunCmd("ifconfig "+iface+" up", true)
	return err
}

// addAddress - adds private address to the interface
func addAddress(iface string, addr string) error {
	var err error
	if strings.Contains(addr, ":") {
		_, err = ncutils.RunCmd("ifconfig "+iface+" inet6 "+addr+" alias", true)
	} else {
		_, err = ncutils.RunCmd("ifconfig "+iface+" inet "+addr+" 255.255.255.0 alias", true)
	}
	return err
}

// setMTU - sets MTU for the interface
func setMTU(iface string, mtu int) error {
	var err error
	if mtu == 0 {
		mtu = 1280
	}
	_, err = ncutils.RunCmd("ifconfig "+iface+" mtu "+strconv.Itoa(mtu), true)
	return err
}

// addRoute - adds network route to the interface if it does not already exist
func addRoute(addr string, iface string) error {
	var err error
	var out string
	var inetx = "inet"
	if strings.Contains(addr, ":") {
		inetx = "inet6"
	}
	out, err = ncutils.RunCmd("route -n get -"+inetx+" "+addr, true)
	if err != nil {
		return err
	}
	if !(strings.Contains(out, iface)) {
		_, err = ncutils.RunCmd("route -q -n add -"+inetx+" "+addr+" -interface "+iface, true)
	}
	return err
}

// setConfig - sets configuration of the wireguard interface from the config file
func setConfig(realIface string, confPath string) error {
	confString := getConfig(confPath)
	// pathFormatted := strings.Replace(confPath, " ", "\\ ", -1)
	err := os.WriteFile(confPath+".tmp", []byte(confString), 0600)
	if err != nil {
		return err
	}
	_, err = ncutils.RunCmd("wg setconf "+realIface+" "+confPath+".tmp", true)
	os.Remove(confPath + ".tmp")
	return err
}

// getConfig - gets config from config file and strips out incompatible fields
func getConfig(path string) string {
	// pathFormatted := strings.Replace(path, " ", "\\ ", -1)
	var confCmd = "grep -v -e Address -e MTU -e PostUp -e PostDown "
	confRaw, _ := ncutils.RunCmd(confCmd+path, false)
	return confRaw
}

// SetMacPeerRoutes - sets routes for interface from the peer list for all AllowedIps
func SetMacPeerRoutes(realIface string) error {
	var err error
	peerIPs := getPeerIPs(realIface)
	if len(peerIPs) == 0 {
		return err
	}
	for _, i := range peerIPs {
		if i != "" {
			err = addRoute(i, realIface)
			if err != nil {
				logger.Log(1, "error adding route to ", realIface, " for ", i)
				return err
			}
		}
	}
	return err
}

// getPeerIPs - retrieves peer AllowedIPs from WireGuard interface
func getPeerIPs(realIface string) []string {
	allowedIps := []string{}
	out, err := ncutils.RunCmd("wg show "+realIface+" allowed-ips", false)
	if err != nil {
		return allowedIps
	}
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 1 {
			allowedIps = append(allowedIps, fields[1:]...)
		}
	}
	return allowedIps
}
