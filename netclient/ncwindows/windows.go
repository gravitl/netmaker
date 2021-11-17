package ncwindows

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

// InitWindows - Initialize windows directory & files and such
func InitWindows() {

	_, directoryErr := os.Stat(ncutils.GetNetclientPath()) // Check if data directory exists or not
	if os.IsNotExist(directoryErr) {                       // create a data directory
		os.Mkdir(ncutils.GetNetclientPath(), 0755)
	}
	wdPath, wdErr := os.Getwd() // get the current working directory
	if wdErr != nil {
		log.Fatal("failed to get current directory..")
	}
	_, dataNetclientErr := os.Stat(ncutils.GetNetclientPathSpecific() + "netclient.exe")
	_, currentNetclientErr := os.Stat(wdPath + "\\netclient.exe")

	if os.IsNotExist(dataNetclientErr) { // check and see if netclient.exe is in appdata
		if currentNetclientErr == nil { // copy it if it exists locally
			input, err := ioutil.ReadFile(wdPath + "\\netclient.exe")
			if err != nil {
				log.Println("failed to find netclient.exe")
				return
			}
			if err = ioutil.WriteFile(ncutils.GetNetclientPathSpecific()+"netclient.exe", input, 0644); err != nil {
				log.Println("failed to copy netclient.exe to", ncutils.GetNetclientPath())
				return
			}
		} else {
			log.Fatalf("[netclient] netclient.exe not found in current working directory: %s \nexiting.", wdPath)
		}
	}
	log.Println("Gravitl Netclient on Windows started")
}
