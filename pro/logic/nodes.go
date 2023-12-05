package logic

import (
	celogic "github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

// IsInternetGw - checks if node is acting as internet gw
func IsInternetGw(node models.Node) bool {
	return node.IsInternetGateway
}

// SetInternetGw - sets the node as internet gw based on flag bool
func SetInternetGw(node *models.Node, flag bool) {
	node.IsInternetGateway = flag
}

// GetNetworkIngresses - gets the gateways of a network
func GetNetworkIngresses(network string) ([]models.Node, error) {
	var ingresses []models.Node
	netNodes, err := celogic.GetNetworkNodes(network)
	if err != nil {
		return []models.Node{}, err
	}
	for i := range netNodes {
		if netNodes[i].IsIngressGateway {
			ingresses = append(ingresses, netNodes[i])
		}
	}
	return ingresses, nil
}
