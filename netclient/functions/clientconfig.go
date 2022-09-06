package functions

import (
	"strconv"
	"strings"

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
	logger.Log(0, "checking for netclient updates...")
	for _, network := range networks {
		cfg := config.ClientConfig{}
		cfg.Network = network
		cfg.ReadConfig()
		major, minor, _ := Version(cfg.Node.Version)
		if major == 0 && minor < 14 {
			logger.Log(0, "schema of network", cfg.Network, "is out of date and cannot be updated\n Correct behaviour of netclient cannot be guaranteed")
			continue
		}
		configChanged := false
		for _, u := range upgrades.Upgrades {
			if ncutils.StringSliceContains(u.RequiredVersions, cfg.Node.Version) {
				logger.Log(0, "upgrading node", cfg.Node.Name, "on network", cfg.Node.Network, "from", cfg.Node.Version, "to", u.NewVersion)
				upgrades.UpgradeFunction(u.OP)(&cfg)
				cfg.Node.Version = u.NewVersion
				configChanged = true
			}
		}
		if configChanged {
			//save and publish
			if err := PublishNodeUpdate(&cfg); err != nil {
				logger.Log(0, "error publishing node update during schema change", err.Error())
			}
			if err := config.ModNodeConfig(&cfg.Node); err != nil {
				logger.Log(0, "error saving local config for node,", cfg.Node.Name, ", on network,", cfg.Node.Network)
			}
		}
	}
	logger.Log(0, "finished updates")
}

// Version - parse version string into component parts
// version string must be semantic version of form 1.2.3 or v1.2.3
// otherwise 0, 0, 0 will be returned.
func Version(version string) (int, int, int) {
	var major, minor, patch int
	var errMajor, errMinor, errPatch error
	parts := strings.Split(version, ".")
	//ensure semantic version
	if len(parts) < 3 {
		return major, minor, patch
	}
	if strings.Contains(parts[0], "v") {
		majors := strings.Split(parts[0], "v")
		major, errMajor = strconv.Atoi(majors[1])
	} else {
		major, errMajor = strconv.Atoi(parts[0])
	}
	minor, errMinor = strconv.Atoi(parts[1])
	patch, errPatch = strconv.Atoi(parts[2])
	if errMajor != nil || errMinor != nil || errPatch != nil {
		return 0, 0, 0
	}
	return major, minor, patch
}
