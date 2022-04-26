package functions

import (
	"encoding/json"
	"fmt"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// Peer - the peer struct for list
type Peer struct {
	Name           string `json:"name"`
	Interface      string `json:"interface,omitempty"`
	PrivateIPv4    string `json:"private_ipv4,omitempty"`
	PrivateIPv6    string `json:"private_ipv6,omitempty"`
	PublicEndpoint string `json:"public_endpoint,omitempty"`
}

// Network - the local node network representation for list command
type Network struct {
	Name        string `json:"name"`
	ID          string `json:"node_id"`
	CurrentNode Peer   `json:"current_node"`
	Peers       []Peer `json:"peers"`
}

// List - lists the current peers for the local node with name and node ID
func List(network string) error {
	nets := []Network{}
	var err error
	var networks []string
	if network == "all" {
		networks, err = ncutils.GetSystemNetworks()
		if err != nil {
			return err
		}
	} else {
		networks = append(networks, network)
	}

	for _, network := range networks {
		net, err := getNetwork(network)
		if err != nil {
			logger.Log(1, network+": Could not retrieve network configuration.")
			return err
		}
		nets = append(nets, net)
	}

	jsoncfg, _ := json.Marshal(struct {
		Networks []Network `json:"networks"`
	}{nets})
	fmt.Println(string(jsoncfg))

	return nil
}

func getNetwork(network string) (Network, error) {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return Network{}, fmt.Errorf("reading configuration for network %v: %w", network, err)
	}
	//peers, err := getPeers(network)
	peers := []Peer{}
	if err != nil {
		return Network{}, fmt.Errorf("listing peers for network %v: %w", network, err)
	}
	return Network{
		Name:  network,
		ID:    cfg.Node.ID,
		Peers: peers,
		CurrentNode: Peer{
			Name:           cfg.Node.Name,
			Interface:      cfg.Node.Interface,
			PrivateIPv4:    cfg.Node.Address,
			PrivateIPv6:    cfg.Node.Address6,
			PublicEndpoint: cfg.Node.Endpoint,
		},
	}, nil
}
