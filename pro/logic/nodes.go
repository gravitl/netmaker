package logic

import (
	"context"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// GetNetworkIngresses - gets the gateways of a network
func GetNetworkIngresses(ctx context.Context, network string) ([]models.Node, error) {
	var ingresses []models.Node
	netNodes, err := logic.GetNetworkNodes(ctx, network)
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

func GetTagMapWithNodes(ctx context.Context) (tagNodesMap map[models.TagID][]models.Node) {
	tagNodesMap = make(map[models.TagID][]models.Node)
	nodes, _ := logic.GetAllNodes(ctx)
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

func AddTagMapWithStaticNodesWithUsers(ctx context.Context, netID schema.NetworkID,
	tagNodesMap map[models.TagID][]models.Node) map[models.TagID][]models.Node {
	extclients, err := logic.GetNetworkExtClients(ctx, netID.String())
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
			tagNodesMap[tagID] = append(tagNodesMap[tagID], models.ConvertToStaticNode(extclient))
		}
		if extclient.Mutex != nil {
			extclient.Mutex.Unlock()
		}

	}
	return tagNodesMap
}

func GetNodeIDsWithTag(ctx context.Context, tagID models.TagID) (ids []string) {

	tag, err := GetTag(ctx, tagID)
	if err != nil {
		return
	}
	nodes, _ := logic.GetNetworkNodes(ctx, tag.Network.String())
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

func GetNodesWithTag(ctx context.Context, tagID models.TagID) map[string]models.Node {
	nMap := make(map[string]models.Node)
	tag, err := GetTag(ctx, tagID)
	if err != nil {
		return nMap
	}
	nodes, _ := logic.GetNetworkNodes(ctx, tag.Network.String())
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
	return AddStaticNodesWithTag(ctx, tag, nMap)
}

func AddStaticNodesWithTag(ctx context.Context, tag models.Tag, nMap map[string]models.Node) map[string]models.Node {
	extclients, err := logic.GetNetworkExtClients(ctx, tag.Network.String())
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
			nMap[extclient.ClientID] = models.ConvertToStaticNode(extclient)
		}
		if extclient.Mutex != nil {
			extclient.Mutex.Unlock()
		}
	}
	return nMap
}

func GetStaticNodeWithTag(ctx context.Context, tagID models.TagID) map[string]models.Node {
	nMap := make(map[string]models.Node)
	tag, err := GetTag(ctx, tagID)
	if err != nil {
		return nMap
	}
	extclients, err := logic.GetNetworkExtClients(ctx, tag.Network.String())
	if err != nil {
		return nMap
	}
	for _, extclient := range extclients {
		nMap[extclient.ClientID] = models.ConvertToStaticNode(extclient)
	}
	return nMap
}
