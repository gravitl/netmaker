package logic

import (
	"sort"

	"github.com/gravitl/netmaker/models"
)

// functions defined here, handle client ACLs, should be set on ee

var (
	// DenyClientNodeAccess - function to handle adding a node to an ext client's denied node set
	DenyClientNodeAccess = func(ec *models.ExtClient, clientOrNodeID string) bool {
		ec.DeniedACLs[clientOrNodeID] = struct{}{}
		return true
	}
	// IsClientNodeAllowed - function to check if an ext client's denied node set contains a node ID
	IsClientNodeAllowed = func(ec *models.ExtClient, clientOrNodeID string) bool {
		_, isNodeDenied := ec.DeniedACLs[clientOrNodeID]
		return !isNodeDenied
	}
	// AllowClientNodeAccess - function to handle removing a node ID from ext client's denied nodes, thus allowing it
	AllowClientNodeAccess = func(ec *models.ExtClient, clientOrNodeID string) bool {
		delete(ec.DeniedACLs, clientOrNodeID)
		return true
	}
)

// SetClientDefaultACLs - set's a client's default ACLs based on network and nodes in network
func SetClientDefaultACLs(ec *models.ExtClient) error {
	if !isEE {
		return nil
	}
	networkNodes, err := GetNetworkNodes(ec.Network)
	if err != nil {
		return err
	}
	network, err := GetNetwork(ec.Network)
	if err != nil {
		return err
	}
	for i := range networkNodes {
		currNode := networkNodes[i]
		if network.DefaultACL == "no" || currNode.DefaultACL == "no" {
			DenyClientNodeAccess(ec, currNode.ID.String())
		} else {
			AllowClientNodeAccess(ec, currNode.ID.String())
		}
	}
	return nil
}

// SetClientACLs - overwrites an ext client's ACL
func SetClientACLs(ec *models.ExtClient, newACLs map[string]struct{}) {
	if ec == nil || newACLs == nil || !isEE {
		return
	}
	ec.DeniedACLs = newACLs
}

// IsClientNodeAllowedByID - checks if a given ext client ID + nodeID are allowed
func IsClientNodeAllowedByID(clientID, networkName, clientOrNodeID string) bool {
	client, err := GetExtClient(clientID, networkName)
	if err != nil {
		return false
	}
	return IsClientNodeAllowed(&client, clientOrNodeID)
}

// SortExtClient - Sorts slice of ExtClients by their ClientID alphabetically with numbers first
func SortExtClient(unsortedExtClient []models.ExtClient) {
	sort.Slice(unsortedExtClient, func(i, j int) bool {
		return unsortedExtClient[i].ClientID < unsortedExtClient[j].ClientID
	})
}
