package controller

import (
        "context"
	"fmt"
	"strconv"
	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/functions"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NodeServiceServer struct {
	NodeDB *mongo.Collection
	nodepb.UnimplementedNodeServiceServer

}
func (s *NodeServiceServer) ReadNode(ctx context.Context, req *nodepb.ReadNodeReq) (*nodepb.ReadNodeRes, error) {
	// convert string id (from proto) to mongoDB ObjectId
	macaddress := req.GetMacaddress()
        networkName := req.GetNetwork()
	network, _ := functions.GetParentNetwork(networkName)

	node, err := GetNode(macaddress, networkName)

	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("Something went wrong: %v", err))
	}

	/*
	if node == nil {
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("Could not find node with Mac Address %s: %v", req.GetMacaddress(), err))
	}
	*/
	// Cast to ReadNodeRes type
	response := &nodepb.ReadNodeRes{
		Node: &nodepb.Node{
			Macaddress: node.MacAddress,
			Name:    node.Name,
			Address:  node.Address,
			Endpoint:  node.Endpoint,
			Password:  node.Password,
			Nodenetwork:  node.Network,
			Interface:  node.Interface,
			Localaddress:  node.LocalAddress,
			Postdown:  node.PostDown,
			Postup:  node.PostUp,
			Checkininterval:  node.CheckInInterval,
			Ispending:  node.IsPending,
			Publickey:  node.PublicKey,
			Listenport:  node.ListenPort,
			Keepalive:  node.PersistentKeepalive,
                        Islocal:  *network.IsLocal,
                        Localrange:  network.LocalRange,

		},
	}
	return response, nil
}

func (s *NodeServiceServer) CreateNode(ctx context.Context, req *nodepb.CreateNodeReq) (*nodepb.CreateNodeRes, error) {
	// Get the protobuf node type from the protobuf request type
	// Essentially doing req.Node to access the struct with a nil check
	data := req.GetNode()
	// Now we have to convert this into a NodeItem type to convert into BSON
	node := models.Node{
		// ID:       primitive.NilObjectID,
                        MacAddress: data.GetMacaddress(),
                        LocalAddress: data.GetLocaladdress(),
                        Name:    data.GetName(),
                        Address:  data.GetAddress(),
                        AccessKey:  data.GetAccesskey(),
                        Endpoint:  data.GetEndpoint(),
                        PersistentKeepalive:  data.GetKeepalive(),
                        Password:  data.GetPassword(),
                        Interface:  data.GetInterface(),
                        Network:  data.GetNodenetwork(),
                        IsPending:  data.GetIspending(),
                        PublicKey:  data.GetPublickey(),
                        ListenPort:  data.GetListenport(),
	}

        err := ValidateNodeCreate(node.Network, node)

        if err != nil {
                // return internal gRPC error to be handled later
                return nil, err
        }

        //Check to see if key is valid
        //TODO: Triple inefficient!!! This is the third call to the DB we make for networks
        validKey := functions.IsKeyValid(node.Network, node.AccessKey)
        network, err := functions.GetParentNetwork(node.Network)
        if err != nil {
                return nil, status.Errorf(codes.NotFound, fmt.Sprintf("Could not find network: %v", err))
        } else {
		fmt.Println("Creating node in network " + network.NetID)
		fmt.Println("Network is local? " + strconv.FormatBool(*network.IsLocal))
		fmt.Println("Range if local: " + network.LocalRange)
	}



        if !validKey {
                //Check to see if network will allow manual sign up
                //may want to switch this up with the valid key check and avoid a DB call that way.
                if *network.AllowManualSignUp {
                        node.IsPending = true
                } else  {
	                return nil, status.Errorf(
		                codes.Internal,
				fmt.Sprintf("Invalid key, and network does not allow no-key signups"),
			)
                }
        }

	node, err = CreateNode(node, node.Network)

	if err != nil {
		// return internal gRPC error to be handled later
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}
	// return the node in a CreateNodeRes type
	response := &nodepb.CreateNodeRes{
		Node: &nodepb.Node{
                        Macaddress: node.MacAddress,
                        Localaddress: node.LocalAddress,
                        Name:    node.Name,
                        Address:  node.Address,
                        Endpoint:  node.Endpoint,
                        Password:  node.Password,
                        Interface:  node.Interface,
                        Nodenetwork:  node.Network,
                        Ispending:  node.IsPending,
                        Publickey:  node.PublicKey,
                        Listenport:  node.ListenPort,
                        Keepalive:  node.PersistentKeepalive,
                        Islocal:  *network.IsLocal,
                        Localrange:  network.LocalRange,
		},
	}
        err = SetNetworkNodesLastModified(node.Network)
        if err != nil {
                return nil, status.Errorf(codes.NotFound, fmt.Sprintf("Could not update network last modified date: %v", err))
        }

	return response, nil
}

func (s *NodeServiceServer) CheckIn(ctx context.Context, req *nodepb.CheckInReq) (*nodepb.CheckInRes, error) {
	// Get the protobuf node type from the protobuf request type
        // Essentially doing req.Node to access the struct with a nil check
	data := req.GetNode()
	//postchanges := req.GetPostchanges()
	// Now we have to convert this into a NodeItem type to convert into BSON
        node := models.Node{
                // ID:       primitive.NilObjectID,
                        MacAddress: data.GetMacaddress(),
                        Address:  data.GetAddress(),
                        Endpoint:  data.GetEndpoint(),
                        Network:  data.GetNodenetwork(),
                        Password:  data.GetPassword(),
                        LocalAddress:  data.GetLocaladdress(),
                        ListenPort:  data.GetListenport(),
                        PersistentKeepalive:  data.GetKeepalive(),
                        PublicKey:  data.GetPublickey(),
        }

	checkinresponse, err := NodeCheckIn(node, node.Network)

        if err != nil {
                // return internal gRPC error to be handled later
		if checkinresponse == (models.CheckInResponse{}) || !checkinresponse.IsPending {
                return nil, status.Errorf(
                        codes.Internal,
                        fmt.Sprintf("Internal error: %v", err),
                )
		}
        }
        // return the node in a CreateNodeRes type
        response := &nodepb.CheckInRes{
                Checkinresponse: &nodepb.CheckInResponse{
                        Success:  checkinresponse.Success,
                        Needpeerupdate:  checkinresponse.NeedPeerUpdate,
                        Needdelete:  checkinresponse.NeedDelete,
                        Needconfigupdate:  checkinresponse.NeedConfigUpdate,
                        Needkeyupdate:  checkinresponse.NeedKeyUpdate,
                        Nodemessage:  checkinresponse.NodeMessage,
                        Ispending:  checkinresponse.IsPending,
                },
        }
        return response, nil
}


func (s *NodeServiceServer) UpdateNode(ctx context.Context, req *nodepb.UpdateNodeReq) (*nodepb.UpdateNodeRes, error) {
	// Get the node data from the request
        data := req.GetNode()
        // Now we have to convert this into a NodeItem type to convert into BSON
        nodechange := models.Node{
                // ID:       primitive.NilObjectID,
                        MacAddress: data.GetMacaddress(),
                        Name:    data.GetName(),
                        Address:  data.GetAddress(),
                        LocalAddress:  data.GetLocaladdress(),
                        Endpoint:  data.GetEndpoint(),
                        Password:  data.GetPassword(),
                        PersistentKeepalive:  data.GetKeepalive(),
                        Network:  data.GetNodenetwork(),
                        Interface:  data.GetInterface(),
                        PostDown:  data.GetPostdown(),
                        PostUp:  data.GetPostup(),
                        IsPending:  data.GetIspending(),
                        PublicKey:  data.GetPublickey(),
                        ListenPort:  data.GetListenport(),
        }


	// Convert the Id string to a MongoDB ObjectId
	macaddress := nodechange.MacAddress
	networkName := nodechange.Network
        network, _ := functions.GetParentNetwork(networkName)


	err := ValidateNodeUpdate(networkName, nodechange)
        if err != nil {
                return nil, err
        }

        node, err := functions.GetNodeByMacAddress(networkName, macaddress)
        if err != nil {
               return nil, status.Errorf(
                        codes.NotFound,
                        fmt.Sprintf("Could not find node with supplied Mac Address: %v", err),
                )
	}


	newnode, err := UpdateNode(nodechange, node)

	if err != nil {
		return nil, status.Errorf(
			codes.NotFound,
			fmt.Sprintf("Could not find node with supplied Mac Address: %v", err),
		)
	}
	return &nodepb.UpdateNodeRes{
		Node: &nodepb.Node{
                        Macaddress: newnode.MacAddress,
                        Localaddress: newnode.LocalAddress,
                        Name:    newnode.Name,
                        Address:  newnode.Address,
                        Endpoint:  newnode.Endpoint,
                        Password:  newnode.Password,
                        Interface:  newnode.Interface,
                        Postdown:  newnode.PostDown,
                        Postup:  newnode.PostUp,
                        Nodenetwork:  newnode.Network,
                        Ispending:  newnode.IsPending,
                        Publickey:  newnode.PublicKey,
                        Listenport:  newnode.ListenPort,
                        Keepalive:  newnode.PersistentKeepalive,
                        Islocal:  *network.IsLocal,
                        Localrange:  network.LocalRange,

		},
	}, nil
}

func (s *NodeServiceServer) DeleteNode(ctx context.Context, req *nodepb.DeleteNodeReq) (*nodepb.DeleteNodeRes, error) {
	fmt.Println("beginning node delete")
	macaddress := req.GetMacaddress()
	network := req.GetNetworkName()

	success, err := DeleteNode(macaddress, network)

	if err != nil || !success {
		fmt.Println("Error deleting node.")
		fmt.Println(err)
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("Could not find/delete node with mac address %s", macaddress))
	}

	fmt.Println("updating network last modified of " + req.GetNetworkName())
	err = SetNetworkNodesLastModified(req.GetNetworkName())
        if err != nil {
		fmt.Println("Error updating Network")
		fmt.Println(err)
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("Could not update network last modified date: %v", err))
        }


	return &nodepb.DeleteNodeRes{
		Success: true,
	}, nil
}

func (s *NodeServiceServer) GetPeers(req *nodepb.GetPeersReq, stream nodepb.NodeService_GetPeersServer) error {
	// Initiate a NodeItem type to write decoded data to
	//data := &models.PeersResponse{}
	// collection.Find returns a cursor for our (empty) query
	//cursor, err := s.NodeDB.Find(context.Background(), bson.M{})
	peers, err := GetPeersList(req.GetNetwork())

	if err != nil {
		return status.Errorf(codes.Internal, fmt.Sprintf("Unknown internal error: %v", err))
	}
	// cursor.Next() returns a boolean, if false there are no more items and loop will break
        for i := 0; i < len(peers); i++ {

		// If no error is found send node over stream
		stream.Send(&nodepb.GetPeersRes{
			Peers: &nodepb.PeersResponse{
                            Address:  peers[i].Address,
                            Endpoint:  peers[i].Endpoint,
                            Gatewayrange:  peers[i].GatewayRange,
                            Isgateway:  peers[i].IsGateway,
                            Publickey:  peers[i].PublicKey,
                            Keepalive:  peers[i].KeepAlive,
                            Listenport:  peers[i].ListenPort,
                            Localaddress:  peers[i].LocalAddress,
			},
		})
	}

	node, err := functions.GetNodeByMacAddress(req.GetNetwork(), req.GetMacaddress())
       if err != nil {
                return status.Errorf(codes.Internal, fmt.Sprintf("Could not get node: %v", err))
        }


	err = TimestampNode(node, false, true, false)
        if err != nil {
                return status.Errorf(codes.Internal, fmt.Sprintf("Internal error occurred: %v", err))
        }


	return nil
}
