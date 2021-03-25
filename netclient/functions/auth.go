package functions

import (
    "github.com/gravitl/netmaker/netclient/config"
    "fmt"
    "os"
    "context"
    "io/ioutil"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
    "google.golang.org/grpc/codes"
    nodepb "github.com/gravitl/netmaker/grpc"

)

// CreateJWT func will used to create the JWT while signing in and signing out
func SetJWT(client nodepb.NodeServiceClient) (context.Context, error) {
		home, err := os.UserHomeDir()
                tokentext, err := ioutil.ReadFile(home + "/.wctoken")
                if err != nil {
			fmt.Println("Error reading token. Logging in to retrieve new token.")
			err = AutoLogin(client)
			if err != nil {
                                return nil, status.Errorf(codes.Unauthenticated, fmt.Sprintf("Something went wrong with Auto Login: %v", err))
                        }
			tokentext, err = ioutil.ReadFile(home + "/.wctoken")
			if err != nil {
				return nil, status.Errorf(codes.Unauthenticated, fmt.Sprintf("Something went wrong: %v", err))
			}
		}
                token := string(tokentext)

                // Anything linked to this variable will transmit request headers.
                md := metadata.New(map[string]string{"authorization": token})
                ctx := context.Background()
                ctx = metadata.NewOutgoingContext(ctx, md)
		return ctx, nil
}

func AutoLogin(client nodepb.NodeServiceClient) error {
	        home, err := os.UserHomeDir()
		nodecfg := config.Config.Node
                login := &nodepb.LoginRequest{
                        Password: nodecfg.Password,
                        Macaddress: nodecfg.MacAddress,
                }
    // RPC call
                res, err := client.Login(context.TODO(), login)
                if err != nil {
                        return err
                }
                fmt.Printf("Token: %s\n", res.Accesstoken)
                tokenstring := []byte(res.Accesstoken)
                err = ioutil.WriteFile(home + "/.wctoken", tokenstring, 0644)
                if err != nil {
                        return err
                }
                return err
}

type Configuration struct {
	MacAddress string
	Password string
}
