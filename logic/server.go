package logic

import (
	"strings"

	"github.com/gravitl/netmaker/models"
)

// EnterpriseCheckFuncs - can be set to run functions for EE
var EnterpriseCheckFuncs []func()

// EnterpriseFailoverFunc - interface to control failover funcs
var EnterpriseFailoverFunc func(node *models.Node) error

// EnterpriseResetFailoverFunc - interface to control reset failover funcs
var EnterpriseResetFailoverFunc func(network string) error

// EnterpriseResetAllPeersFailovers - resets all nodes that are considering a node to be failover worthy (inclusive)
var EnterpriseResetAllPeersFailovers func(nodeid, network string) error

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

// == Private ==

func isDeleteError(err error) bool {
	return err != nil && strings.Contains(err.Error(), models.NODE_DELETE)
}
