package controller

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

// NodeServiceServer - represents the service server for gRPC
type NodeServiceServer struct {
	nodepb.UnimplementedNodeServiceServer
}

// NodeServiceServer.ReadNode - reads node and responds with gRPC
func (s *NodeServiceServer) ReadNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	// convert string id (from proto) to mongoDB ObjectId
	macAndNetwork := strings.Split(req.Data, "###")

	if len(macAndNetwork) != 2 {
		return nil, errors.New("could not read node, invalid node id given")
	}
	node, err := GetNode(macAndNetwork[0], macAndNetwork[1])
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
	// Get the protobuf node type from the protobuf request type
	// Essentially doing req.Node to access the struct with a nil check
	var node = &models.Node{}
	data := req.GetData()
	if err := json.Unmarshal([]byte(data), node); err != nil {
		return nil, err
	}

	//Check to see if key is valid
	//TODO: Triple inefficient!!! This is the third call to the DB we make for networks
	validKey := logic.IsKeyValid(node.Network, node.AccessKey)
	network, err := logic.GetParentNetwork(node.Network)
	if err != nil {
		return nil, err
	}

	if !validKey {
		//Check to see if network will allow manual sign up
		//may want to switch this up with the valid key check and avoid a DB call that way.
		if network.AllowManualSignUp == "yes" {
			node.IsPending = "yes"
		} else {
			return nil, errors.New("invalid key, and network does not allow no-key signups")
		}
	}

	node, err = logic.CreateNode(node, node.Network)
	if err != nil {
		return nil, err
	}
	node.NetworkSettings, err = logic.GetNetworkSettings(node.Network)
	if err != nil {
		return nil, err
	}
	nodeData, errN := json.Marshal(&node)
	if errN != nil {
		return nil, err
	}
	// return the node in a CreateNodeRes type
	response := &nodepb.Object{
		Data: string(nodeData),
		Type: nodepb.NODE_TYPE,
	}
	err = logic.SetNetworkNodesLastModified(node.Network)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// NodeServiceServer.UpdateNode updates a node and responds over gRPC
func (s *NodeServiceServer) UpdateNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	// Get the node data from the request
	var newnode models.Node
	if err := json.Unmarshal([]byte(req.GetData()), &newnode); err != nil {
		return nil, err
	}
	macaddress := newnode.MacAddress
	networkName := newnode.Network

	node, err := logic.GetNodeByMacAddress(networkName, macaddress)
	if err != nil {
		return nil, err
	}
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
	return &nodepb.Object{
		Data: string(nodeData),
		Type: nodepb.NODE_TYPE,
	}, nil
}

// NodeServiceServer.DeleteNode - deletes a node and responds over gRPC
func (s *NodeServiceServer) DeleteNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	nodeID := req.GetData()

	err := DeleteNode(nodeID, true)
	if err != nil {
		return nil, err
	}

	return &nodepb.Object{
		Data: "success",
		Type: nodepb.STRING_TYPE,
	}, nil
}

// NodeServiceServer.GetPeers - fetches peers over gRPC
func (s *NodeServiceServer) GetPeers(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	macAndNetwork := strings.Split(req.Data, "###")
	if len(macAndNetwork) == 2 {
		// TODO: Make constant and new variable for isServer
		node, err := GetNode(macAndNetwork[0], macAndNetwork[1])
		if err != nil {
			return nil, err
		}
		if node.IsServer == "yes" && logic.IsLeader(&node) {
			logic.SetNetworkServerPeers(&node)
		}
		excludeIsRelayed := node.IsRelay != "yes"
		var relayedNode string
		if node.IsRelayed == "yes" {
			relayedNode = node.Address
		}
		peers, err := logic.GetPeersList(macAndNetwork[1], excludeIsRelayed, relayedNode)
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
	return &nodepb.Object{
		Data: "",
		Type: nodepb.NODE_TYPE,
	}, errors.New("could not fetch peers, invalid node id")
}

// NodeServiceServer.GetExtPeers - returns ext peers for a gateway node
func (s *NodeServiceServer) GetExtPeers(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	// Initiate a NodeItem type to write decoded data to
	//data := &models.PeersResponse{}
	// collection.Find returns a cursor for our (empty) query
	macAndNetwork := strings.Split(req.Data, "###")
	if len(macAndNetwork) != 2 {
		return nil, errors.New("did not receive valid node id when fetching ext peers")
	}
	peers, err := logic.GetExtPeersList(macAndNetwork[0], macAndNetwork[1])
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
