package functions

import (
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/daemon"
)

//Install - installs binary/daemon
func Install() error {
	daemon.Stop()
	if err := daemon.InstallDaemon(); err != nil {
		logger.Log(0, "error installing daemon", err.Error())
		return err
	}
	return daemon.Restart()
}
