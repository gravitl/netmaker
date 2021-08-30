package auth

import (
	"encoding/json"
	"fmt"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/netclientutils"

	//    "os"
	"context"
	"io/ioutil"

	nodepb "github.com/gravitl/netmaker/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// CreateJWT func will used to create the JWT while signing in and signing out
func SetJWT(client nodepb.NodeServiceClient, network string) (context.Context, error) {
	home := netclientutils.GetNetclientPathSpecific()
	tokentext, err := ioutil.ReadFile(home + "nettoken-" + network)
	if err != nil {
		err = AutoLogin(client, network)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, fmt.Sprintf("Something went wrong with Auto Login: %v", err))
		}
		tokentext, err = ioutil.ReadFile(home + "nettoken-" + network)
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
	home := netclientutils.GetNetclientPathSpecific()
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
		Network:    network,
	}
	data, err := json.Marshal(&node)
	if err != nil {
		return nil
	}

	login := &nodepb.Object{
		Data: string(data),
	}
	// RPC call
	res, err := client.Login(context.TODO(), login)
	if err != nil {
		return err
	}
	tokenstring := []byte(res.Data)
	err = ioutil.WriteFile(home+"nettoken-"+network, tokenstring, 0644)
	if err != nil {
		return err
	}
	return err
}

func StoreSecret(key string, network string) error {
	d1 := []byte(key)
	err := ioutil.WriteFile(netclientutils.GetNetclientPathSpecific()+"secret-"+network, d1, 0644)
	return err
}

func RetrieveSecret(network string) (string, error) {
	dat, err := ioutil.ReadFile(netclientutils.GetNetclientPathSpecific() + "secret-" + network)
	return string(dat), err
}

type Configuration struct {
	MacAddress string
	Password   string
}
