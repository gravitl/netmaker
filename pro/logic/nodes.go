package logic

import (
	"errors"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func ValidateInetGwReq(req models.InetNodeReq) error {
	for _, clientNodeID := range req.InetNodeClientIDs {
		clientNode, err := logic.GetNodeByID(clientNodeID)
		if err != nil {
			continue
		}
		if clientNode.IsInternetGateway {
			return errors.New("node acting as internet gateway cannot use another internet gateway")
		}
	}
	return nil
}

// SetInternetGw - sets the node as internet gw based on flag bool
func SetInternetGw(node *models.Node, req models.InetNodeReq) {
	node.IsInternetGateway = true
	node.InetNodeReq = req
	for _, clientNodeID := range req.InetNodeClientIDs {
		clientNode, err := logic.GetNodeByID(clientNodeID)
		if err != nil {
			continue
		}
		clientNode.InternetGwID = node.ID.String()
		logic.UpsertNode(&clientNode)
	}

}

func UnsetInternetGw(node *models.Node) {
	for _, nodeID := range node.InetNodeReq.InetNodeClientIDs {
		clientNode, err := logic.GetNodeByID(nodeID)
		if err != nil {
			continue
		}
		if node.ID.String() == clientNode.InternetGwID {
			node.InternetGwID = ""
			logic.UpsertNode(node)
		}

	}
	node.IsInternetGateway = false
	node.InetNodeReq = models.InetNodeReq{}

}

// GetNetworkIngresses - gets the gateways of a network
func GetNetworkIngresses(network string) ([]models.Node, error) {
	var ingresses []models.Node
	netNodes, err := logic.GetNetworkNodes(network)
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
