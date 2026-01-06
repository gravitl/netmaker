package logic

import (
	"github.com/gravitl/netmaker/models"
)

var featureFlagsCache models.FeatureFlags
var DeploymentMode string

func SetFeatureFlags(featureFlags models.FeatureFlags) {
	featureFlagsCache = featureFlags
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
