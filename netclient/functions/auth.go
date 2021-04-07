package functions

import (
    "github.com/gravitl/netmaker/netclient/config"
    "fmt"
//    "os"
    "context"
    "io/ioutil"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
    "google.golang.org/grpc/codes"
    nodepb "github.com/gravitl/netmaker/grpc"

)

// CreateJWT func will used to create the JWT while signing in and signing out
func SetJWT(client nodepb.NodeServiceClient, network string) (context.Context, error) {
		//home, err := os.UserHomeDir()
		home := "/etc/netclient"
		tokentext, err := ioutil.ReadFile(home + "/nettoken-"+network)
                if err != nil {
			fmt.Println("Error reading token. Logging in to retrieve new token.")
			err = AutoLogin(client, network)
			if err != nil {
                                return nil, status.Errorf(codes.Unauthenticated, fmt.Sprintf("Something went wrong with Auto Login: %v", err))
                        }
			tokentext, err = ioutil.ReadFile(home + "/nettoken-"+network)
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

func AutoLogin(client nodepb.NodeServiceClient, network string) error {
	        //home, err := os.UserHomeDir()
		home := "/etc/netclient"
		//nodecfg := config.Config.Node
                cfg, err := config.ReadConfig(network) 
		if err != nil {
			return err
		}
		login := &nodepb.LoginRequest{
                        Password: cfg.Node.Password,
                        Macaddress: cfg.Node.MacAddress,
                        Network: network,
                }
    // RPC call
                res, err := client.Login(context.TODO(), login)
                if err != nil {
                        return err
                }
                tokenstring := []byte(res.Accesstoken)
                err = ioutil.WriteFile(home + "/nettoken-"+network, tokenstring, 0644)
                if err != nil {
                        return err
                }
                return err
}

type Configuration struct {
	MacAddress string
	Password string
}
