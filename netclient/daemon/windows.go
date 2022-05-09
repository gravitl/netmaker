package daemon

import (
	"fmt"
	"strings"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// RestartWindowsDaemon - restarts windows service
func RestartWindowsDaemon() {
	StopWindowsDaemon()
	// start daemon, will not restart or start another
	dirPath := strings.Replace(ncutils.GetNetclientPathSpecific(), `\\`, `\`, -1)
	winCmd := fmt.Sprintf(`"%swinsw.exe" "start"`, dirPath)
	out, err := ncutils.RunCmdFormatted(winCmd, true)
	if err != nil {
		logger.Log(0, "error starting Windows, Netclient daemon: "+err.Error()+" : "+out)
	} else {
		logger.Log(0, "started Windows Netclient daemon")
	}
}

// == Daemon ==

// StopWindowsDaemon - stops the Windows daemon
func StopWindowsDaemon() {
	// stop daemon, will not overwrite
	dirPath := strings.Replace(ncutils.GetNetclientPathSpecific(), `\\`, `\`, -1)
	winCmd := fmt.Sprintf(`"%swinsw.exe" "stop"`, dirPath)
	out, err := ncutils.RunCmdFormatted(winCmd, true)
	if err != nil {
		logger.Log(0, "error stopping Windows, Netclient daemon: "+err.Error()+" : "+out)
	} else {
		logger.Log(0, "stopped Windows Netclient daemon")
	}
}
