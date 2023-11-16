package logic

import (
	"errors"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func SetFailOverCtx(failOverNode, victimNode, peerNode models.Node) error {
	peerNode.FailOverPeers[victimNode.ID.String()] = struct{}{}
	victimNode.FailOverPeers[peerNode.ID.String()] = struct{}{}
	victimNode.FailedOverBy = failOverNode.ID
	peerNode.FailedOverBy = failOverNode.ID
	if err := logic.UpsertNode(&failOverNode); err != nil {
		return err
	}
	if err := logic.UpsertNode(&victimNode); err != nil {
		return err
	}
	if err := logic.UpsertNode(&peerNode); err != nil {
		return err
	}
	return nil
}

// GetFailOverNode - gets the host acting as failOver
func GetFailOverNode(network string, allNodes []models.Node) (models.Node, error) {
	nodes := logic.GetNetworkNodesMemory(allNodes, network)
	for _, node := range nodes {
		if node.IsFailOver {
			return node, nil
		}
	}
	return models.Node{}, errors.New("auto relay not found")
}

// FailOverExists - checks if failOver exists already in the network
func FailOverExists(network string) (exists bool) {
	nodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		return
	}
	for _, node := range nodes {
		if node.IsFailOver {
			exists = true
			return
		}
	}
	return
}

// ResetFailOveredPeers - reset failovered peers
func ResetFailOveredPeers(failOverNode *models.Node) error {
	// Unset FailedOverPeers
	nodes, err := logic.GetNetworkNodes(failOverNode.Network)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.FailedOverBy == failOverNode.ID {
			node.FailedOverBy = uuid.Nil
			node.FailOverPeers = make(map[string]struct{})
			logic.UpsertNode(&node)
		}
	}
	return nil
}
