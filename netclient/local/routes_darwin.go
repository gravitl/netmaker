package local

import (
	"fmt"
	"github.com/c-robinson/iplib"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"net"
	"regexp"
	"strings"
)

// GetDefaultRoute - Gets the default route (ip and interface) on a mac machine
func GetDefaultRoute() (string, string, error) {
	var ipaddr string
	var iface string
	var err error
	var outLine string
	output, err := ncutils.RunCmd("netstat -nr", false)
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if strings.Contains(line, "default") {
			outLine = line
			break
		}
	}
	space := regexp.MustCompile(`\s+`)
	outFormatted := space.ReplaceAllString(outLine, " ")
	if err != nil {
		return ipaddr, iface, err
	}
	outputSlice := strings.Split(string(outFormatted), " ")
	if !strings.Contains(outputSlice[0], "default") {
		return ipaddr, iface, fmt.Errorf("could not find default gateway")
	}
	ipaddr = outputSlice[1]
	if err = ncutils.CheckIPAddress(ipaddr); err != nil {
		return ipaddr, iface, err
	}
	iface = outputSlice[3]

	return ipaddr, iface, err
}

// route -n add -net 10.0.0.0/8 192.168.0.254
// networksetup -setadditionalroutes Ethernet 192.168.1.0 255.255.255.0 10.0.0.2 persistent
func setRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	var out string
	var inetx = "inet"
	if strings.Contains(addr.IP.String(), ":") {
		inetx = "inet6"
	}
	out, err = ncutils.RunCmd("route -n get -"+inetx+" "+addr.IP.String(), false)
	if err != nil {
		return err
	}
	if !(strings.Contains(out, iface)) {
		_, err = ncutils.RunCmd("route -q -n add -"+inetx+" "+addr.String()+" -interface "+iface, false)
	}
	return err
}

// SetExplicitRoute - sets route via explicit ip address
func SetExplicitRoute(iface string, destination *net.IPNet, gateway string) error {
	return setRoute(iface, destination, gateway)
}

func deleteRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	_, err = ncutils.RunCmd("route -q -n delete "+addr.String(), false)
	return err
}

func setCidr(iface, address string, addr *net.IPNet) {
	if iplib.Version(addr.IP) == 4 {
		ncutils.RunCmd("route -q -n add -net "+addr.String()+" "+address, false)
	} else if iplib.Version(addr.IP) == 6 {
		ncutils.RunCmd("route -A inet6 -q -n add -net "+addr.String()+" "+address, false)
	} else {
		logger.Log(1, "could not parse address: "+addr.String())
	}
}

func removeCidr(iface string, addr *net.IPNet, address string) {
	ncutils.RunCmd("route -q -n delete "+addr.String()+" -interface "+iface, false)
}
