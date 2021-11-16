package daemon

import (
	"errors"
	"runtime"

	"github.com/gravitl/netmaker/netclient/config"
)

func InstallDaemon(cfg config.ClientConfig) error {
	os := runtime.GOOS
	var err error

	interval := "15"
	if cfg.Server.CheckinInterval != "" {
		interval = cfg.Server.CheckinInterval
	}

	switch os {
	case "windows":
		err = SetupWindowsDaemon()
	case "darwin":
		err = SetupMacDaemon(interval)
	case "linux":
		err = SetupSystemDDaemon(interval)
	default:
		err = errors.New("this os is not yet supported for daemon mode. Run join cmd with flag '--daemon off'")
	}
	return err
}
