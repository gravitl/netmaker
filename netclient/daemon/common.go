package daemon

import (
	"errors"
	"runtime"

	"github.com/gravitl/netmaker/netclient/config"
)

func InstallDaemon(cfg config.ClientConfig) error {
	os := runtime.GOOS
	var err error
	switch os {
	case "windows":
		err = SetupWindowsDaemon()
	case "darwin":
		err = SetupMacDaemon()
	case "linux":
		err = SetupSystemDDaemon(cfg.Network)
	default:
		err = errors.New("this os is not yet supported for daemon mode. Run join cmd with flag '--daemon off'")
	}
	return err
}
