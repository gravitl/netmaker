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

func setInternetGatewayRoute(iface, port string, v4, v6 bool) error {
	cmd := "wg set " + iface + " fwmark " + port
	if v4 {
		cmd += ";ip -4 route add 0.0.0.0/0 dev " + iface + " table " + port
		cmd += ";ip -4 rule add not fwmark 1234 table 2468"
		cmd += ";ip -4 rule add table main suppress_prefixlength 0"
		cmd += ";iptables-restore -n"
	}
	if v6 {
		cmd += ";ip -6 route add ::/0 dev " + iface + " table " + port
		cmd += ";ip -6 rule add not fwmark 1234 table 2468"
		cmd += ";ip -6 rule add table main suppress_prefixlength 0"
		cmd += ";ip6tables-restore -n"
	}
	if _, err := ncutils.RunCmd(cmd, true); err != nil {
		return err
	}
	return nil
}

func removeInternetGatewayRoute(iface, port string, v4, v6 bool) error {
	cmd := ""
	if v4 {
		cmd := "ip -4 rule delete table " + port
		cmd += ";ip -4 rule delete table main suppress_prefixlength 0"
	}
	if v6 {
		cmd := "ip -6 rule delete table " + port
		cmd += ";ip -6 rule delete table main suppress_prefixlength 0"
	}
	cmd += ":ip link del dev " + iface
	if v4 {
		cmd += ";iptables-restore -n"
	}
	if v6 {
		cmd += ";ip6tables-restore -n"
	}
	if _, err := ncutils.RunCmd(cmd, true); err != nil {
		return err
	}
	return nil
}
