package controller

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"

	"github.com/gravitl/netmaker/functions"
	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
)

type NodeServiceServer struct {
	nodepb.UnimplementedNodeServiceServer
}

func (s *NodeServiceServer) ReadNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	// convert string id (from proto) to mongoDB ObjectId
	var node models.Node
	if err := json.Unmarshal([]byte(req.Data), &node); err != nil {
		return nil, err
	}
	macaddress := node.MacAddress
	networkName := node.Network

	node, err := GetNode(macaddress, networkName)

	if err != nil {
		log.Println("could not get node "+macaddress+" "+networkName, err)
		return nil, err
	}
	// Cast to ReadNodeRes type
	nodeData, err := json.Marshal(&node)
	if err != nil {
		return nil, err
	}
	response := &nodepb.Object{
		Data: string(nodeData),
		Type: nodepb.NODE_TYPE,
	}
	return response, nil
}

func (s *NodeServiceServer) CreateNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	// Get the protobuf node type from the protobuf request type
	// Essentially doing req.Node to access the struct with a nil check
	var node models.Node
	data := req.GetData()
	if err := json.Unmarshal([]byte(data), &node); err != nil {
		return nil, err
	}

	//Check to see if key is valid
	//TODO: Triple inefficient!!! This is the third call to the DB we make for networks
	validKey := functions.IsKeyValid(node.Network, node.AccessKey)
	network, err := functions.GetParentNetwork(node.Network)
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

	node, err = CreateNode(node, node.Network)
	if err != nil {
		log.Println("could not create node on network " + node.Network + " (gRPC controller)")
		return nil, err
	}
	nodeData, err := json.Marshal(&node)
	// return the node in a CreateNodeRes type
	response := &nodepb.Object{
		Data: string(nodeData),
		Type: nodepb.NODE_TYPE,
	}
	err = SetNetworkNodesLastModified(node.Network)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (s *NodeServiceServer) UpdateNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	// Get the node data from the request
	var newnode models.Node
	if err := json.Unmarshal([]byte(req.GetData()), &newnode); err != nil {
		return nil, err
	}
	macaddress := newnode.MacAddress
	networkName := newnode.Network

	node, err := functions.GetNodeByMacAddress(networkName, macaddress)
	if err != nil {
		return nil, err
	}

	err = node.Update(&newnode)
	if err != nil {
		return nil, err
	}
	nodeData, err := json.Marshal(&node)

	return &nodepb.Object{
		Data: string(nodeData),
		Type: nodepb.NODE_TYPE,
	}, nil
}

func (s *NodeServiceServer) DeleteNode(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	nodeID := req.GetData()

	err := DeleteNode(nodeID, true)
	if err != nil {
		log.Println("Error deleting node (gRPC controller).")
		return nil, err
	}

	return &nodepb.Object{
		Data: "success",
		Type: nodepb.STRING_TYPE,
	}, nil
}

func (s *NodeServiceServer) GetPeers(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	macAndNetwork := strings.Split(req.Data, "###")
	if len(macAndNetwork) == 2 {
		peers, err := GetPeersList(macAndNetwork[1])
		if err != nil {
			return nil, err
		}

		peersData, err := json.Marshal(&peers)
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

/**
 * Return Ext Peers (clients).NodeCheckIn
 * When a gateway node checks in, it pulls these peers to add to peers list in addition to normal network peers.
 */
func (s *NodeServiceServer) GetExtPeers(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {
	// Initiate a NodeItem type to write decoded data to
	//data := &models.PeersResponse{}
	// collection.Find returns a cursor for our (empty) query
	var reqNode models.Node
	if err := json.Unmarshal([]byte(req.Data), &reqNode); err != nil {
		return nil, err
	}
	peers, err := GetExtPeersList(reqNode.Network, reqNode.MacAddress)
	if err != nil {
		return nil, err
	}
	// cursor.Next() returns a boolean, if false there are no more items and loop will break
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
