package logic

import (
	"context"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
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

	var networkAcls acls.ACLContainer
	networkAcls, err = networkAcls.Get(acls.ContainerID(ec.Network))
	if err != nil {
		slog.Error("failed to get network acls", "error", err)
		return err
	}

	networkAcls[acls.AclID(ec.ClientID)] = make(acls.ACL)

	for _, _node := range _networkNodes {
		if _network.DefaultACL == "no" || _node.DefaultACL == "no" {
			DenyClientNode(ec, _node.ID)
			networkAcls[acls.AclID(ec.ClientID)][acls.AclID(_node.ID)] = acls.NotAllowed
			networkAcls[acls.AclID(_node.ID)][acls.AclID(ec.ClientID)] = acls.NotAllowed
		} else {
			RemoveDeniedNodeFromClient(ec, _node.ID)
			networkAcls[acls.AclID(ec.ClientID)][acls.AclID(_node.ID)] = acls.Allowed
			networkAcls[acls.AclID(_node.ID)][acls.AclID(ec.ClientID)] = acls.Allowed
		}
	}

	extClients, err := logic.GetNetworkExtClients(ec.Network)
	if err != nil {
		return err
	}

	for _, client := range extClients {
		// TODO: revisit when client-client acls are supported
		networkAcls[acls.AclID(ec.ClientID)][acls.AclID(client.ClientID)] = acls.Allowed
		networkAcls[acls.AclID(client.ClientID)][acls.AclID(ec.ClientID)] = acls.Allowed
	}

	delete(networkAcls[acls.AclID(ec.ClientID)], acls.AclID(ec.ClientID)) // remove oneself
	if _, err = networkAcls.Save(acls.ContainerID(ec.Network)); err != nil {
		slog.Error("failed to update network acls", "error", err)
		return err
	}
	return nil
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

	currentACLs, err := nodeacls.FetchAllACLs(nodeacls.NetworkID(_network.ID))
	if err != nil {
		return err
	}

	for _, _node := range _networkNodes {
		if _node.ID == nodeID {
			continue
		}
		// 2 cases
		// both allow - allow
		// either 1 denies - deny
		if node.DoesACLDeny() || _node.DefaultACL == "no" {
			currentACLs.ChangeAccess(acls.AclID(nodeID), acls.AclID(_node.ID), acls.NotAllowed)
		} else if node.DoesACLAllow() || _node.DefaultACL == "yes" {
			currentACLs.ChangeAccess(acls.AclID(nodeID), acls.AclID(_node.ID), acls.Allowed)
		}
	}

	_, err = currentACLs.Save(acls.ContainerID(node.Network))
	return nil
}
