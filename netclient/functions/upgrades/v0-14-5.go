package upgrades

import "github.com/gravitl/netmaker/netclient/config"

var upgrade0145 = UpgradeInfo{
	RequiredVersions: []string{"0.14.1", "0.14.2", "0.14.3", "0.14.4"},
	NewVersion:       "0.14.5",
	OP:               update0145,
}

func update0145(cfg *config.ClientConfig) {
	// do stuff for 14.4 -> 14.5
	// No-op
}
