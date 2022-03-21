package ncwindows

import (
	"log"
	"os"

	"github.com/gravitl/netmaker/logger"
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

	dataPath := ncutils.GetNetclientPathSpecific() + "netclient.exe"
	currentPath := wdPath + "\\netclient.exe"
	_, dataNetclientErr := os.Stat(dataPath)
	_, currentNetclientErr := os.Stat(currentPath)

	if currentPath == dataPath && currentNetclientErr == nil {
		logger.Log(0, "netclient.exe is in proper location, "+currentPath)
	} else if os.IsNotExist(dataNetclientErr) { // check and see if netclient.exe is in appdata
		if currentNetclientErr == nil { // copy it if it exists locally
			input, err := os.ReadFile(currentPath)
			if err != nil {
				log.Println("failed to find netclient.exe")
				return
			}
			if err = os.WriteFile(dataPath, input, 0700); err != nil {
				log.Println("failed to copy netclient.exe to", ncutils.GetNetclientPath())
				return
			}
		} else {
			log.Fatalf("[netclient] netclient.exe not found in current working directory: %s \nexiting.", wdPath)
		}
	}
	log.Println("Gravitl Netclient on Windows started")
}
