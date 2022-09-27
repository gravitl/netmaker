package logic

import (
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

// SetFailover - finds a suitable failover candidate and sets it
func SetFailover(node *models.Node) error {
	failoverNode := determineFailoverCandidate(node)
	if failoverNode != nil {
		return setFailoverNode(failoverNode, node)
	}
	return nil
}

// determineFailoverCandidate - returns a list of nodes that
// are suitable for relaying a given node
func determineFailoverCandidate(nodeToBeRelayed *models.Node) *models.Node {

	currentNetworkNodes, err := logic.GetNetworkNodes(nodeToBeRelayed.Network)
	if err != nil {
		return nil
	}

	currentMetrics, err := logic.GetMetrics(nodeToBeRelayed.ID)
	if err != nil || currentMetrics == nil || currentMetrics.Connectivity == nil {
		return nil
	}

	minLatency := int64(9223372036854775807) // max signed int64 value
	var fastestCandidate *models.Node
	for i := range currentNetworkNodes {
		if currentNetworkNodes[i].ID == nodeToBeRelayed.ID {
			continue
		}

		if currentMetrics.Connectivity[currentNetworkNodes[i].ID].Connected && (currentNetworkNodes[i].Failover == "yes") {
			if currentMetrics.Connectivity[currentNetworkNodes[i].ID].Latency < int64(minLatency) {
				fastestCandidate = &currentNetworkNodes[i]
				minLatency = currentMetrics.Connectivity[currentNetworkNodes[i].ID].Latency
			}
		}
	}

	if fastestCandidate == nil {
		leader, err := logic.GetNetworkServerLeader(nodeToBeRelayed.Network)
		if err != nil {
			return nil
		}
		return &leader
	}

	return fastestCandidate
}

// setFailoverNode - changes node's failover node
func setFailoverNode(failoverNode, node *models.Node) error {
	node.FailoverNode = failoverNode.ID
	nodeToUpdate, err := logic.GetNodeByID(node.ID)
	if err != nil {
		return err
	}
	return logic.UpdateNode(&nodeToUpdate, node)
}
