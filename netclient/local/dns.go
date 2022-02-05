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

	"github.com/gravitl/netmaker/netclient/ncutils"
)

const DNS_UNREACHABLE_ERROR = "nameserver unreachable"

func SetDNSWithRetry(iface, network, address string) {
	var reachable bool
	for counter := 0; !reachable && counter < 5; counter++ {
		reachable = IsDNSReachable(address)
		time.Sleep(time.Second << 1)
	}
	if !reachable {
		ncutils.Log("not setting dns, server unreachable: " + address)
	} else if err := UpdateDNS(iface, network, address); err != nil {
		ncutils.Log("error applying dns" + err.Error())
	}
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

// UpdateDNS - updates local DNS of client
func UpdateDNS(ifacename string, network string, nameserver string) error {
	if ifacename == "" {
		return fmt.Errorf("cannot set dns: interface name is blank")
	}
	if network == "" {
		return fmt.Errorf("cannot set dns: network name is blank")
	}
	if nameserver == "" {
		return fmt.Errorf("cannot set dns: nameserver is blank")
	}
	if ncutils.IsWindows() {
		return nil
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
