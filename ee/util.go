package ee

import (
	"encoding/base64"

	"github.com/gravitl/netmaker/logic"
)

var isEnterprise bool

// setIsEnterprise - sets server to use enterprise features
func setIsEnterprise() {
	isEnterprise = true
	logic.SetEEForTelemetry(isEnterprise)
}

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
	hosts, hErr := logic.GetAllHosts()
	if hErr == nil {
		limits.Hosts = len(hosts)
	}
	clients, cErr := logic.GetAllExtClients()
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
	ingresses, err := logic.GetAllIngresses()
	if err == nil {
		limits.Ingresses = len(ingresses)
	}
	egresses, err := logic.GetAllEgresses()
	if err == nil {
		limits.Egresses = len(egresses)
	}
	return
}
