package local

import (
	//"github.com/davecgh/go-spew/spew"

	"fmt"
	"net"
	"strings"

	"github.com/c-robinson/iplib"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

func setRoute(iface string, addr *net.IPNet, address string) error {
	out, err := ncutils.RunCmd(fmt.Sprintf("ip route get %s", addr.IP.String()), false)
	if err != nil || !strings.Contains(out, iface) {
		_, err = ncutils.RunCmd(fmt.Sprintf("ip route add %s dev %s", addr.String(), iface), false)
	}
	return err
}

func deleteRoute(iface string, addr *net.IPNet, address string) error {
	var err error
	out, _ := ncutils.RunCmd(fmt.Sprintf("ip route get %s", addr.IP.String()), false)
	if strings.Contains(out, iface) {
		_, err = ncutils.RunCmd(fmt.Sprintf("ip route del %s dev %s", addr.String(), iface), false)
	}
	return err
}

func setCidr(iface, address string, addr *net.IPNet) {
	if iplib.Version(addr.IP) == 4 {
		ncutils.RunCmd("ip -4 route add "+addr.String()+" dev "+iface, false)
	} else if iplib.Version(addr.IP) == 6 {
		ncutils.RunCmd("ip -6 route add "+addr.String()+" dev "+iface, false)
	} else {
		logger.Log(1, "could not parse address: "+addr.String())
	}
}

func removeCidr(iface string, addr *net.IPNet, address string) {
	ncutils.RunCmd("ip route delete "+addr.String()+" dev "+iface, false)
}
