package logic

import (
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
)

// EnterpriseCheckFuncs - can be set to run functions for EE
var EnterpriseCheckFuncs []func()

// EnterpriseFailoverFunc - interface to control failover funcs
var EnterpriseFailoverFunc func(node *models.Node) error

// EnterpriseResetFailoverFunc - interface to control reset failover funcs
var EnterpriseResetFailoverFunc func(network string) error

// EnterpriseResetAllPeersFailovers - resets all nodes that are considering a node to be failover worthy (inclusive)
var EnterpriseResetAllPeersFailovers func(nodeid uuid.UUID, network string) error

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

func GetCurrentServerUsage() (usage models.Usage) {
	usage.SetDefaults()
	hosts, hErr := GetAllHosts()
	if hErr == nil {
		usage.Hosts = len(hosts)
	}
	clients, cErr := GetAllExtClients()
	if cErr == nil {
		usage.Clients = len(clients)
	}
	users, err := GetUsers()
	if err == nil {
		usage.Users = len(users)
	}
	networks, err := GetNetworks()
	if err == nil {
		usage.Networks = len(networks)
	}
	// TODO this part bellow can be optimized to get nodes just once
	ingresses, err := GetAllIngresses()
	if err == nil {
		usage.Ingresses = len(ingresses)
	}
	egresses, err := GetAllEgresses()
	if err == nil {
		usage.Egresses = len(egresses)
	}
	relays, err := GetRelays()
	if err == nil {
		usage.Relays = len(relays)
	}
	gateways, err := GetInternetGateways()
	if err == nil {
		usage.InternetGateways = len(gateways)
	}
	return
}
