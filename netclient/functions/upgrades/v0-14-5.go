package upgrades

import (
	"github.com/netmakerio/netmaker/logger"
	"github.com/netmakerio/netmaker/netclient/config"
)

var upgrade0145 = UpgradeInfo{
	RequiredVersions: []string{
		"v0.14.0",
		"v0.14.1",
		"v0.14.2",
		"v0.14.3",
		"v0.14.4",
	},
	NewVersion: "v0.14.5",
	OP:         update0145,
}

func update0145(cfg *config.ClientConfig) {
	// do stuff for 14.X -> 14.5
	// No-op
	logger.Log(0, "updating schema for 0.14.5")
}
