package upgrades

import (
	"github.com/gravitl/netmaker/netclient/config"
)

var upgrade0162 = UpgradeInfo{
	RequiredVersions: []string{
		"v0.14.6",
		"v0.15.0",
		"v0.15.1",
		"v0.15.2",
		"v0.16.1",
	},
	NewVersion: "v0.16.2",
	OP:         update0162,
}

func update0162(cfg *config.ClientConfig) {
	// set connect default if not present 15.X -> 16.0
	update0161(cfg)
}
