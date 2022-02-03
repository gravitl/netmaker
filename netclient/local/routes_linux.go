//go:build linux
// +build linux

package local

import (
	//"github.com/davecgh/go-spew/spew"

	"fmt"
	"net"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

func setRoute(iface string, addr *net.IPNet) error {
	var err error
	_, err = ncutils.RunCmd(fmt.Sprintf("ip route add %s dev %s", addr.String(), iface), true)
	return err
}

func deleteRoute(iface string, addr *net.IPNet) error {
	var err error
	_, err = ncutils.RunCmd(fmt.Sprintf("ip route del %s dev %s", addr.String(), iface), true)
	return err
}
