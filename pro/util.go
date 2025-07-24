//go:build ee
// +build ee

package pro

import (
	"encoding/base64"
	"github.com/gravitl/netmaker/models"

	"github.com/gravitl/netmaker/logic"
)

// base64encode - base64 encode helper function
func base64encode(input []byte) string {
	return base64.StdEncoding.EncodeToString(input)
}

// base64decode - base64 decode helper function
func base64decode(input string) []byte {
	bytes, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return nil
	}

	return bytes
}

func getCurrentServerUsage() (limits Usage) {
	limits.SetDefaults()
	hosts, hErr := logic.GetAllHostsWithStatus(models.OnlineSt)
	if hErr == nil {
		limits.Hosts = len(hosts)
	}
	clients, cErr := logic.GetAllExtClientsWithStatus(models.OnlineSt)
	if cErr == nil {
		limits.Clients = len(clients)
	}
	users, err := logic.GetUsers()
	if err == nil {
		limits.Users = len(users)
	}
	networks, err := logic.GetNetworks()
	if err == nil {
		limits.Networks = len(networks)
	}
	// TODO this part bellow can be optimized to get nodes just once
	ingresses, err := logic.GetAllIngresses()
	if err == nil {
		limits.Ingresses = len(ingresses)
	}
	egresses, err := logic.GetAllEgresses()
	if err == nil {
		limits.Egresses = len(egresses)
	}
	relays, err := logic.GetRelays()
	if err == nil {
		limits.Relays = len(relays)
	}
	gateways, err := logic.GetInternetGateways()
	if err == nil {
		limits.InternetGateways = len(gateways)
	}
	failovers, err := logic.GetAllFailOvers()
	if err == nil {
		limits.FailOvers = len(failovers)
	}
	return
}
