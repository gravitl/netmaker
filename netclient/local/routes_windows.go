package local

import (
	"net"
	"time"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

func setRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	_, err = ncutils.RunCmd("route -p add "+addr.IP.String()+" mask "+addr.Mask.String()+" "+address, true)
	time.Sleep(time.Second >> 2)
	ncutils.RunCmd("route change "+addr.IP.String()+" mask "+addr.Mask.String()+" "+address, true)
	return err
}

func deleteRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	_, err = ncutils.RunCmd("route delete "+addr.IP.String()+" mask "+addr.Mask.String()+" "+address, true)
	return err
}
