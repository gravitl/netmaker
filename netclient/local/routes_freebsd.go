package local

import (
	"github.com/gravitl/netmaker/netclient/ncutils"
	"log"
	"net"
)

func setRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	log.Println("DELETE ME: running command route add -net " + addr.String() + " -interface " + iface)
	_, _ = ncutils.RunCmd("route add -net "+addr.String()+" -interface "+iface, false)
	return err
}

func deleteRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	log.Println("DELETE ME: running command route delete -net " + addr.String() + " -interface " + iface)
	_, _ = ncutils.RunCmd("route delete -net "+addr.String()+" -interface "+iface, false)
	return err
}

func setCidr(iface, address string, addr *net.IPNet) {
	log.Println("DELETE ME: running command route add -net " + addr.String() + " -interface " + iface)
	ncutils.RunCmd("route add -net "+addr.String()+" -interface "+iface, false)
}

func removeCidr(iface string, addr *net.IPNet, address string) {
	log.Println("DELETE ME: running command route delete -net " + addr.String() + " -interface " + iface)
	ncutils.RunCmd("route delete -net "+addr.String()+" -interface "+iface, false)
}
