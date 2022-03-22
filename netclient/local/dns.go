package local

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	//"github.com/davecgh/go-spew/spew"
	"log"
	"os/exec"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

const DNS_UNREACHABLE_ERROR = "nameserver unreachable"

// SetDNSWithRetry - Attempt setting dns, if it fails return true (to reset dns)
func SetDNSWithRetry(node models.Node, address string) bool {
	var reachable bool
	if !hasPrereqs() {
		return true
	}
	for counter := 0; !reachable && counter < 5; counter++ {
		reachable = IsDNSReachable(address)
		time.Sleep(time.Second << 1)
	}
	if !reachable {
		logger.Log(0, "not setting dns (server unreachable), will try again later: "+address)
		return true
	} else if err := UpdateDNS(node.Interface, node.Network, address); err != nil {
		logger.Log(0, "error applying dns"+err.Error())
	} else if IsDNSWorking(node.Network, address) {
		return true
	}
	resetDNS()
	return false
}

func resetDNS() {
	ncutils.RunCmd("systemctl restart systemd-resolved", true)
}

// SetDNS - sets the DNS of a local machine
func SetDNS(nameserver string) error {
	bytes, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return err
	}
	resolvstring := string(bytes)
	// //check whether s contains substring text
	hasdns := strings.Contains(resolvstring, nameserver)
	if hasdns {
		return nil
	}
	resolv, err := os.OpenFile("/etc/resolv.conf", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer resolv.Close()
	_, err = resolv.WriteString("nameserver " + nameserver + "\n")

	return err
}

func hasPrereqs() bool {
	if !ncutils.IsLinux() {
		return false
	}
	_, err := exec.LookPath("resolvectl")
	return err == nil
}

// UpdateDNS - updates local DNS of client
func UpdateDNS(ifacename string, network string, nameserver string) error {
	if !ncutils.IsLinux() {
		return nil
	}
	if ifacename == "" {
		return fmt.Errorf("cannot set dns: interface name is blank")
	}
	if network == "" {
		return fmt.Errorf("cannot set dns: network name is blank")
	}
	if nameserver == "" {
		return fmt.Errorf("cannot set dns: nameserver is blank")
	}
	if !IsDNSReachable(nameserver) {
		return fmt.Errorf(DNS_UNREACHABLE_ERROR + " : " + nameserver + ":53")
	}
	_, err := exec.LookPath("resolvectl")
	if err != nil {
		log.Println(err)
		log.Println("WARNING: resolvectl not present. Unable to set dns. Install resolvectl or run manually.")
	} else {
		_, err = ncutils.RunCmd("resolvectl domain "+ifacename+" ~"+network, true)
		if err != nil {
			log.Println("WARNING: Error encountered setting domain on dns. Aborted setting dns.")
		} else {
			_, err = ncutils.RunCmd("resolvectl default-route "+ifacename+" false", true)
			if err != nil {
				log.Println("WARNING: Error encountered setting default-route on dns. Aborted setting dns.")
			} else {
				_, err = ncutils.RunCmd("resolvectl dns "+ifacename+" "+nameserver, true)
				if err != nil {
					log.Println("WARNING: Error encountered running resolvectl dns " + ifacename + " " + nameserver)
				}
			}
		}
	}
	return err
}

// IsDNSReachable - checks if nameserver is reachable
func IsDNSReachable(nameserver string) bool {
	port := "53"
	protocols := [2]string{"tcp", "udp"}
	for _, proto := range protocols {
		timeout := time.Second
		conn, err := net.DialTimeout(proto, net.JoinHostPort(nameserver, port), timeout)
		if err != nil {
			return false
		}
		if conn != nil {
			defer conn.Close()
		} else {
			return false
		}
	}
	return true
}

// IsDNSWorking - checks if record is returned by correct nameserver
func IsDNSWorking(network string, nameserver string) bool {
	var isworking bool
	servers, err := net.LookupNS("netmaker" + "." + network)
	if err != nil {
		return isworking
	}
	for _, ns := range servers {
		if strings.Contains(ns.Host, nameserver) {
			isworking = true
		}
	}
	return isworking
}
