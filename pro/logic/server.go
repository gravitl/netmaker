package logic

import "github.com/gravitl/netmaker/models"

var featureFlagsCache models.FeatureFlags

func SetFeatureFlags(featureFlags models.FeatureFlags) {
	featureFlagsCache = featureFlags
}

func GetFeatureFlags() models.FeatureFlags {
	return models.FeatureFlags{
		EnableEgressHA:          true,
		EnableNetworkActivity:   true,
		EnableOAuth:             true,
		EnableIDPIntegration:    true,
		AllowMultiServerLicense: false,
		EnableGwsHA:             true,
		EnableDeviceApproval:    true,
	}
}
