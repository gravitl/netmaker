package controller

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func AuthServerUnaryInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {
	// Skip authorize when GetJWT is requested

	if info.FullMethod != "/node.NodeService/Login" {
		if info.FullMethod != "/node.NodeService/CreateNode" {

			err := grpcAuthorize(ctx)
			if err != nil {
				return nil, err
			}
		}
	}

	// Calls the handler
	h, err := handler(ctx, req)

	return h, err
}
func AuthServerStreamInterceptor(
	srv interface{},
	stream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	if info.FullMethod == "/node.NodeService/GetPeers" {
		if err := grpcAuthorize(stream.Context()); err != nil {
			return err
		}
	}

	// Calls the handler
	return handler(srv, stream)
}

func grpcAuthorize(ctx context.Context) error {

	md, ok := metadata.FromIncomingContext(ctx)

	if !ok {
		return status.Errorf(codes.InvalidArgument, "Retrieving metadata is failed")
	}

	authHeader, ok := md["authorization"]
	if !ok {
		return status.Errorf(codes.Unauthenticated, "Authorization token is not supplied")
	}

	authToken := authHeader[0]

	mac, network, err := functions.VerifyToken(authToken)

	if err != nil {
		return err
	}

	networkexists, err := functions.NetworkExists(network)

	if err != nil {
		return status.Errorf(codes.Unauthenticated, "Unauthorized. Network does not exist: "+network)

	}
	emptynode := models.Node{}
	node, err := functions.GetNodeByMacAddress(network, mac)
	if !database.IsEmptyRecord(err) {
		if node, err = functions.GetDeletedNodeByMacAddress(network, mac); err != nil {
			if !database.IsEmptyRecord(err) {
				return status.Errorf(codes.Unauthenticated, "Node does not exist.")
			}
		} else {
			node.SetID()
			if functions.RemoveDeletedNode(node.ID) {
				return nil
			}
			return status.Errorf(codes.Unauthenticated, "Node does not exist.")
		}
	} else if err != nil || node.MacAddress == emptynode.MacAddress {
		return status.Errorf(codes.Unauthenticated, "Node does not exist.")
	}

	//check that the request is for a valid network
	//if (networkCheck && !networkexists) || err != nil {
	if !networkexists {

		return status.Errorf(codes.Unauthenticated, "Network does not exist.")

	} else {
		return nil
	}
}

//Node authenticates using its password and retrieves a JWT for authorization.
func (s *NodeServiceServer) Login(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {

	//out := new(LoginResponse)
	var reqNode models.Node
	if err := json.Unmarshal([]byte(req.Data), &reqNode); err != nil {
		return nil, err
	}

	macaddress := reqNode.MacAddress
	network := reqNode.Network
	password := reqNode.Password

	var result models.NodeAuth

	err := errors.New("Generic server error.")

	if macaddress == "" {
		//TODO: Set Error  response
		err = errors.New("Missing Mac Address.")
		return nil, err
	} else if password == "" {
		err = errors.New("Missing Password.")
		return nil, err
	} else {
		//Search DB for node with Mac Address. Ignore pending nodes (they should not be able to authenticate with API untill approved).
		collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
		if err != nil {
			return nil, err
		}
		for _, value := range collection {
			if err = json.Unmarshal([]byte(value), &result); err != nil {
				continue // finish going through nodes
			}
			if result.MacAddress == macaddress && result.Network == network {
				break
			}
		}

		//compare password from request to stored password in database
		//might be able to have a common hash (certificates?) and compare those so that a password isn't passed in in plain text...
		//TODO: Consider a way of hashing the password client side before sending, or using certificates
		err = bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(password))
		if err != nil && result.Password != password {
			return nil, err
		} else {
			//Create a new JWT for the node
			tokenString, err := functions.CreateJWT(macaddress, result.Network)

			if err != nil {
				return nil, err
			}
			if tokenString == "" {
				err = errors.New("Something went wrong. Could not retrieve token.")
				return nil, err
			}

			response := &nodepb.Object{
				Data: tokenString,
				Type: nodepb.ACCESS_TOKEN,
			}
			return response, nil
		}
	}
}
