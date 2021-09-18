package local

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gravitl/netmaker/netclient/netclientutils"
)

func IsWindowsWGInstalled() bool {
	out, err := RunCmd("wg help", true)
	if err != nil {
		return false
	}
	return strings.Contains(out, "Available subcommand")
}

func ApplyWindowsConf(confPath string) error {
	if _, err := RunCmd("wireguard.exe /installtunnelservice "+confPath, true); err != nil {
		return err
	}
	return nil
}

func RemoveWindowsConf(ifacename string) error {
	if _, err := RunCmd("wireguard.exe /uninstalltunnelservice "+ifacename, true); err != nil {
		return err
	}
	return nil
}

func writeServiceConfig() error {
	serviceConfigPath := netclientutils.GetNetclientPathSpecific() + "winsw.xml"
	scriptString := fmt.Sprintf(`<service>
<id>netclient</id>
<name>Netclient</name>
<description>Connects Windows nodes to one or more Netmaker networks.</description>
<executable>%v</executable>
<log mode="roll"></log>
</service>
`, strings.Replace(netclientutils.GetNetclientPathSpecific()+"netclient.exe", `\\`, `\`, -1))
	if !FileExists(serviceConfigPath) {
		err := ioutil.WriteFile(serviceConfigPath, []byte(scriptString), 0644)
		if err != nil {
			return err
		}
		netclientutils.Log("wrote the daemon config file to the Netclient directory")
	}
	return nil
}

// == Daemon ==
func StopWindowsDaemon() {
	netclientutils.Log("no networks detected, stopping Windows, Netclient daemon")
	// stop daemon, will not overwrite
	RunCmd(strings.Replace(netclientutils.GetNetclientPathSpecific(), `\\`, `\`, -1)+`winsw.exe stop`, true)
}

func RemoveWindowsDaemon() {
	// uninstall daemon, will not restart or start another
	RunCmd(strings.Replace(netclientutils.GetNetclientPathSpecific(), `\\`, `\`, -1)+`winsw.exe uninstall`, true)
	netclientutils.Log("uninstalled Windows, Netclient daemon")
}

func copyWinswOver() error {

	input, err := ioutil.ReadFile(".\\winsw.exe")
	if err != nil {
		netclientutils.Log("failed to find winsw.exe")
		return err
	}
	if err = ioutil.WriteFile(netclientutils.GetNetclientPathSpecific()+"winsw.exe", input, 0644); err != nil {
		netclientutils.Log("failed to copy winsw.exe to " + netclientutils.GetNetclientPath())
		return err
	}
	if err = os.Remove(".\\winsw.exe"); err != nil {
		netclientutils.Log("failed to cleanup local winsw.exe, feel free to delete it")
		return err
	}
	netclientutils.Log("finished copying winsw.exe")
	return nil
}

func downloadWinsw() error {
	fullURLFile := "https://github.com/winsw/winsw/releases/download/v2.11.0/WinSW-x64.exe"
	fileName := "winsw.exe"

	// Create the file
	file, err := os.Create(fileName)
	if err != nil {
		netclientutils.Log("could not create file on OS for Winsw")
		return err
	}
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}
	// Put content on file
	netclientutils.Log("downloading service tool...")
	resp, err := client.Get(fullURLFile)
	if err != nil {
		netclientutils.Log("could not GET Winsw")
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		netclientutils.Log("could not mount winsw.exe")
		return err
	}
	defer file.Close()
	netclientutils.Log("finished downloading Winsw")
	return nil
}

func CreateAndRunMacDaemon() error {
	log.Println("TODO: Create Mac Daemon")
	return errors.New("no mac daemon yet")
}

func CreateAndRunWindowsDaemon() error {

	if !FileExists(netclientutils.GetNetclientPathSpecific() + "winsw.xml") {
		if err := writeServiceConfig(); err != nil {
			return err
		}
	}

	if !FileExists(netclientutils.GetNetclientPathSpecific() + "winsw.exe") {
		netclientutils.Log("performing first time daemon setup")
		if !FileExists(".\\winsw.exe") {
			err := downloadWinsw()
			if err != nil {
				return err
			}
		}
		err := copyWinswOver()
		if err != nil {
			return err
		}
		netclientutils.Log("finished daemon setup")
	}
	// install daemon, will not overwrite
	RunCmd(strings.Replace(netclientutils.GetNetclientPathSpecific(), `\\`, `\`, -1)+`winsw.exe install`, true)
	// start daemon, will not restart or start another
	RunCmd(strings.Replace(netclientutils.GetNetclientPathSpecific(), `\\`, `\`, -1)+`winsw.exe start`, true)
	netclientutils.Log(strings.Replace(netclientutils.GetNetclientPathSpecific(), `\\`, `\`, -1) + `winsw.exe start`)
	return nil
}

func CleanupWindows() {
	if !FileExists(netclientutils.GetNetclientPathSpecific() + "winsw.xml") {
		writeServiceConfig()
	}
	StopWindowsDaemon()
	RemoveWindowsDaemon()
	os.RemoveAll(netclientutils.GetNetclientPath())
	log.Println("Netclient on Windows, uninstalled")
}

func CleanupMac() {
	//StopWindowsDaemon()
	//RemoveWindowsDaemon()
	//os.RemoveAll(netclientutils.GetNetclientPath())
	log.Println("TODO: Not implemented yet")
}
