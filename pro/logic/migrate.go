package logic

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func MigrateToGws() {
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return
	}
	for _, node := range nodes {
		if node.IsIngressGateway || node.IsRelay || node.IsInternetGateway || node.IsFailOver {
			node.IsGw = true
			node.IsIngressGateway = true
			node.IsRelay = true
			node.IsAutoRelay = true
			if node.Tags == nil {
				node.Tags = make(map[models.TagID]struct{})
			}
			node.Tags[models.TagID(fmt.Sprintf("%s.%s", node.Network, models.GwTagName))] = struct{}{}
			delete(node.Tags, models.TagID(fmt.Sprintf("%s.%s", node.Network, models.OldRemoteAccessTagName)))
			logic.UpsertNode(&node)
		}
		// deprecate failover  and initialise auto relay fields
		if node.IsFailOver {
			node.IsFailOver = false
			node.FailOverPeers = make(map[string]struct{})
			node.FailedOverBy = uuid.Nil
			node.AutoRelayedPeers = make(map[string]string)
			logic.UpsertNode(&node)
		}
		if node.FailedOverBy != uuid.Nil || len(node.FailOverPeers) > 0 {
			node.FailOverPeers = make(map[string]struct{})
			node.FailedOverBy = uuid.Nil
			node.AutoRelayedPeers = make(map[string]string)
			logic.UpsertNode(&node)
		}
		if node.IsInternetGateway && len(node.InetNodeReq.InetNodeClientIDs) > 0 {
			node.RelayedNodes = append(node.RelayedNodes, node.InetNodeReq.InetNodeClientIDs...)
			node.RelayedNodes = logic.UniqueStrings(node.RelayedNodes)
			for _, nodeID := range node.InetNodeReq.InetNodeClientIDs {
				relayedNode, err := logic.GetNodeByID(nodeID)
				if err == nil {
					relayedNode.IsRelayed = true
					relayedNode.RelayedBy = node.ID.String()
					logic.UpsertNode(&relayedNode)
				}
			}
			logic.UpsertNode(&node)
		}
	}
	acls := logic.ListAcls()
	for _, acl := range acls {
		upsert := false
		for i, srcI := range acl.Src {
			if srcI.ID == models.NodeTagID && srcI.Value == fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.OldRemoteAccessTagName) {
				srcI.Value = fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.GwTagName)
				acl.Src[i] = srcI
				upsert = true
			}
		}
		for i, dstI := range acl.Dst {
			if dstI.ID == models.NodeTagID && dstI.Value == fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.OldRemoteAccessTagName) {
				dstI.Value = fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.GwTagName)
				acl.Dst[i] = dstI
				upsert = true
			}
		}
		if upsert {
			logic.UpsertAcl(acl)
		}
	}
	nets, _ := logic.GetNetworks()
	for _, netI := range nets {
		DeleteTag(models.TagID(fmt.Sprintf("%s.%s", netI.Name, models.OldRemoteAccessTagName)), true)
	}
}
