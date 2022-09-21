package logic

import (
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

// AutoRelay - finds a suitable relay candidate and creates a relay
func AutoRelay(nodeToBeRelayed *models.Node) (updateNodes []models.Node, err error) {
	newRelayer := determineFailoverCandidate(nodeToBeRelayed)
	if newRelayer != nil {
		return changeRelayStatus(newRelayer, nodeToBeRelayed)
	}
	return
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

		if currentMetrics.Connectivity[currentNetworkNodes[i].ID].Connected && (currentNetworkNodes[i].Failover == "yes" || currentNetworkNodes[i].IsServer == "yes") {
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

// changeRelayStatus - changes nodes to relay
func changeRelayStatus(relayer, nodeToBeRelayed *models.Node) ([]models.Node, error) {
	var newRelayRequest models.RelayRequest

	if relayer.IsRelay == "yes" {
		newRelayRequest.RelayAddrs = relayer.RelayAddrs
	}
	newRelayRequest.NodeID = relayer.ID
	newRelayRequest.NetID = relayer.Network
	newRelayRequest.RelayAddrs = append(newRelayRequest.RelayAddrs, nodeToBeRelayed.PrimaryAddress())

	updatenodes, _, err := logic.CreateRelay(newRelayRequest)
	if err != nil {
		logger.Log(0, "failed to create relay automatically for node", nodeToBeRelayed.Name, "on network", nodeToBeRelayed.Network)
		return nil, err
	}
	logger.Log(0, "created relay automatically for node", nodeToBeRelayed.Name, "on network", nodeToBeRelayed.Network)

	return updatenodes, nil
}
