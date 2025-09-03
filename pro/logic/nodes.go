package logic

import (
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

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

func GetTagMapWithNodes() (tagNodesMap map[models.TagID][]models.Node) {
	tagNodesMap = make(map[models.TagID][]models.Node)
	nodes, _ := logic.GetAllNodes()
	for _, nodeI := range nodes {
		if nodeI.Tags == nil {
			continue
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Lock()
		}
		for nodeTagID := range nodeI.Tags {
			tagNodesMap[nodeTagID] = append(tagNodesMap[nodeTagID], nodeI)
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Unlock()
		}

	}
	return
}

func AddTagMapWithStaticNodesWithUsers(netID models.NetworkID,
	tagNodesMap map[models.TagID][]models.Node) map[models.TagID][]models.Node {
	extclients, err := logic.GetNetworkExtClients(netID.String())
	if err != nil {
		return tagNodesMap
	}
	for _, extclient := range extclients {
		tagNodesMap[models.TagID(extclient.ClientID)] = []models.Node{
			{
				IsStatic:   true,
				StaticNode: extclient,
			},
		}
		if extclient.Tags == nil {
			continue
		}
		if extclient.Mutex != nil {
			extclient.Mutex.Lock()
		}
		for tagID := range extclient.Tags {
			tagNodesMap[tagID] = append(tagNodesMap[tagID], extclient.ConvertToStaticNode())
		}
		if extclient.Mutex != nil {
			extclient.Mutex.Unlock()
		}

	}
	return tagNodesMap
}

func GetNodeIDsWithTag(tagID models.TagID) (ids []string) {

	tag, err := GetTag(tagID)
	if err != nil {
		return
	}
	nodes, _ := logic.GetNetworkNodes(tag.Network.String())
	for _, nodeI := range nodes {
		if nodeI.Tags == nil {
			continue
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Lock()
		}
		if _, ok := nodeI.Tags[tagID]; ok {
			ids = append(ids, nodeI.ID.String())
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Unlock()
		}
	}
	return
}

func GetNodesWithTag(tagID models.TagID) map[string]models.Node {
	nMap := make(map[string]models.Node)
	tag, err := GetTag(tagID)
	if err != nil {
		return nMap
	}
	nodes, _ := logic.GetNetworkNodes(tag.Network.String())
	for _, nodeI := range nodes {
		if nodeI.Tags == nil {
			continue
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Lock()
		}
		if _, ok := nodeI.Tags[tagID]; ok {
			nMap[nodeI.ID.String()] = nodeI
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Unlock()
		}
	}
	return AddStaticNodesWithTag(tag, nMap)
}

func AddStaticNodesWithTag(tag models.Tag, nMap map[string]models.Node) map[string]models.Node {
	extclients, err := logic.GetNetworkExtClients(tag.Network.String())
	if err != nil {
		return nMap
	}
	for _, extclient := range extclients {
		if extclient.RemoteAccessClientID != "" {
			continue
		}
		if extclient.Mutex != nil {
			extclient.Mutex.Lock()
		}
		if _, ok := extclient.Tags[tag.ID]; ok {
			nMap[extclient.ClientID] = extclient.ConvertToStaticNode()
		}
		if extclient.Mutex != nil {
			extclient.Mutex.Unlock()
		}
	}
	return nMap
}

func GetStaticNodeWithTag(tagID models.TagID) map[string]models.Node {
	nMap := make(map[string]models.Node)
	tag, err := GetTag(tagID)
	if err != nil {
		return nMap
	}
	extclients, err := logic.GetNetworkExtClients(tag.Network.String())
	if err != nil {
		return nMap
	}
	for _, extclient := range extclients {
		nMap[extclient.ClientID] = extclient.ConvertToStaticNode()
	}
	return nMap
}
