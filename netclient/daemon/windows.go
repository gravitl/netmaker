package daemon

import (
	"strings"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// RestartWindowsDaemon - restarts windows service
func RestartWindowsDaemon() {
	StopWindowsDaemon()
	// start daemon, will not restart or start another
	ncutils.RunCmd(strings.Replace(ncutils.GetNetclientPathSpecific(), `\\`, `\`, -1)+`winsw.exe start`, false)
}

// == Daemon ==

// StopWindowsDaemon - stops the Windows daemon
func StopWindowsDaemon() {
	logger.Log(0, "stopping Windows, Netclient daemon")
	// stop daemon, will not overwrite
	ncutils.RunCmd(strings.Replace(ncutils.GetNetclientPathSpecific(), `\\`, `\`, -1)+`winsw.exe stop`, true)
}
