package functions

import (
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
)

func InstallDaemon(cfg config.ClientConfig) error {

	var err error
	err = local.ConfigureSystemD(cfg.Network)
	return err
}
