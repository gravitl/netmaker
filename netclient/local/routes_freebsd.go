package local

import (
	"net"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

func setRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	_, _ = ncutils.RunCmd("route add -net "+addr.String()+" -interface "+iface, true)
	return err
}

func deleteRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	_, err = ncutils.RunCmd("route delete -net "+addr.String()+" -interface "+iface, true)
	return err
}
