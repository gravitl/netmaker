package upgrades

import (
	"github.com/gravitl/netmaker/netclient/config"
)

var upgrade0160 = UpgradeInfo{
	RequiredVersions: []string{
		"v0.14.6",
		"v0.15.0",
		"v0.15.1",
		"v0.15.2",
	},
	NewVersion: "v0.16.0",
	OP:         update0160,
}

func update0160(cfg *config.ClientConfig) {
	// set connect default if not present 15.X -> 16.0
	if cfg.Node.Connected == "" {
		cfg.Node.SetDefaultConnected()
	}
}
