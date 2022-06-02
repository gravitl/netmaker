package local

import (
	"net"

	"github.com/c-robinson/iplib"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

func setRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	_, _ = ncutils.RunCmd("route add -net "+addr.String()+" -interface "+iface, false)
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
