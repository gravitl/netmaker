package controller

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// NodeServiceServer - represents the service server for gRPC
type NodeServiceServer struct {
	nodepb.UnimplementedNodeServiceServer
}

// NodeServiceServer.ReadNode - reads node and responds with gRPC
func (s *NodeServiceServer) ReadNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	var node, err = getNewOrLegacyNode(req.Data)
	if err != nil {
		return nil, err
	}

	node.NetworkSettings, err = logic.GetNetworkSettings(node.Network)
	if err != nil {
		return nil, err
	}
	node.SetLastCheckIn()
	// Cast to ReadNodeRes type
	nodeData, errN := json.Marshal(&node)
	if errN != nil {
		return nil, err
	}
	logic.UpdateNode(&node, &node)
	response := &nodepb.Object{
		Data: string(nodeData),
		Type: nodepb.NODE_TYPE,
	}
	return response, nil
}

// NodeServiceServer.CreateNode - creates a node and responds over gRPC
func (s *NodeServiceServer) CreateNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	var node = models.Node{}
	var err error
	data := req.GetData()
	if err := json.Unmarshal([]byte(data), &node); err != nil {
		return nil, err
	}

	validKey := logic.IsKeyValid(node.Network, node.AccessKey)
	node.NetworkSettings, err = logic.GetNetworkSettings(node.Network)
	if err != nil {
		return nil, err
	}

	if !validKey {
		if node.NetworkSettings.AllowManualSignUp == "yes" {
			node.IsPending = "yes"
		} else {
			return nil, errors.New("invalid key, and network does not allow no-key signups")
		}
	}

	getServerAddrs(&node)

	key, keyErr := logic.RetrievePublicTrafficKey()
	if keyErr != nil {
		logger.Log(0, "error retrieving key: ", keyErr.Error())
		return nil, keyErr
	}

	node.TrafficKeys = models.TrafficKeys{
		Mine:   node.TrafficKeys.Mine,
		Server: key,
	}

	err = logic.CreateNode(&node)
	if err != nil {
		return nil, err
	}

	nodeData, errN := json.Marshal(&node)
	if errN != nil {
		return nil, err
	}

	response := &nodepb.Object{
		Data: string(nodeData),
		Type: nodepb.NODE_TYPE,
	}

	runUpdates(&node, false)

	return response, nil
}

// NodeServiceServer.UpdateNode updates a node and responds over gRPC
func (s *NodeServiceServer) UpdateNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {

	var newnode models.Node
	if err := json.Unmarshal([]byte(req.GetData()), &newnode); err != nil {
		return nil, err
	}

	node, err := logic.GetNodeByIDorMacAddress(newnode.ID, newnode.MacAddress, newnode.Network)
	if err != nil {
		return nil, err
	}

	if !servercfg.GetRce() {
		newnode.PostDown = node.PostDown
		newnode.PostUp = node.PostUp
	}

	getServerAddrs(&node)

	err = logic.UpdateNode(&node, &newnode)
	if err != nil {
		return nil, err
	}
	newnode.NetworkSettings, err = logic.GetNetworkSettings(node.Network)
	if err != nil {
		return nil, err
	}
	nodeData, errN := json.Marshal(&newnode)
	if errN != nil {
		return nil, err
	}

	runUpdates(&node, false)

	return &nodepb.Object{
		Data: string(nodeData),
		Type: nodepb.NODE_TYPE,
	}, nil
}

func getServerAddrs(node *models.Node) {
	var serverNodes = logic.GetServerNodes(node.Network)
	var serverAddrs = make([]models.ServerAddr, len(serverNodes))
	for i, server := range serverNodes {
		serverAddrs[i] = models.ServerAddr{
			IsLeader: logic.IsLeader(&server),
			Address:  server.Address,
		}
	}
	// TODO consolidate functionality around files
	node.NetworkSettings.NodesLastModified = time.Now().Unix()
	node.NetworkSettings.DefaultServerAddrs = serverAddrs
	if err := logic.SaveNetwork(&node.NetworkSettings); err != nil {
		logger.Log(1, "unable to save network on serverAddr update", err.Error())
	}
}

// NodeServiceServer.DeleteNode - deletes a node and responds over gRPC
func (s *NodeServiceServer) DeleteNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {

	var node, err = getNewOrLegacyNode(req.Data)
	if err != nil {
		return nil, err
	}

	err = logic.DeleteNodeByID(&node, true)
	if err != nil {
		return nil, err
	}
	runServerPeerUpdate(&node, false)

	return &nodepb.Object{
		Data: "success",
		Type: nodepb.STRING_TYPE,
	}, nil
}

// NodeServiceServer.GetPeers - fetches peers over gRPC
func (s *NodeServiceServer) GetPeers(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {

	var node, err = getNewOrLegacyNode(req.Data)
	if err != nil {
		return nil, err
	}

	excludeIsRelayed := node.IsRelay != "yes"
	var relayedNode string
	if node.IsRelayed == "yes" {
		relayedNode = node.Address
	}
	peers, err := logic.GetPeersList(node.Network, excludeIsRelayed, relayedNode)
	if err != nil {
		return nil, err
	}

	peersData, err := json.Marshal(&peers)
	logger.Log(3, node.Address, "checked in successfully")
	return &nodepb.Object{
		Data: string(peersData),
		Type: nodepb.NODE_TYPE,
	}, err
}

// NodeServiceServer.GetExtPeers - returns ext peers for a gateway node
func (s *NodeServiceServer) GetExtPeers(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {

	var node, err = getNewOrLegacyNode(req.Data)
	if err != nil {
		return nil, err
	}

	peers, err := logic.GetExtPeersList(&node)
	if err != nil {
		return nil, err
	}
	var extPeers []models.Node
	for i := 0; i < len(peers); i++ {
		extPeers = append(extPeers, models.Node{
			Address:             peers[i].Address,
			Address6:            peers[i].Address6,
			Endpoint:            peers[i].Endpoint,
			PublicKey:           peers[i].PublicKey,
			PersistentKeepalive: peers[i].KeepAlive,
			ListenPort:          peers[i].ListenPort,
			LocalAddress:        peers[i].LocalAddress,
		})
	}

	extData, err := json.Marshal(&extPeers)
	if err != nil {
		return nil, err
	}

	return &nodepb.Object{
		Data: string(extData),
		Type: nodepb.EXT_PEER,
	}, nil
}

// == private methods ==

func getNewOrLegacyNode(data string) (models.Node, error) {
	var reqNode, node models.Node
	var err error

	if err = json.Unmarshal([]byte(data), &reqNode); err != nil {
		oldID := strings.Split(data, "###") // handle legacy client IDs
		if len(oldID) == 2 {
			if node, err = logic.GetNodeByIDorMacAddress(reqNode.ID, oldID[0], oldID[1]); err != nil {
				return models.Node{}, err
			}
		} else {
			return models.Node{}, err
		}
	} else {
		node, err = logic.GetNodeByIDorMacAddress(reqNode.ID, reqNode.MacAddress, reqNode.Network)
		if err != nil {
			return models.Node{}, err
		}
	}
	return node, nil
}

func isServer(node *models.Node) bool {
	return node.IsServer == "yes"
}
