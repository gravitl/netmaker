package logic

import (
	"context"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/nodeacls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// DenyClientNode - add a denied node to an ext client's list
func DenyClientNode(ec *models.ExtClient, clientOrNodeID string) (ok bool) {
	if ec == nil || len(clientOrNodeID) == 0 {
		return
	}
	if ec.DeniedACLs == nil {
		ec.DeniedACLs = map[string]struct{}{}
	}
	ok = true
	ec.DeniedACLs[clientOrNodeID] = struct{}{}
	return
}

// IsClientNodeAllowed - checks if given ext client and node are allowed to communicate
func IsClientNodeAllowed(ec *models.ExtClient, clientOrNodeID string) bool {
	if ec == nil || len(clientOrNodeID) == 0 {
		return false
	}
	if ec.DeniedACLs == nil {
		return true
	}
	_, ok := ec.DeniedACLs[clientOrNodeID]
	return !ok
}

// RemoveDeniedNodeFromClient - removes a node id from set of denied nodes
func RemoveDeniedNodeFromClient(ec *models.ExtClient, clientOrNodeID string) bool {
	if ec.DeniedACLs == nil {
		return true
	}
	_, ok := ec.DeniedACLs[clientOrNodeID]
	if !ok {
		return false
	}
	delete(ec.DeniedACLs, clientOrNodeID)
	return true
}

// SetClientDefaultACLs - set's a client's default ACLs based on network and nodes in network
func SetClientDefaultACLs(ec *models.ExtClient) error {
	_network := &schema.Network{
		ID: ec.Network,
	}
	err := _network.Get(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	_networkNodes, err := _network.GetNodes(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	_networkACL := &schema.NetworkACL{
		ID: ec.Network,
	}
	err = _networkACL.Get(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	_networkACL.Access.Data()[ec.ClientID] = make(map[string]byte)

	for _, _node := range _networkNodes {
		if _network.DefaultACL == "no" || _node.DefaultACL == "no" {
			DenyClientNode(ec, _node.ID)

			_networkACL.Access.Data()[ec.ClientID][_node.ID] = nodeacls.NotAllowed
			_networkACL.Access.Data()[_node.ID][ec.ClientID] = nodeacls.NotAllowed
		} else {
			RemoveDeniedNodeFromClient(ec, _node.ID)

			_networkACL.Access.Data()[ec.ClientID][_node.ID] = nodeacls.Allowed
			_networkACL.Access.Data()[_node.ID][ec.ClientID] = nodeacls.Allowed
		}
	}

	extClients, err := logic.GetNetworkExtClients(ec.Network)
	if err != nil {
		return err
	}

	for _, client := range extClients {
		if _networkACL.Access.Data()[client.ClientID] == nil {
			_networkACL.Access.Data()[client.ClientID] = make(map[string]byte)
		}

		// TODO: revisit when client-client acls are supported
		_networkACL.Access.Data()[ec.ClientID][client.ClientID] = nodeacls.Allowed
		_networkACL.Access.Data()[client.ClientID][ec.ClientID] = nodeacls.Allowed
	}

	// remove access policy to self.
	delete(_networkACL.Access.Data()[ec.ClientID], ec.ClientID)

	return _networkACL.Update(db.WithContext(context.TODO()))
}

// SetClientACLs - overwrites an ext client's ACL
func SetClientACLs(ec *models.ExtClient, newACLs map[string]struct{}) {
	if ec == nil || newACLs == nil {
		return
	}
	ec.DeniedACLs = newACLs
}

func UpdateProNodeACLs(node *models.Node) error {
	_network := &schema.Network{
		ID: node.Network,
	}
	_networkNodes, err := _network.GetNodes(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	nodeID := node.ID.String()

	for _, _node := range _networkNodes {
		if _node.ID == nodeID {
			continue
		}
		// 2 cases
		// both allow - allow
		// either 1 denies - deny
		if node.DoesACLDeny() || _node.DefaultACL == "no" {
			err = nodeacls.ChangeAccess(node.Network, nodeID, _node.ID, nodeacls.NotAllowed)
			if err != nil {
				return err
			}
		} else if node.DoesACLAllow() || _node.DefaultACL == "yes" {
			err = nodeacls.ChangeAccess(node.Network, nodeID, _node.ID, nodeacls.Allowed)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
