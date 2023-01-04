package functions

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// Peer - the peer struct for list
type Peer struct {
	Name           string    `json:"name,omitempty"`
	Interface      string    `json:"interface,omitempty"`
	PrivateIPv4    string    `json:"private_ipv4,omitempty"`
	PrivateIPv6    string    `json:"private_ipv6,omitempty"`
	PublicKey      string    `json:"public_key,omitempty"`
	PublicEndpoint string    `json:"public_endpoint,omitempty"`
	Addresses      []address `json:"addresses,omitempty"`
}

// Network - the local node network representation for list command
type Network struct {
	Name        string `json:"name"`
	ID          string `json:"node_id"`
	CurrentNode Peer   `json:"current_node"`
	Peers       []Peer `json:"peers"`
}

type address struct {
	CIDR string `json:"cidr,omitempty"`
	IP   string `json:"ip,omitempty"`
}

// List - lists the current peers for the local node with name and node ID
func List(network string) ([]Network, error) {
	nets := []Network{}
	var err error
	var networks []string
	if network == "all" {
		networks, err = ncutils.GetSystemNetworks()
		if err != nil {
			return nil, err
		}
	} else {
		networks = append(networks, network)
	}

	for _, network := range networks {
		net, err := getNetwork(network)
		if err != nil {
			logger.Log(1, network+": Could not retrieve network configuration.")
			return nil, err
		}
		peers, err := getPeers(network)
		if err == nil && len(peers) > 0 {
			net.Peers = peers
		}
		nets = append(nets, net)
	}

	jsoncfg, _ := json.Marshal(struct {
		Networks []Network `json:"networks"`
	}{nets})
	fmt.Println(string(jsoncfg))

	return nets, nil
}

func getNetwork(network string) (Network, error) {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return Network{}, fmt.Errorf("reading configuration for network %v: %w", network, err)
	}
	// peers, err := getPeers(network)
	peers := []Peer{}
	/*	if err != nil {
		return Network{}, fmt.Errorf("listing peers for network %v: %w", network, err)
	}*/
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

func getPeers(network string) ([]Peer, error) {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return []Peer{}, err
	}
	token, err := Authenticate(cfg)
	if err != nil {
		return nil, err
	}
	url := "https://" + cfg.Server.API + "/api/nodes/" + cfg.Network + "/" + cfg.Node.ID
	response, err := API("", http.MethodGet, url, token)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		bytes, err := io.ReadAll(response.Body)
		if err != nil {
			fmt.Println(err)
		}
		return nil, (fmt.Errorf("%s %w", string(bytes), err))
	}
	defer response.Body.Close()
	var nodeGET models.NodeGet
	if err := json.NewDecoder(response.Body).Decode(&nodeGET); err != nil {
		return nil, fmt.Errorf("error decoding node %w", err)
	}
	if nodeGET.Peers == nil {
		nodeGET.Peers = []wgtypes.PeerConfig{}
	}

	peers := []Peer{}
	for _, peer := range nodeGET.Peers {
		var addresses = []address{}
		for j := range peer.AllowedIPs {
			newAddress := address{
				CIDR: peer.AllowedIPs[j].String(),
				IP:   peer.AllowedIPs[j].IP.String(),
			}
			addresses = append(addresses, newAddress)
		}
		peers = append(peers, Peer{
			PublicKey:      peer.PublicKey.String(),
			PublicEndpoint: peer.Endpoint.String(),
			Addresses:      addresses,
		})
	}

	return peers, nil
}
