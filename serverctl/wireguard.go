package serverctl

import (
	"github.com/gravitl/netmaker/functions"
	"golang.zx2c4.com/wireguard/wgctrl"
)

func GetPeers(networkName string) (map[string]string, error) {
	peers := make(map[string]string)
	network, err := functions.GetParentNetwork(networkName)
	if err != nil {
		return peers, err
	}
	iface := network.DefaultInterface

	client, err := wgctrl.New()
	if err != nil {
		return peers, err
	}
	device, err := client.Device(iface)
	if err != nil {
		return nil, err
	}
	for _, peer := range device.Peers {
		if functions.IsBase64(peer.PublicKey.String()) && peer.Endpoint != nil && functions.CheckEndpoint(peer.Endpoint.String()) {
			peers[peer.PublicKey.String()] = peer.Endpoint.String()
		}
	}
	return peers, nil
}
