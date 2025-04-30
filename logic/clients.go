package logic

import (
	"context"
	"errors"
	"fmt"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/nodeacls"
	"github.com/gravitl/netmaker/schema"
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
		// allow all on CE
		_networkACL := &schema.NetworkACL{
			ID: ec.Network,
		}
		err := _networkACL.Get(db.WithContext(context.TODO()))
		if err != nil {
			logger.Log(0, fmt.Sprintf("failed to get network (%s) acls: %s", _networkACL.ID, err.Error()))
			return err
		}

		_networkACL.Access.Data()[ec.ClientID] = make(map[string]byte)

		for peerID := range _networkACL.Access.Data() {
			_networkACL.Access.Data()[peerID][ec.ClientID] = nodeacls.Allowed
			_networkACL.Access.Data()[ec.ClientID][peerID] = nodeacls.Allowed
		}

		// delete self loop.
		delete(_networkACL.Access.Data()[ec.ClientID], ec.ClientID)

		err = _networkACL.Update(db.WithContext(context.TODO()))
		if err != nil {
			logger.Log(0, fmt.Sprintf("failed to update network (%s) acls: %s", _networkACL.ID, err.Error()))
			return err
		}

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
