package auth

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"

	//    "os"
	"context"

	nodepb "github.com/gravitl/netmaker/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// SetJWT func will used to create the JWT while signing in and signing out
func SetJWT(client nodepb.NodeServiceClient, network string) (context.Context, error) {
	home := ncutils.GetNetclientPathSpecific()
	tokentext, err := ncutils.GetFileWithRetry(home+"nettoken-"+network, 1)
	if err != nil {
		err = AutoLogin(client, network)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, fmt.Sprintf("Something went wrong with Auto Login: %v", err))
		}
		tokentext, err = ncutils.GetFileWithRetry(home+"nettoken-"+network, 1)
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

// AutoLogin - auto logins whenever client needs to request from server
func AutoLogin(client nodepb.NodeServiceClient, network string) error {
	home := ncutils.GetNetclientPathSpecific()
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	pass, err := RetrieveSecret(network)
	if err != nil {
		return err
	}
	node := models.Node{
		Password:   pass,
		MacAddress: cfg.Node.MacAddress,
		ID:         cfg.Node.ID,
		Network:    network,
	}
	data, err := json.Marshal(&node)
	if err != nil {
		return nil
	}

	login := &nodepb.Object{
		Data: string(data),
		Type: nodepb.NODE_TYPE,
	}
	// RPC call
	res, err := client.Login(context.TODO(), login)
	if err != nil {
		return err
	}
	tokenstring := []byte(res.Data)
	err = os.WriteFile(home+"nettoken-"+network, tokenstring, 0600)
	if err != nil {
		return err
	}
	return err
}

// StoreSecret - stores auth secret locally
func StoreSecret(key string, network string) error {
	d1 := []byte(key)
	return os.WriteFile(ncutils.GetNetclientPathSpecific()+"secret-"+network, d1, 0600)
}

// RetrieveSecret - fetches secret locally
func RetrieveSecret(network string) (string, error) {
	dat, err := ncutils.GetFileWithRetry(ncutils.GetNetclientPathSpecific()+"secret-"+network, 3)
	return string(dat), err
}

// StoreTrafficKey - stores traffic key
func StoreTrafficKey(key *[32]byte, network string) error {
	var data, err = ncutils.ConvertKeyToBytes(key)
	if err != nil {
		return err
	}
	return os.WriteFile(ncutils.GetNetclientPathSpecific()+"traffic-"+network, data, 0600)
}

// RetrieveTrafficKey - reads traffic file locally
func RetrieveTrafficKey(network string) (*[32]byte, error) {
	data, err := ncutils.GetFileWithRetry(ncutils.GetNetclientPathSpecific()+"traffic-"+network, 2)
	if err != nil {
		return nil, err
	}
	return ncutils.ConvertBytesToKey(data)
}

// Configuraion - struct for mac and pass
type Configuration struct {
	MacAddress string
	Password   string
}
