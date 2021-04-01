package controller

import (
	"errors"
	"context"
        "golang.org/x/crypto/bcrypt"
        "time"
        nodepb "github.com/gravitl/netmaker/grpc"
        "github.com/gravitl/netmaker/models"
        "github.com/gravitl/netmaker/functions"
        "github.com/gravitl/netmaker/mongoconn"
        "go.mongodb.org/mongo-driver/bson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"

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

		mac, group, err := functions.VerifyToken(authToken)

		if err  != nil { return err }

                groupexists, err := functions.GroupExists(group)

		if err != nil {
			return status.Errorf(codes.Unauthenticated, "Unauthorized. Group does not exist: " + group)

		}
		emptynode := models.Node{}
		node, err := functions.GetNodeByMacAddress(group, mac)
		if err != nil || node == emptynode {
                        return status.Errorf(codes.Unauthenticated, "Node does not exist.")
		}

                //check that the request is for a valid group
                //if (groupCheck && !groupexists) || err != nil {
                if (!groupexists) {

			return status.Errorf(codes.Unauthenticated, "Group does not exist.")

                } else {
                        return nil
                }
}


//Node authenticates using its password and retrieves a JWT for authorization.
func (s *NodeServiceServer) Login(ctx context.Context, req *nodepb.LoginRequest) (*nodepb.LoginResponse, error) {

	//out := new(LoginResponse)
	macaddress := req.GetMacaddress()
	password := req.GetPassword()

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
            collection := mongoconn.Client.Database("netmaker").Collection("nodes")
            ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
            var err = collection.FindOne(ctx, bson.M{ "macaddress": macaddress}).Decode(&result)

            defer cancel()

            if err != nil {
                return nil, err
            }

           //compare password from request to stored password in database
           //might be able to have a common hash (certificates?) and compare those so that a password isn't passed in in plain text...
           //TODO: Consider a way of hashing the password client side before sending, or using certificates
           err = bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(password))
           if err != nil && result.Password != password {
			return nil, err
           } else {
                //Create a new JWT for the node
                tokenString, _ := functions.CreateJWT(macaddress, result.Group)

                if tokenString == "" {
		    err = errors.New("Something went wrong. Could not retrieve token.")
                    return nil, err
                }

		response := &nodepb.LoginResponse{
				Accesstoken: tokenString,
			}
		return response, nil
        }
    }
}
