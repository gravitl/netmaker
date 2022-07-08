package upgrades

import "github.com/gravitl/netmaker/netclient/config"

// UpgradeFunction - logic for upgrade
type UpgradeFunction func(*config.ClientConfig)

// UpgradeInfo - struct for holding upgrade info
type UpgradeInfo struct {
	RequiredVersions []string
	NewVersion       string
	OP               UpgradeFunction
}
