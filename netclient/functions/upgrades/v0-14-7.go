package upgrades

import (
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/config"
)

var upgrade0147 = UpgradeInfo{
	RequiredVersions: []string{
		"v0.14.0",
		"v0.14.1",
		"v0.14.2",
		"v0.14.3",
		"v0.14.4",
		"v0.14.5",
		"v0.14.6",
	},
	NewVersion: "v0.14.7",
	OP:         update0147,
}

func update0147(cfg *config.ClientConfig) {
	//do stuff for 14.X -> 14.6
	// No-op
	/*
		if runtime.GOARCH == "darwin" {
			oldLocation := "/Applications/Netclient"
			newLocation := ncutils.MAC_APP_DATA_PATH
			err := os.Rename(oldLocation, newLocation)
			if err != nil {
				logger.FatalLog("There was an issue moving the Netclient file from Applications to Application Support:", err.Error())
			} else {
				logger.Log(0, "The Netclient data file has been moved from Applications to Application Support")
			}

		}
	*/
	logger.Log(0, "updating schema for v0.14.7")
}
