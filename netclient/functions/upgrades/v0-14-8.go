package upgrades

import (
	"github.com/gravitl/netmaker/netclient/config"
)

var upgrade0148 = UpgradeInfo{
	RequiredVersions: []string{
		"v0.14.5",
		"v0.14.6",
		"v0.14.7",
	},
	NewVersion: "v0.14.8",
	OP:         update0148,
}

func update0148(cfg *config.ClientConfig) {
	// do stuff for 14.X -> 14.5
	if cfg.Node.Connected == "" {
		cfg.Node.SetDefaultConnected()
	}
}
