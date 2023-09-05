package logic

import (
	"errors"
	"sort"

	"github.com/gravitl/netmaker/models"
)

// functions defined here, handle client ACLs, should be set on ee

var (
	// DenyClientNodeAccess - function to handle adding a node to an ext client's denied node set
	DenyClientNodeAccess = func(ec *models.ExtClient, clientOrNodeID string) bool {
		return true
	}
	// IsClientNodeAllowed - function to check if an ext client's denied node set contains a node ID
	IsClientNodeAllowed = func(ec *models.ExtClient, clientOrNodeID string) bool {
		return true
	}
	// AllowClientNodeAccess - function to handle removing a node ID from ext client's denied nodes, thus allowing it
	AllowClientNodeAccess = func(ec *models.ExtClient, clientOrNodeID string) bool {
		return true
	}
	SetClientDefaultACLs = func(ec *models.ExtClient) error {
		return nil
	}
	SetClientACLs = func(ec *models.ExtClient, newACLs map[string]struct{}) {
	}
	UpdateProNodeACLs = func(node *models.Node) error {
		return nil
	}
)

// SortExtClient - Sorts slice of ExtClients by their ClientID alphabetically with numbers first
func SortExtClient(unsortedExtClient []models.ExtClient) {
	sort.Slice(unsortedExtClient, func(i, j int) bool {
		return unsortedExtClient[i].ClientID < unsortedExtClient[j].ClientID
	})
}

// GetExtClientByName - gets an ext client by name
func GetExtClientByName(ID string) (models.ExtClient, error) {
	clients, err := GetAllExtClients()
	if err != nil {
		return models.ExtClient{}, err
	}
	for i := range clients {
		if clients[i].ClientID == ID {
			return clients[i], nil
		}
	}
	return models.ExtClient{}, errors.New("client not found")
}
