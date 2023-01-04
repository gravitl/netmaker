package logic

import (
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/logger"
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

// ResetFailover - sets the failover node and wipes disconnected status
func ResetFailover(network string) error {
	nodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		err = SetFailover(&node)
		if err != nil {
			logger.Log(2, "error setting failover for node", node.ID.String(), ":", err.Error())
		}
		err = WipeFailover(node.ID.String())
		if err != nil {
			logger.Log(2, "error wiping failover for node", node.ID.String(), ":", err.Error())
		}
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

	currentMetrics, err := logic.GetMetrics(nodeToBeRelayed.ID.String())
	if err != nil || currentMetrics == nil || currentMetrics.Connectivity == nil {
		return nil
	}

	minLatency := int64(9223372036854775807) // max signed int64 value
	var fastestCandidate *models.Node
	for i := range currentNetworkNodes {
		if currentNetworkNodes[i].ID == nodeToBeRelayed.ID {
			continue
		}

		if currentMetrics.Connectivity[currentNetworkNodes[i].ID.String()].Connected && (currentNetworkNodes[i].Failover) {
			if currentMetrics.Connectivity[currentNetworkNodes[i].ID.String()].Latency < int64(minLatency) {
				fastestCandidate = &currentNetworkNodes[i]
				minLatency = currentMetrics.Connectivity[currentNetworkNodes[i].ID.String()].Latency
			}
		}
	}

	return fastestCandidate
}

// setFailoverNode - changes node's failover node
func setFailoverNode(failoverNode, node *models.Node) error {

	node.FailoverNode = failoverNode.ID
	nodeToUpdate, err := logic.GetNodeByID(node.ID.String())
	if err != nil {
		return err
	}
	if nodeToUpdate.FailoverNode == failoverNode.ID {
		return nil
	}
	return logic.UpdateNode(&nodeToUpdate, node)
}

// WipeFailover - removes the failover peers of given node (ID)
func WipeFailover(nodeid string) error {
	metrics, err := logic.GetMetrics(nodeid)
	if err != nil {
		return err
	}
	if metrics != nil {
		metrics.FailoverPeers = make(map[string]string)
		return logic.UpdateMetrics(nodeid, metrics)
	}
	return nil
}

// WipeAffectedFailoversOnly - wipes failovers for nodes that have given node (ID)
// in their respective failover lists
func WipeAffectedFailoversOnly(nodeid uuid.UUID, network string) error {
	currentNetworkNodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		return nil
	}
	WipeFailover(nodeid.String())

	for i := range currentNetworkNodes {
		currNodeID := currentNetworkNodes[i].ID
		if currNodeID == nodeid {
			continue
		}
		currMetrics, err := logic.GetMetrics(currNodeID.String())
		if err != nil || currMetrics == nil {
			continue
		}
		if currMetrics.FailoverPeers != nil {
			if len(currMetrics.FailoverPeers[nodeid.String()]) > 0 {
				WipeFailover(currNodeID.String())
			}
		}
	}
	return nil
}
