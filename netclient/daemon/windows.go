package daemon

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// SetupWindowsDaemon - sets up the Windows daemon service
func SetupWindowsDaemon() error {

	if ncutils.FileExists(ncutils.GetNetclientPathSpecific() + "winsw.xml") {
		logger.Log(0, "updating netclient service")
	}
	if err := writeServiceConfig(); err != nil {
		return err
	}

	if ncutils.FileExists(ncutils.GetNetclientPathSpecific() + "winsw.exe") {
		logger.Log(0, "updating netclient binary")
	}
	err := ncutils.GetEmbedded()
	if err != nil {
		return err
	}
	logger.Log(0, "finished daemon setup")
	//get exact formatted commands
	RunWinSWCMD("install")
	time.Sleep(time.Millisecond)
	RunWinSWCMD("start")

	return nil
}

// RestartWindowsDaemon - restarts windows service
func RestartWindowsDaemon() {
	RunWinSWCMD("stop")
	time.Sleep(time.Millisecond)
	RunWinSWCMD("start")
}

// CleanupWindows - cleans up windows files
func CleanupWindows() {
	if !ncutils.FileExists(ncutils.GetNetclientPathSpecific() + "winsw.xml") {
		writeServiceConfig()
	}
	RunWinSWCMD("stop")
	RunWinSWCMD("uninstall")
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

// RunWinSWCMD - Run a command with the winsw.exe tool (start, stop, install, uninstall)
func RunWinSWCMD(command string) {

	// check if command allowed
	allowedCommands := map[string]bool{
		"start":     true,
		"stop":      true,
		"install":   true,
		"uninstall": true,
	}
	if !allowedCommands[command] {
		logger.Log(0, "command "+command+" unsupported by winsw")
		return
	}

	// format command
	dirPath := strings.Replace(ncutils.GetNetclientPathSpecific(), `\\`, `\`, -1)
	winCmd := fmt.Sprintf(`"%swinsw.exe" "%s"`, dirPath, command)
	logger.Log(0, "running "+command+" of Windows Netclient daemon")

	// run command and log for success/failure
	out, err := ncutils.RunCmdFormatted(winCmd, true)
	if err != nil {
		logger.Log(0, "error with "+command+" of Windows Netclient daemon: "+err.Error()+" : "+out)
	} else {
		logger.Log(0, "successfully ran "+command+" of Windows Netclient daemon")
	}
}
