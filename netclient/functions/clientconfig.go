package functions

import (
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/functions/upgrades"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// UpdateClientConfig - function is called on daemon start to update clientConfig if required
// Usage :  set update required to true and and update logic to function
func UpdateClientConfig() {
	defer upgrades.ReleaseUpgrades()

	networks, _ := ncutils.GetSystemNetworks()
	if len(networks) == 0 {
		return
	}
	logger.Log(0, "updating netclient...")
	for _, network := range networks {
		cfg := config.ClientConfig{}
		cfg.Network = network
		cfg.ReadConfig()
		//update any new fields
		configChanged := false
		for _, u := range upgrades.Upgrades {
			if cfg.Node.Version == u.RequiredVersion {
				upgrades.UpgradeFunction(u.OP)(&cfg)
				cfg.Node.Version = u.NewVersion
				configChanged = true
			}
		}
		//insert update logic here
		if configChanged {
			logger.Log(0, "updating clientConfig for network", cfg.Network)
			if err := config.Write(&cfg, cfg.Network); err != nil {
				logger.Log(0, "failed to update clientConfig for ", cfg.Network, err.Error())
			}
		}
	}
	logger.Log(0, "finished updates")
}
