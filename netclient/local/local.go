package local

import (
	//"github.com/davecgh/go-spew/spew"
	"errors"
	"log"
	"net"
	"runtime"
	"strings"
	"os/exec"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

func SetIPForwarding() error {
	os := runtime.GOOS
	var err error
	switch os {
	case "linux":
		err = SetIPForwardingLinux()
	case "darwin":
		err = SetIPForwardingMac()
	default:
		err = errors.New("This OS is not supported")
	}
	return err
}

func SetIPForwardingLinux() error {
	out, err := ncutils.RunCmd("sysctl net.ipv4.ip_forward", true)
	if err != nil {
		log.Println("WARNING: Error encountered setting ip forwarding. This can break functionality.")
		return err
	} else {
		s := strings.Fields(string(out))
		if s[2] != "1" {
			_, err = ncutils.RunCmd("sysctl -w net.ipv4.ip_forward=1", true)
			if err != nil {
				log.Println("WARNING: Error encountered setting ip forwarding. You may want to investigate this.")
				return err
			}
		}
	}
	return nil
}

func SetIPForwardingMac() error {
	_, err := ncutils.RunCmd("sysctl -w net.inet.ip.forwarding=1", true)
	if err != nil {
		log.Println("WARNING: Error encountered setting ip forwarding. This can break functionality.")
	}
	return err
}

func IsWGInstalled() bool {
	out, err := ncutils.RunCmd("wg help", true)
	if err != nil {
		_, err1 := exec.LookPath("wireguard-go")
		_, err2 := exec.LookPath("boringtun")
		return err1 == nil || err2 == nil
	}
	return strings.Contains(out, "Available subcommand")
}

func GetMacIface(ipstring string) (string, error) {
	var wgiface string
	_, checknet, err := net.ParseCIDR(ipstring + "/24")
	if err != nil {
		return wgiface, errors.New("could not parse ip " + ipstring)
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return wgiface, err
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := addr.(*net.IPNet).IP
			if checknet.Contains(ip) {
				wgiface = iface.Name
				break
			}
		}
	}
	if wgiface == "" {
		err = errors.New("could not find iface for address " + ipstring)
	}
	return wgiface, err
}

func HasNetwork(network string) bool {

	if ncutils.IsWindows() {
		return ncutils.FileExists(ncutils.GetNetclientPathSpecific() + "netconfig-" + network)
	}
	return ncutils.FileExists("/etc/systemd/system/netclient-"+network+".timer") ||
		ncutils.FileExists(ncutils.GetNetclientPathSpecific()+"netconfig-"+network)
}
