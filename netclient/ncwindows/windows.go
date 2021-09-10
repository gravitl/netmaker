package ncwindows

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/gravitl/netmaker/netclient/netclientutils"
)

// Initialize windows directory & files and such
func InitWindows() {

	_, directoryErr := os.Stat(netclientutils.GetNetclientPath()) // Check if data directory exists or not
	if os.IsNotExist(directoryErr) {                              // create a data directory
		os.Mkdir(netclientutils.GetNetclientPath(), 0755)
	}
	wdPath, wdErr := os.Getwd() // get the current working directory
	if wdErr != nil {
		log.Fatal("failed to get current directory..")
	}
	_, dataNetclientErr := os.Stat(netclientutils.GetNetclientPathSpecific() + "netclient.exe")
	_, currentNetclientErr := os.Stat(wdPath + "\\netclient.exe")
	if os.IsNotExist(dataNetclientErr) { // check and see if netclient.exe is in appdata
		if currentNetclientErr == nil { // copy it if it exists locally
			input, err := ioutil.ReadFile(wdPath + "\\netclient.exe")
			if err != nil {
				log.Println("failed to find netclient.exe")
				return
			}
			if err = ioutil.WriteFile(netclientutils.GetNetclientPathSpecific()+"netclient.exe", input, 0644); err != nil {
				log.Println("failed to copy netclient.exe to", netclientutils.GetNetclientPath())
				return
			}
		}
	}
	log.Println("Gravitl Netclient on Windows started")
}
