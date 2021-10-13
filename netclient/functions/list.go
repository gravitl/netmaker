package functions

import (
	"encoding/json"
	"fmt"
	"log"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Peer struct {
	Name           string `json:"name"`
	Interface      string `json:"interface,omitempty"`
	PrivateIPv4    string `json:"private_ipv4,omitempty"`
	PrivateIPv6    string `json:"private_ipv6,omitempty"`
	PublicEndpoint string `json:"public_endoint,omitempty"`
}

type Network struct {
	Name        string `json:"name"`
	CurrentNode Peer   `json:"current_node"`
	Peers       []Peer `json:"peers"`
}

func List() error {
	networks, err := ncutils.GetSystemNetworks()
	if err != nil {
		return err
	}

	nets := []Network{}
	for _, network := range networks {
		cfg, err := config.ReadConfig(network)
		if err != nil {
			ncutils.PrintLog(network+": Could not retrieve network configuration.", 1)
			return err
		}
		peers, err := getPeers(network)
		if err != nil {
			ncutils.PrintLog(network+": Could not retrieve network configuration.", 1)
			return err
		}

		nets = append(nets, Network{
			Name:  network,
			Peers: peers,
			CurrentNode: Peer{
				Name:           cfg.Node.Name,
				Interface:      cfg.Node.Interface,
				PrivateIPv4:    cfg.Node.Address,
				PrivateIPv6:    cfg.Node.Address6,
				PublicEndpoint: cfg.Node.Endpoint,
			},
		})
	}

	jsoncfg, _ := json.Marshal(struct {
		Networks []Network `json:"networks"`
	}{nets})
	fmt.Println(string(jsoncfg))

	return nil
}

func getPeers(network string) ([]Peer, error) {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return []Peer{}, err
	}
	nodecfg := cfg.Node
	var nodes []models.Node

	var wcclient nodepb.NodeServiceClient
	conn, err := grpc.Dial(cfg.Server.GRPCAddress,
		ncutils.GRPCRequestOpts(cfg.Server.GRPCSSL))

	if err != nil {
		log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
	}
	defer conn.Close()
	// Instantiate the BlogServiceClient with our client connection to the server
	wcclient = nodepb.NewNodeServiceClient(conn)

	req := &nodepb.Object{
		Data: nodecfg.MacAddress + "###" + nodecfg.Network,
		Type: nodepb.STRING_TYPE,
	}

	ctx, err := auth.SetJWT(wcclient, network)
	if err != nil {
		log.Println("Failed to authenticate.")
		return []Peer{}, err
	}
	var header metadata.MD

	response, err := wcclient.GetPeers(ctx, req, grpc.Header(&header))
	if err != nil {
		log.Println("Error retrieving peers")
		return []Peer{}, err
	}
	if err := json.Unmarshal([]byte(response.GetData()), &nodes); err != nil {
		log.Println("Error unmarshaling data for peers")
		return []Peer{}, err
	}

	peers := []Peer{}
	for _, node := range nodes {
		if node.Name != cfg.Node.Name {
			peers = append(peers, Peer{Name: fmt.Sprintf("%v.%v", node.Name, network), PrivateIPv4: node.Address, PrivateIPv6: node.Address6})
		}
	}

	return peers, nil
}
