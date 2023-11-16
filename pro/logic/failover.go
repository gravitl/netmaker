package logic

import (
	"errors"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func SetFailOverCtx(failOverNode, victimNode, peerNode models.Node) error {
	peerNode.FailOverPeers[victimNode.ID.String()] = struct{}{}
	victimNode.FailedOverBy = failOverNode.ID
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
		if node.FailOver {
			return node, nil
		}
	}
	return models.Node{}, errors.New("auto relay not found")
}
