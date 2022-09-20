package proacls

import (
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
)

// AdjustNodeAcls - adjusts ACLs based on a node's default value
func AdjustNodeAcls(node *models.Node, networkNodes []models.Node) error {
	networkID := nodeacls.NetworkID(node.Network)
	nodeID := nodeacls.NodeID(node.ID)
	currentACLs, err := nodeacls.FetchAllACLs(networkID)
	if err != nil {
		return err
	}

	for i := range networkNodes {
		currentNodeID := nodeacls.NodeID(networkNodes[i].ID)
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
