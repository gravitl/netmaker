package local

import (
	"net"
	"strings"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

// route -n add -net 10.0.0.0/8 192.168.0.254
// networksetup -setadditionalroutes Ethernet 192.168.1.0 255.255.255.0 10.0.0.2 persistent
func setRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	var out string
	var inetx = "inet"
	if strings.Contains(addr.IP.String(), ":") {
		inetx = "inet6"
	}
	out, err = ncutils.RunCmd("route -n get -"+inetx+" "+addr.IP.String(), true)
	if err != nil {
		return err
	}
	if !(strings.Contains(out, iface)) {
		_, err = ncutils.RunCmd("route -q -n add -"+inetx+" "+addr.String()+" -interface "+iface, true)
	}
	return err
}

func deleteRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	_, err = ncutils.RunCmd("route -q -n delete "+addr.String(), true)
	return err
}

func setCidr(iface, address string, addr *net.IPNet) {
	ncutils.RunCmd("route -q -n add -net "+addr.String()+" "+address, true)
}
