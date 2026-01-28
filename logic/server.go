package logic

import (
	"context"
	"sync"

	"github.com/gravitl/netmaker/models"
)

// EnterpriseCheckFuncs - can be set to run functions for EE
var EnterpriseCheckFuncs []func(ctx context.Context, wg *sync.WaitGroup)
var GetFeatureFlags = func() models.FeatureFlags {
	return models.FeatureFlags{}
}
var GetDeploymentMode = func() string {
	// All CE deployments are self-hosted.
	return "self-hosted"
}
var StartFlowCleanupLoop = func() {}
var StopFlowCleanupLoop = func() {}

// == Join, Checkin, and Leave for Server ==

// KUBERNETES_LISTEN_PORT - starting port for Kubernetes in order to use NodePort range
const KUBERNETES_LISTEN_PORT = 31821

// KUBERNETES_SERVER_MTU - ideal mtu for kubernetes deployments right now
const KUBERNETES_SERVER_MTU = 1024

// EnterpriseCheck - Runs enterprise functions if presented
func EnterpriseCheck(ctx context.Context, wg *sync.WaitGroup) {
	for _, check := range EnterpriseCheckFuncs {
		check(ctx, wg)
	}
}
