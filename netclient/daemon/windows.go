package daemon

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// SetupWindowsDaemon - sets up the Windows daemon service
func SetupWindowsDaemon() error {

	if !ncutils.FileExists(ncutils.GetNetclientPathSpecific() + "winsw.xml") {
		if err := writeServiceConfig(); err != nil {
			return err
		}
	}

	if !ncutils.FileExists(ncutils.GetNetclientPathSpecific() + "winsw.exe") {
		logger.Log(0, "performing first time daemon setup")
		err := ncutils.GetEmbedded()
		if err != nil {
			return err
		}
		logger.Log(0, "finished daemon setup")
	}
	// install daemon, will not overwrite
	ncutils.RunCmd(strings.Replace(ncutils.GetNetclientPathSpecific(), `\\`, `\`, -1)+`winsw.exe install`, false)
	// start daemon, will not restart or start another
	ncutils.RunCmd(strings.Replace(ncutils.GetNetclientPathSpecific(), `\\`, `\`, -1)+`winsw.exe start`, false)
	logger.Log(0, strings.Replace(ncutils.GetNetclientPathSpecific(), `\\`, `\`, -1)+`winsw.exe start`)
	return nil
}

// RestartWindowsDaemon - restarts windows service
func RestartWindowsDaemon() {
	StopWindowsDaemon()
	// start daemon, will not restart or start another
	ncutils.RunCmd(strings.Replace(ncutils.GetNetclientPathSpecific(), `\\`, `\`, -1)+`winsw.exe start`, false)
}

// CleanupWindows - cleans up windows files
func CleanupWindows() {
	if !ncutils.FileExists(ncutils.GetNetclientPathSpecific() + "winsw.xml") {
		writeServiceConfig()
	}
	StopWindowsDaemon()
	RemoveWindowsDaemon()
	os.RemoveAll(ncutils.GetNetclientPath())
	log.Println("Netclient on Windows, uninstalled")
}

func writeServiceConfig() error {
	serviceConfigPath := ncutils.GetNetclientPathSpecific() + "winsw.xml"
	scriptString := fmt.Sprintf(`<service>
<id>netclient</id>
<name>Netclient</name>
<description>Connects Windows nodes to one or more Netmaker networks.</description>
<executable>%v</executable>
<arguments>daemon</arguments>
<log mode="roll"></log>
</service>
`, strings.Replace(ncutils.GetNetclientPathSpecific()+"netclient.exe", `\\`, `\`, -1))
	if !ncutils.FileExists(serviceConfigPath) {
		err := os.WriteFile(serviceConfigPath, []byte(scriptString), 0600)
		if err != nil {
			return err
		}
		logger.Log(0, "wrote the daemon config file to the Netclient directory")
	}
	return nil
}

// == Daemon ==

// StopWindowsDaemon - stops the Windows daemon
func StopWindowsDaemon() {
	logger.Log(0, "stopping Windows, Netclient daemon")
	// stop daemon, will not overwrite
	ncutils.RunCmd(strings.Replace(ncutils.GetNetclientPathSpecific(), `\\`, `\`, -1)+`winsw.exe stop`, true)
}

// RemoveWindowsDaemon - removes the Windows daemon
func RemoveWindowsDaemon() {
	// uninstall daemon, will not restart or start another
	ncutils.RunCmd(strings.Replace(ncutils.GetNetclientPathSpecific(), `\\`, `\`, -1)+`winsw.exe uninstall`, true)
	logger.Log(0, "uninstalled Windows, Netclient daemon")
}

// func copyWinswOver() error {

// 	input, err := ioutil.ReadFile(".\\winsw.exe")
// 	if err != nil {
// 		logger.Log(0, "failed to find winsw.exe")
// 		return err
// 	}
// 	if err = ioutil.WriteFile(ncutils.GetNetclientPathSpecific()+"winsw.exe", input, 0644); err != nil {
// 		logger.Log(0, "failed to copy winsw.exe to " + ncutils.GetNetclientPath())
// 		return err
// 	}
// 	if err = os.Remove(".\\winsw.exe"); err != nil {
// 		logger.Log(0, "failed to cleanup local winsw.exe, feel free to delete it")
// 		return err
// 	}
// 	logger.Log(0, "finished copying winsw.exe")
// 	return nil
// }

// func downloadWinsw() error {
// 	fullURLFile := "https://github.com/winsw/winsw/releases/download/v2.11.0/WinSW-x64.exe"
// 	fileName := "winsw.exe"

// 	// Create the file
// 	file, err := os.Create(fileName)
// 	if err != nil {
// 		logger.Log(0, "could not create file on OS for Winsw")
// 		return err
// 	}
// 	defer file.Close()

// 	client := http.Client{
// 		CheckRedirect: func(r *http.Request, via []*http.Request) error {
// 			r.URL.Opaque = r.URL.Path
// 			return nil
// 		},
// 	}
// 	// Put content on file
// 	logger.Log(0, "downloading service tool...")
// 	resp, err := client.Get(fullURLFile)
// 	if err != nil {
// 		logger.Log(0, "could not GET Winsw")
// 		return err
// 	}
// 	defer resp.Body.Close()

// 	_, err = io.Copy(file, resp.Body)
// 	if err != nil {
// 		logger.Log(0, "could not mount winsw.exe")
// 		return err
// 	}
// 	logger.Log(0, "finished downloading Winsw")
// 	return nil
// }
