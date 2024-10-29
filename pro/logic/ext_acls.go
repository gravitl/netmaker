package logic

import (
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
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
	networkNodes, err := logic.GetNetworkNodes(ec.Network)
	if err != nil {
		return err
	}
	network, err := logic.GetNetwork(ec.Network)
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
	for i := range networkNodes {
		currNode := networkNodes[i]
		if network.DefaultACL == "no" || currNode.DefaultACL == "no" {
			DenyClientNode(ec, currNode.ID.String())
			networkAcls[acls.AclID(ec.ClientID)][acls.AclID(currNode.ID.String())] = acls.NotAllowed
			networkAcls[acls.AclID(currNode.ID.String())][acls.AclID(ec.ClientID)] = acls.NotAllowed
		} else {
			RemoveDeniedNodeFromClient(ec, currNode.ID.String())
			networkAcls[acls.AclID(ec.ClientID)][acls.AclID(currNode.ID.String())] = acls.Allowed
			networkAcls[acls.AclID(currNode.ID.String())][acls.AclID(ec.ClientID)] = acls.Allowed
		}
	}
	networkClients, err := logic.GetNetworkExtClients(ec.Network)
	if err != nil {
		slog.Error("failed to get network clients", "error", err)
		return err
	}
	for _, client := range networkClients {
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
	networkNodes, err := logic.GetNetworkNodes(node.Network)
	if err != nil {
		return err
	}
	if err = adjustNodeAcls(node, networkNodes[:]); err != nil {
		return err
	}
	return nil
}

// adjustNodeAcls - adjusts ACLs based on a node's default value
func adjustNodeAcls(node *models.Node, networkNodes []models.Node) error {
	networkID := nodeacls.NetworkID(node.Network)
	nodeID := nodeacls.NodeID(node.ID.String())
	currentACLs, err := nodeacls.FetchAllACLs(networkID)
	if err != nil {
		return err
	}

	for i := range networkNodes {
		currentNodeID := nodeacls.NodeID(networkNodes[i].ID.String())
		if currentNodeID == nodeID {
			continue
		}
		// 2 cases
		// both allow - allow
		// either 1 denies - deny
		if node.DoesACLDeny() || networkNodes[i].DoesACLDeny() {
			currentACLs.ChangeAccess(acls.AclID(nodeID), acls.AclID(currentNodeID), acls.NotAllowed)
		} else if node.DoesACLAllow() || networkNodes[i].DoesACLAllow() {
			currentACLs.ChangeAccess(acls.AclID(nodeID), acls.AclID(currentNodeID), acls.Allowed)
		}
	}

	_, err = currentACLs.Save(acls.ContainerID(node.Network))
	return err
}
