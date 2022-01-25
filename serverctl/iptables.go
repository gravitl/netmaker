package serverctl

import (
	"net"
	"os/exec"
	"strings"

	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
)

// InitServerNetclient - intializes the server netclient
func InitIPTables() error {
	_, err := exec.LookPath("iptables")
	if err != nil {
		return err
	}
	setForwardPolicy()
	portForwardServices()
	return nil
}

func portForwardServices() {
	services := servercfg.GetPortForwardServiceList()

	for _, service := range services {
		switch service {
		case "mq":
			iptablesPortForward("mq", "1883", false)
		case "dns":
			iptablesPortForward("mq", "1883", false)
		case "ssh":
			iptablesPortForward("127.0.0.1", "22", true)
		default:
			params := strings.Split(service, ":")
			iptablesPortForward(params[0], params[1], true)
		}
	}
}

func setForwardPolicy() {
	ncutils.RunCmd("iptables --policy FORWARD ACCEPT", true)
}

func iptablesPortForward(entry string, port string, isIP bool) {
	var address string
	if !isIP {
		ips, _ := net.LookupIP(entry)
		for _, ip := range ips {
			if ipv4 := ip.To4(); ipv4 != nil {
				address = ip.String()
				break
			}
		}
	} else {
		address = entry
	}
	ncutils.RunCmd("iptables -t nat -A PREROUTING -p tcp --dport "+port+" -j DNAT --to-destination "+address+":"+port, true)
	ncutils.RunCmd("iptables -t nat -A POSTROUTING -j MASQUERADE", true)
}
