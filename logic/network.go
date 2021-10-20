package logic

import (
	"net"
	"os/exec"
	"strings"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// GetLocalIP - gets the local ip
func GetLocalIP(node models.Node) string {

	var local string

	ifaces, err := net.Interfaces()
	if err != nil {
		return local
	}
	_, localrange, err := net.ParseCIDR(node.LocalRange)
	if err != nil {
		return local
	}

	found := false
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if i.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := i.Addrs()
		if err != nil {
			return local
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				if !found {
					ip = v.IP
					local = ip.String()
					if node.IsLocal == "yes" {
						found = localrange.Contains(ip)
					} else {
						found = true
					}
				}
			case *net.IPAddr:
				if !found {
					ip = v.IP
					local = ip.String()
					if node.IsLocal == "yes" {
						found = localrange.Contains(ip)

					} else {
						found = true
					}
				}
			}
		}
	}
	return local
}

// == Private ==

func deleteInterface(ifacename string, postdown string) error {
	var err error
	if !ncutils.IsKernel() {
		err = RemoveConf(ifacename, true)
	} else {
		ipExec, errN := exec.LookPath("ip")
		err = errN
		if err != nil {
			ncutils.PrintLog(err.Error(), 1)
		}
		_, err = ncutils.RunCmd(ipExec+" link del "+ifacename, false)
		if postdown != "" {
			runcmds := strings.Split(postdown, "; ")
			err = ncutils.RunCmds(runcmds, true)
		}
	}
	return err
}

func isInterfacePresent(iface string, address string) (string, bool) {
	var interfaces []net.Interface
	var err error
	interfaces, err = net.Interfaces()
	if err != nil {
		Log("ERROR: could not read interfaces", 0)
		return "", true
	}
	for _, currIface := range interfaces {
		var currAddrs []net.Addr
		currAddrs, err = currIface.Addrs()
		if err != nil || len(currAddrs) == 0 {
			continue
		}
		for _, addr := range currAddrs {
			if strings.Contains(addr.String(), address) && currIface.Name != iface {
				return currIface.Name, false
			}
		}
	}
	return "", true
}
