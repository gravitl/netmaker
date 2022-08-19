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
	// set connect default if not present 14.X -> 14.8
	if cfg.Node.Connected == "" {
		cfg.Node.SetDefaultConnected()
	}
}
