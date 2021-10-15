package tests

import "github.com/gravitl/netmaker/models"

func DeleteAllNetworks() {
	deleteAllNodes()
	nets, _ := models.GetNetworks()
	for _, net := range nets {
		DeleteNetwork(net.NetID)
	}
}

func CreateNet() {
	var network models.Network
	network.NetID = "skynet"
	network.AddressRange = "10.0.0.1/24"
	network.DisplayName = "mynetwork"
	_, err := GetNetwork("skynet")
	if err != nil {
		CreateNetwork(network)
	}
}

func GetNet() models.Network {
	network, _ := GetNetwork("skynet")
	return network
}
