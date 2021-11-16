package local

import (
	"io/ioutil"
	"os"
	"strings"

	//"github.com/davecgh/go-spew/spew"
	"log"
	"os/exec"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

// SetDNS - sets the DNS of a local machine
func SetDNS(nameserver string) error {
	bytes, err := ioutil.ReadFile("/etc/resolv.conf")
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
	if ncutils.IsWindows() {
		return nil
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
