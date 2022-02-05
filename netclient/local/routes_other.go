//go:build !linux
// +build !linux

package local

import (
	//"github.com/davecgh/go-spew/spew"

	"fmt"
	"net"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

//"github.com/davecgh/go-spew/spew"

/*

These functions are not used. These should only be called by Linux (see routes_linux.go). These routes return nothing if called.

*/

func setRoute(iface string, addr *net.IPNet) error {
	out, err := ncutils.RunCmd(fmt.Sprintf("ip route get %s", addr.IP.String()), false)
	if err != nil || !strings.Contains(out, iface) {
		_, err = ncutils.RunCmd(fmt.Sprintf("ip route add %s dev %s", addr.String(), iface), true)
	}
	return err
}

func deleteRoute(iface string, addr *net.IPNet) error {
	var err error
	out, _ := ncutils.RunCmd(fmt.Sprintf("ip route get %s", addr.IP.String()), false)
	if strings.Contains(out, iface) {
		_, err = ncutils.RunCmd(fmt.Sprintf("ip route del %s dev %s", addr.String(), iface), true)
	}
	return err
}
