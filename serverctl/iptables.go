package serverctl

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
)

const netmakerProcessName = "netmaker"

// InitIPTables - intializes the server iptables
func InitIPTables(force bool) error {
	_, err := exec.LookPath("iptables")
	if err != nil {
		return err
	}
	err = setForwardPolicy()
	if err != nil {
		logger.Log(0, "error setting iptables forward policy: "+err.Error())
	}

	err = portForwardServices(force)
	if err != nil {
		return err
	}
	if isContainerized() && servercfg.IsHostNetwork() {
		err = setHostCoreDNSMapping()
	}
	return err
}

// set up port forwarding for services listed in config
func portForwardServices(force bool) error {
	var err error
	services := servercfg.GetPortForwardServiceList()
	if len(services) == 0 || services[0] == "" {
		return nil
	}
	for _, service := range services {
		switch service {
		case "mq":
			err = iptablesPortForward("mq", servercfg.GetMQServerPort(), servercfg.GetMQServerPort(), false, force)
		case "dns":
			err = iptablesPortForward("coredns", "53", "53", false, force)
		case "ssh":
			err = iptablesPortForward("netmaker", "22", "22", false, force)
		default:
			params := strings.Split(service, ":")
			if len(params) == 3 {
				err = iptablesPortForward(params[0], params[1], params[2], true, force)
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// determine if process is running in container
func isContainerized() bool {
	fileBytes, err := os.ReadFile("/proc/1/sched")
	if err != nil {
		logger.Log(1, "error determining containerization: "+err.Error())
		return false
	}
	fileString := string(fileBytes)
	return strings.Contains(fileString, netmakerProcessName)
}

// make sure host allows forwarding
func setForwardPolicy() error {
	logger.Log(2, "setting iptables forward policy")
	_, err := ncutils.RunCmd("iptables --policy FORWARD ACCEPT", false)
	return err
}

// port forward from an entry, can contain a dns name for lookup
func iptablesPortForward(entry string, inport string, outport string, isIP, force bool) error {

	var address string
	if !isIP {
	out:
		for i := 1; i < 4; i++ {
			ips, err := net.LookupIP(entry)
			if err != nil && i > 2 {
				return err
			}
			for _, ip := range ips {
				if ipv4 := ip.To4(); ipv4 != nil {
					address = ipv4.String()
				}
			}
			if address != "" {
				break out
			}
			time.Sleep(time.Second)
		}
	} else {
		address = entry
	}
	if address == "" {
		return errors.New("could not locate ip for " + entry)
	}

	if output, err := ncutils.RunCmd("iptables -t nat -C PREROUTING -p tcp --dport "+inport+" -j DNAT --to-destination "+address+":"+outport, false); output != "" || err != nil || force {
		_, err := ncutils.RunCmd("iptables -t nat -A PREROUTING -p tcp --dport "+inport+" -j DNAT --to-destination "+address+":"+outport, false)
		if err != nil {
			return err
		}
		_, err = ncutils.RunCmd("iptables -t nat -A PREROUTING -p udp --dport "+inport+" -j DNAT --to-destination "+address+":"+outport, false)
		if err != nil {
			return err
		}
		_, err = ncutils.RunCmd("iptables -t nat -A POSTROUTING -j MASQUERADE", false)
		return err
	} else {
		logger.Log(3, "mq forwarding is already set... skipping")
	}
	return nil
}

// if running in host networking mode, run iptables to map to CoreDNS container
func setHostCoreDNSMapping() error {
	logger.Log(1, "forwarding dns traffic on host from netmaker interfaces to 53053")
	ncutils.RunCmd("iptables -t nat -A PREROUTING -i nm-+ -p tcp --match tcp --dport 53 --jump REDIRECT --to-ports 53053", true)
	_, err := ncutils.RunCmd("iptables -t nat -A PREROUTING -i nm-+ -p udp --match udp --dport 53 --jump REDIRECT --to-ports 53053", true)
	return err
}
