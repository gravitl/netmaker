package local

import (
	"net"
	"time"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

func setRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	_, err = ncutils.RunCmd("route ADD "+addr.String()+" "+address, false)
	time.Sleep(time.Second >> 2)
	ncutils.RunCmd("route CHANGE "+addr.IP.String()+" MASK "+addr.Mask.String()+" "+address, false)
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
