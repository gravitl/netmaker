package logic

import (
	"github.com/gravitl/netmaker/models"
)

var featureFlagsCache models.FeatureFlags
var DeploymentMode string

func SetFeatureFlags(featureFlags models.FeatureFlags) {
	featureFlagsCache = featureFlags
}

// EnableAllFeatureFlags turns on every pro feature. Used by airgapped builds
// that cannot retrieve flags from license validation.
func EnableAllFeatureFlags() {
	featureFlagsCache = models.FeatureFlags{
		EnableEgressHA:                true,
		EnableNetworkActivity:         true,
		EnableOAuth:                   true,
		EnableIDPIntegration:          true,
		AllowMultiServerLicense:       true,
		EnableGwsHA:                   true,
		EnableDeviceApproval:          true,
		EnableFlowLogs:                true,
		EnablePostureChecks:           true,
		EnableJIT:                     true,
		EnableOverlappingEgressRanges: true,
	}
}

func GetFeatureFlags() models.FeatureFlags {
	return featureFlagsCache
}

func SetDeploymentMode(deploymentMode string) {
	DeploymentMode = deploymentMode
}

func GetDeploymentMode() string {
	return DeploymentMode
}
