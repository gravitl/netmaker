package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
)

// NodeServiceServer - represents the service server for gRPC
type NodeServiceServer struct {
	nodepb.UnimplementedNodeServiceServer
}

// NodeServiceServer.ReadNode - reads node and responds with gRPC
func (s *NodeServiceServer) ReadNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	var node, err = getNodeFromRequestData(req.Data)
	if err != nil {
		return nil, err
	}

	node.NetworkSettings, err = logic.GetNetworkSettings(node.Network)
	if err != nil {
		return nil, err
	}
	getServerAddrs(&node)

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

	if key == nil {
		logger.Log(0, "error: server traffic key is nil")
		return nil, fmt.Errorf("error: server traffic key is nil")
	}
	if node.TrafficKeys.Mine == nil {
		logger.Log(0, "error: node traffic key is nil")
		return nil, fmt.Errorf("error: node traffic key is nil")
	}

	node.TrafficKeys = models.TrafficKeys{
		Mine:   node.TrafficKeys.Mine,
		Server: key,
	}

	commID, err := logic.FetchCommsNetID()
	if err != nil {
		return nil, err
	}
	node.CommID = commID

	_, err = logic.CreateNode(&node)
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

	runForceServerUpdate(&node)

	go func(node *models.Node) {
		if node.UDPHolePunch == "yes" {
			var currentServerNode, getErr = logic.GetNetworkServerLeader(node.Network)
			if getErr != nil {
				return
			}
			for i := 0; i < 5; i++ {
				if logic.HasPeerConnected(node) {
					if logic.ShouldPublishPeerPorts(&currentServerNode) {
						err = mq.PublishPeerUpdate(&currentServerNode)
						if err != nil {
							logger.Log(1, "error publishing port updates when node", node.Name, "joined")
						}
						break
					}
				}
				time.Sleep(time.Second << 1) // allow time for client to startup
			}
		}
	}(&node)

	return response, nil
}

// NodeServiceServer.UpdateNode updates a node and responds over gRPC
// DELETE ONE DAY - DEPRECATED
func (s *NodeServiceServer) UpdateNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {

	var newnode models.Node
	if err := json.Unmarshal([]byte(req.GetData()), &newnode); err != nil {
		return nil, err
	}

	node, err := logic.GetNodeByID(newnode.ID)
	if err != nil {
		return nil, err
	}

	if !servercfg.GetRce() {
		newnode.PostDown = node.PostDown
		newnode.PostUp = node.PostUp
	}

	err = logic.UpdateNode(&node, &newnode)
	if err != nil {
		return nil, err
	}
	newnode.NetworkSettings, err = logic.GetNetworkSettings(node.Network)
	if err != nil {
		return nil, err
	}
	getServerAddrs(&newnode)

	nodeData, errN := json.Marshal(&newnode)
	if errN != nil {
		return nil, err
	}

	return &nodepb.Object{
		Data: string(nodeData),
		Type: nodepb.NODE_TYPE,
	}, nil
}

func getServerAddrs(node *models.Node) {
	serverNodes := logic.GetServerNodes(serverctl.COMMS_NETID)
	//pubIP, _ := servercfg.GetPublicIP()
	if len(serverNodes) == 0 {
		if err := serverctl.SyncServerNetwork(serverctl.COMMS_NETID); err != nil {
			return
		}
	}

	var serverAddrs = make([]models.ServerAddr, 0)

	for _, node := range serverNodes {
		if node.Address != "" {
			serverAddrs = append(serverAddrs, models.ServerAddr{
				IsLeader: logic.IsLeader(&node),
				Address:  node.Address,
			})
		}
	}

	networkSettings, _ := logic.GetParentNetwork(node.Network)
	// TODO consolidate functionality around files
	networkSettings.NodesLastModified = time.Now().Unix()
	networkSettings.DefaultServerAddrs = serverAddrs
	if err := logic.SaveNetwork(&networkSettings); err != nil {
		logger.Log(1, "unable to save network on serverAddr update", err.Error())
	}
	node.NetworkSettings.DefaultServerAddrs = networkSettings.DefaultServerAddrs
}

// NodeServiceServer.DeleteNode - deletes a node and responds over gRPC
func (s *NodeServiceServer) DeleteNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {

	var node, err = getNodeFromRequestData(req.Data)
	if err != nil {
		return nil, err
	}

	err = logic.DeleteNodeByID(&node, true)
	if err != nil {
		return nil, err
	}

	runForceServerUpdate(&node)

	return &nodepb.Object{
		Data: "success",
		Type: nodepb.STRING_TYPE,
	}, nil
}

// NodeServiceServer.GetPeers - fetches peers over gRPC
func (s *NodeServiceServer) GetPeers(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {

	var node, err = getNodeFromRequestData(req.Data)
	if err != nil {
		return nil, err
	}

	peers, err := logic.GetPeersList(&node)
	if err != nil {
		if strings.Contains(err.Error(), logic.RELAY_NODE_ERR) {
			peers, err = logic.PeerListUnRelay(node.ID, node.Network)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
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

	var node, err = getNodeFromRequestData(req.Data)
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

func getNodeFromRequestData(data string) (models.Node, error) {
	var reqNode models.Node
	var err error

	if err = json.Unmarshal([]byte(data), &reqNode); err != nil {
		return models.Node{}, err
	}
	return logic.GetNodeByID(reqNode.ID)
}

func isServer(node *models.Node) bool {
	return node.IsServer == "yes"
}

func runForceServerUpdate(node *models.Node) {
	go func() {
		if err := mq.PublishPeerUpdate(node); err != nil {
			logger.Log(1, "failed a peer update after creation of node", node.Name)
		}

		var currentServerNode, getErr = logic.GetNetworkServerLeader(node.Network)
		if getErr == nil {
			if err := logic.ServerUpdate(&currentServerNode, false); err != nil {
				logger.Log(1, "server node:", currentServerNode.ID, "failed update")
			}
		}
	}()
}
