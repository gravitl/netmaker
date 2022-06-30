package functions

import (
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

var updateRequired = false

// UpdateClientConfig - function is called on daemon start to update clientConfig if required
// Usage :  set update required to true and and update logic to function
func UpdateClientConfig() {
	if !updateRequired {
		return
	}
	networks, _ := ncutils.GetSystemNetworks()
	if len(networks) == 0 {
		return
	}
	for _, network := range networks {
		cfg := config.ClientConfig{}
		cfg.Network = network
		cfg.ReadConfig()
		//update any new fields
		logger.Log(0, "updating clientConfig for network", cfg.Network)
		//insert update logic here
		if err := config.Write(&cfg, cfg.Network); err != nil {
			logger.Log(0, "failed to update clientConfig for ", cfg.Network, err.Error())
		}
	}
	//reset so future calls will return immediately
	updateRequired = false
}
