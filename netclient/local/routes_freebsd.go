package local

import (
	"fmt"
	"net"
	"strings"

	"github.com/c-robinson/iplib"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// GetDefaultRoute - Gets the default route (ip and interface) on a freebsd machine
func GetDefaultRoute() (string, string, error) {
	var ipaddr string
	var iface string
	var err error

	output, err := ncutils.RunCmd("route show default", true)
	if err != nil {
		return ipaddr, iface, err
	}
	outFormatted := strings.ReplaceAll(output, "\n", "")
	if !strings.Contains(outFormatted, "default") && !strings.Contains(outFormatted, "interface:") {
		return ipaddr, iface, fmt.Errorf("could not find default gateway")
	}
	outputSlice := strings.Split(string(outFormatted), " ")
	for i, outString := range outputSlice {
		if outString == "gateway:" {
			ipaddr = outputSlice[i+1]
		}
		if outString == "interface:" {
			iface = outputSlice[i+1]
		}
	}
	return ipaddr, iface, err
}

func setRoute(iface string, addr *net.IPNet, address string) error {
	_, err := ncutils.RunCmd("route add -net "+addr.String()+" -interface "+iface, false)
	return err
}

// SetExplicitRoute - sets route via explicit ip address
func SetExplicitRoute(iface string, destination *net.IPNet, gateway string) error {
	_, err := ncutils.RunCmd("route add "+destination.String()+" "+gateway, false)
	return err
}

func deleteRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	_, _ = ncutils.RunCmd("route delete -net "+addr.String()+" -interface "+iface, false)
	return err
}

func setCidr(iface, address string, addr *net.IPNet) {
	if iplib.Version(addr.IP) == 4 {
		ncutils.RunCmd("route add -net "+addr.String()+" -interface "+iface, false)
	} else if iplib.Version(addr.IP) == 6 {
		ncutils.RunCmd("route add -net -inet6 "+addr.String()+" -interface "+iface, false)
	} else {
		logger.Log(1, "could not parse address: "+addr.String())
	}
	ncutils.RunCmd("route add -net "+addr.String()+" -interface "+iface, false)
}

func removeCidr(iface string, addr *net.IPNet, address string) {
	ncutils.RunCmd("route delete -net "+addr.String()+" -interface "+iface, false)
}
