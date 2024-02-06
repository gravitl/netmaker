package logic

import (
	"errors"
	"net"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/exp/slog"
)

func SetFailOverCtx(failOverNode, victimNode, peerNode models.Node) error {
	if peerNode.FailOverPeers == nil {
		peerNode.FailOverPeers = make(map[string]struct{})
	}
	if victimNode.FailOverPeers == nil {
		victimNode.FailOverPeers = make(map[string]struct{})
	}
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
func FailOverExists(network string) (failOverNode models.Node, exists bool) {
	nodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		return
	}
	for _, node := range nodes {
		if node.IsFailOver {
			exists = true
			failOverNode = node
			return
		}
	}
	return
}

// ResetFailedOverPeer - removes failed over node from network peers
func ResetFailedOverPeer(failedOveredNode *models.Node) error {
	nodes, err := logic.GetNetworkNodes(failedOveredNode.Network)
	if err != nil {
		return err
	}
	failedOveredNode.FailedOverBy = uuid.Nil
	failedOveredNode.FailOverPeers = make(map[string]struct{})
	err = logic.UpsertNode(failedOveredNode)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.FailOverPeers == nil || node.ID == failedOveredNode.ID {
			continue
		}
		delete(node.FailOverPeers, failedOveredNode.ID.String())
		logic.UpsertNode(&node)
	}
	return nil
}

// ResetFailOver - reset failovered peers
func ResetFailOver(failOverNode *models.Node) error {
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

// GetFailOverPeerIps - adds the failedOvered peerIps by the peer
func GetFailOverPeerIps(peer, node *models.Node) []net.IPNet {
	allowedips := []net.IPNet{}
	for failOverpeerID := range node.FailOverPeers {
		failOverpeer, err := logic.GetNodeByID(failOverpeerID)
		if err == nil && failOverpeer.FailedOverBy == peer.ID {
			if failOverpeer.Address.IP != nil {
				allowed := net.IPNet{
					IP:   failOverpeer.Address.IP,
					Mask: net.CIDRMask(32, 32),
				}
				allowedips = append(allowedips, allowed)
			}
			if failOverpeer.Address6.IP != nil {
				allowed := net.IPNet{
					IP:   failOverpeer.Address6.IP,
					Mask: net.CIDRMask(128, 128),
				}
				allowedips = append(allowedips, allowed)
			}

		}
	}
	return allowedips
}

func CreateFailOver(node models.Node) error {
	if _, exists := FailOverExists(node.Network); exists {
		return errors.New("failover already exists in the network")
	}
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		return err
	}
	if host.OS != models.OS_Types.Linux {
		return err
	}
	if node.IsRelayed {
		return err
	}
	node.IsFailOver = true
	err = logic.UpsertNode(&node)
	if err != nil {
		slog.Error("failed to upsert node", "node", node.ID.String(), "error", err)
		return err
	}
	return nil
}
