package controller

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthServerUnaryInterceptor - auth unary interceptor logic
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

// AuthServerStreamInterceptor - auth stream interceptor
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

	nodeID, mac, network, err := logic.VerifyToken(authToken)
	if err != nil {
		return err
	}

	networkexists, err := functions.NetworkExists(network)

	if err != nil {
		return status.Errorf(codes.Unauthenticated, "Unauthorized. Network does not exist: "+network)
	}
	emptynode := models.Node{}
	node, err := logic.GetNodeByIDorMacAddress(nodeID, mac, network)
	if database.IsEmptyRecord(err) {
		// == DELETE replace logic after 2 major version updates ==
		if node, err = logic.GetDeletedNodeByID(node.ID); err == nil {
			if functions.RemoveDeletedNode(node.ID) {
				return status.Errorf(codes.Unauthenticated, models.NODE_DELETE)
			}
			return status.Errorf(codes.Unauthenticated, "Node does not exist.")
		}
		return status.Errorf(codes.Unauthenticated, "Empty record")
	}
	if err != nil || node.MacAddress == emptynode.MacAddress {
		return status.Errorf(codes.Unauthenticated, "Node does not exist.")
	}

	if !networkexists {
		return status.Errorf(codes.Unauthenticated, "Network does not exist.")
	}
	return nil
}

// Login - node authenticates using its password and retrieves a JWT for authorization.
func (s *NodeServiceServer) Login(ctx context.Context, req *nodepb.Object) (*nodepb.Object, error) {

	//out := new(LoginResponse)
	var reqNode models.Node
	if err := json.Unmarshal([]byte(req.Data), &reqNode); err != nil {
		return nil, err
	}

	nodeID := reqNode.ID
	network := reqNode.Network
	password := reqNode.Password
	macaddress := reqNode.MacAddress

	var result models.NodeAuth
	var err error
	// err := errors.New("generic server error")

	if nodeID == "" {
		//TODO: Set Error  response
		err = errors.New("missing node ID")
		return nil, err
	} else if password == "" {
		err = errors.New("missing password")
		return nil, err
	} else {
		//Search DB for node with Mac Address. Ignore pending nodes (they should not be able to authenticate with API until approved).
		collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
		if err != nil {
			return nil, err
		}
		for _, value := range collection {
			if err = json.Unmarshal([]byte(value), &result); err != nil {
				continue // finish going through nodes
			}
			if result.ID == nodeID && result.Network == network {
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
			tokenString, err := logic.CreateJWT(result.ID, macaddress, result.Network)

			if err != nil {
				return nil, err
			}
			if tokenString == "" {
				err = errors.New("something went wrong, could not retrieve token")
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
