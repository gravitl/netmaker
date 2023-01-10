package logic

import (
	"github.com/gravitl/netmaker/models"
)

// IfaceDelta - checks if the new node causes an interface change
func IfaceDelta(currentNode *models.Node, newNode *models.Node) bool {
	// single comparison statements
	if newNode.Address.String() != currentNode.Address.String() ||
		newNode.Address6.String() != currentNode.Address6.String() ||
		newNode.IsEgressGateway != currentNode.IsEgressGateway ||
		newNode.IsIngressGateway != currentNode.IsIngressGateway ||
		newNode.PersistentKeepalive != currentNode.PersistentKeepalive ||
		newNode.DNSOn != currentNode.DNSOn ||
		newNode.Connected != currentNode.Connected {
		return true
	}
	// multi-comparison statements
	if newNode.IsEgressGateway {
		if len(currentNode.EgressGatewayRanges) != len(newNode.EgressGatewayRanges) {
			return true
		}
		for _, address := range newNode.EgressGatewayRanges {
			if !StringSliceContains(currentNode.EgressGatewayRanges, address) {
				return true
			}
		}
	}
	return false
}

// == Private Functions ==
