package logic

// EnterpriseCheckFuncs - can be set to run functions for EE
var EnterpriseCheckFuncs []func()

// == Join, Checkin, and Leave for Server ==

// KUBERNETES_LISTEN_PORT - starting port for Kubernetes in order to use NodePort range
const KUBERNETES_LISTEN_PORT = 31821

// KUBERNETES_SERVER_MTU - ideal mtu for kubernetes deployments right now
const KUBERNETES_SERVER_MTU = 1024

// EnterpriseCheck - Runs enterprise functions if presented
func EnterpriseCheck() {
	for _, check := range EnterpriseCheckFuncs {
		check()
	}
}
