package wireguard

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

func AddInterface(iface string) (string, error) {
	ncutils.RunCmd("mkdir -p /var/run/wireguard/", true)
	ncutils.RunCmd("wireguard-go utun", true)
	return ncutils.GetNewIface("/var/run/wireguard/")
}

func GetRealIface(iface string) (string, error) {
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

func DeleteRoutes(iface string) error {
	realIface, err := GetRealIface(iface)
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

func DeleteInterface(iface string) error {
	var err error
	if iface != "" {
		ncutils.RunCmd("rm -f /var/run/wireguard/"+iface+".sock", true)
	}
	_, err = ncutils.RunCmd("ifconfig "+iface+" down", false)
	if strings.Contains(err.Error(), "does not exist") {
		err = nil
	}
	return err
}

func UpInterface(iface string) error {
	var err error
	_, err = ncutils.RunCmd("ifconfig "+iface+" up", true)
	return err
}

func AddAddress(iface string, addr string) error {
	var err error
	if strings.Contains(addr, ":") {
		_, err = ncutils.RunCmd("ifconfig "+iface+" inet6 "+addr+" alias", true)
	} else {
		_, err = ncutils.RunCmd("ifconfig "+iface+" inet "+addr+" 255.255.255.0 alias", true)
	}
	return err
}

func SetMTU(iface string, mtu int) error {
	var err error
	if mtu == 0 {
		mtu = 1280
	}
	_, err = ncutils.RunCmd("ifconfig "+iface+" mtu "+strconv.Itoa(mtu), true)
	return err
}

func AddRoute(addr string, iface string) error {
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
	if out == "" {
		_, err = ncutils.RunCmd("route -q -n add -"+inetx+" "+addr+" -interface "+iface, true)
	}
	return err
}

func SetConfig(realIface string, confPath string) error {
	confString := GetConfig(confPath)
	err := os.WriteFile(confPath+".tmp", []byte(confString), 0644)
	if err != nil {
		return err
	}
	_, err = ncutils.RunCmd("wg setconf "+realIface+" "+confPath+".tmp", true)
	os.Remove(confPath + ".tmp")
	return err
}

func GetConfig(path string) string {
	var confCmd = "grep -v -e Address -e MTU -e PostUp -e PostDown "
	confRaw, _ := ncutils.RunCmd(confCmd+path, false)
	return confRaw
}

func WgQuickUpMac(node models.Node, iface string, confPath string) error {
	var err error
	var realIface string
	DeleteInterface(iface)
	DeleteRoutes(iface)

	realIface, err = AddInterface(iface)
	if err != nil {
		ncutils.PrintLog("error creating wg interface", 1)
		return err
	}
	time.Sleep(1)
	err = SetConfig(realIface, confPath)
	if err != nil {
		ncutils.PrintLog("error setting config for "+realIface, 1)
		return err
	}
	var ips []string
	ips = append(node.AllowedIPs, node.Address)
	ips = append(ips, node.Address6)
	for _, i := range ips {
		if i != "" {
			err = AddAddress(realIface, i)
			if err != nil {
				ncutils.PrintLog("error adding address "+i+" on interface "+realIface, 1)
				return err
			}
		}
	}
	SetMTU(realIface, int(node.MTU))
	err = UpInterface(realIface)
	if err != nil {
		ncutils.PrintLog("error turning on interface "+iface, 1)
		return err
	}
	for _, i := range ips {
		if i != "" {
			err = AddRoute(i, realIface)
			if err != nil {
				ncutils.PrintLog("error adding route to "+realIface+" for "+i, 1)
				return err
			}
		}
	}
	//next, wg-quick runs set_endpoint_direct_route
	//next, wg-quick runs monitor_daemon
	time.Sleep(1)
	if node.PostUp != "" {
		runcmds := strings.Split(node.PostUp, "; ")
		ncutils.RunCmds(runcmds, true)
	}
	return err
}

func WgQuickDownShortMac(iface string) error {
	var err error
	realIface, err := GetRealIface(iface)
	if realIface != "" {
		err = DeleteInterface(iface)
	}
	return err
}

func WgQuickDownMac(node models.Node, iface string) error {
	var err error
	realIface, err := GetRealIface(iface)
	if realIface != "" {
		err = DeleteInterface(iface)
	} else if err != nil {
		return err
	}
	if node.PostDown != "" {
		runcmds := strings.Split(node.PostDown, "; ")
		ncutils.RunCmds(runcmds, true)
	}
	return err
}
