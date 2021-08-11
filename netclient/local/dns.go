package local

import (
	"io/ioutil"
	"os"
	"strings"

	//"github.com/davecgh/go-spew/spew"
	"log"
	"os/exec"
)

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

func UpdateDNS(ifacename string, network string, nameserver string) error {
	_, err := exec.LookPath("resolvectl")
	if err != nil {
		log.Println(err)
		log.Println("WARNING: resolvectl not present. Unable to set dns. Install resolvectl or run manually.")
	} else {
		_, err = RunCmd("resolvectl domain " + ifacename + " ~" + network)
		if err != nil {
			log.Println(err)
			log.Println("WARNING: Error encountered setting domain on dns. Aborted setting dns.")
		} else {
			_, err = RunCmd("resolvectl default-route " + ifacename + " false")
			if err != nil {
				log.Println(err)
				log.Println("WARNING: Error encountered setting default-route on dns. Aborted setting dns.")
			} else {
				_, err = RunCmd("resolvectl dns " + ifacename + " " + nameserver)
				if err != nil {
					log.Println("WARNING: Error encountered running resolvectl dns " + ifacename + " " + nameserver)
					log.Println(err)
				}
			}
		}
	}
	return err
}
