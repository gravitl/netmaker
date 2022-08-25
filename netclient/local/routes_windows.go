package local

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

// GetDefaultRoute - Gets the default route (ip and interface) on a windows machine
func GetDefaultRoute() (string, string, error) {
	var ipaddr string
	var iface string
	var err error
	var outLine string
	output, err := ncutils.RunCmd("netstat -rn", false)
	if err != nil {
		return ipaddr, iface, err
	}
	var startLook bool
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if strings.Contains(line, "Active Routes:") {
			startLook = true
		}
		if startLook && strings.Contains(line, "0.0.0.0") {
			outLine = line
			break
		}
	}
	if outLine == "" {
		return ipaddr, iface, fmt.Errorf("could not find default gateway")
	}
	space := regexp.MustCompile(`\s+`)
	outputSlice := strings.Split(strings.TrimSpace(space.ReplaceAllString(outLine, " ")), " ")
	ipaddr = outputSlice[len(outputSlice)-3]
	if err = ncutils.CheckIPAddress(ipaddr); err != nil {
		return ipaddr, iface, fmt.Errorf("invalid output for ip address check: " + err.Error())
	}
	iface = "irrelevant"
	return ipaddr, iface, err
}

func setRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	_, err = ncutils.RunCmd("route ADD "+addr.String()+" "+address, false)
	time.Sleep(time.Second >> 2)
	ncutils.RunCmd("route CHANGE "+addr.IP.String()+" MASK "+addr.Mask.String()+" "+address, false)
	return err
}

// SetExplicitRoute - sets route via explicit ip address
func SetExplicitRoute(iface string, destination *net.IPNet, gateway string) error {
	var err error
	_, err = ncutils.RunCmd("route ADD "+destination.String()+" "+gateway, false)
	time.Sleep(time.Second >> 2)
	ncutils.RunCmd("route CHANGE "+destination.IP.String()+" MASK "+destination.Mask.String()+" "+gateway, false)
	return err
}

func deleteRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	_, err = ncutils.RunCmd("route DELETE "+addr.IP.String()+" MASK "+addr.Mask.String()+" "+address, false)
	return err
}

func setCidr(iface, address string, addr *net.IPNet) {
	ncutils.RunCmd("route ADD "+addr.String()+" "+address, false)
	time.Sleep(time.Second >> 2)
	ncutils.RunCmd("route CHANGE "+addr.IP.String()+" MASK "+addr.Mask.String()+" "+address, false)
}

func removeCidr(iface string, addr *net.IPNet, address string) {
	ncutils.RunCmd("route DELETE "+addr.String(), false)
}
