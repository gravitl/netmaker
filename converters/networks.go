package converters

import (
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"time"
)

func ToSchemaNetwork(network models.Network) schema.Network {
	return schema.Network{
		ID:                  network.NetID,
		IsIPv4:              network.IsIPv4,
		IsIPv6:              network.IsIPv6,
		AddressRange:        network.AddressRange,
		AddressRange6:       network.AddressRange6,
		NodeLimit:           network.NodeLimit,
		AllowManualSignUp:   network.AllowManualSignUp,
		DefaultInterface:    network.DefaultInterface,
		DefaultPostDown:     network.DefaultPostDown,
		DefaultUDPHolePunch: network.DefaultUDPHolePunch,
		DefaultACL:          network.DefaultACL,
		DefaultListenPort:   network.DefaultListenPort,
		DefaultKeepalive:    network.DefaultKeepalive,
		DefaultMTU:          network.DefaultMTU,
		NameServers:         network.NameServers,
		NodesLastModified:   time.Unix(network.NodesLastModified, 0),
		NetworkLastModified: time.Unix(network.NetworkLastModified, 0),
	}
}

func ToSchemaNetworks(networks []models.Network) []schema.Network {
	var schemaNetworks []schema.Network
	for _, network := range networks {
		schemaNetworks = append(schemaNetworks, ToSchemaNetwork(network))
	}

	return schemaNetworks
}

func ToModelNetwork(network schema.Network) models.Network {
	return models.Network{
		AddressRange:        network.AddressRange,
		AddressRange6:       network.AddressRange6,
		NetID:               network.ID,
		NodesLastModified:   network.NodesLastModified.Unix(),
		NetworkLastModified: network.NetworkLastModified.Unix(),
		DefaultInterface:    network.DefaultInterface,
		DefaultListenPort:   network.DefaultListenPort,
		NodeLimit:           network.NodeLimit,
		DefaultPostDown:     network.DefaultPostDown,
		DefaultKeepalive:    network.DefaultKeepalive,
		AllowManualSignUp:   network.AllowManualSignUp,
		IsIPv4:              network.IsIPv4,
		IsIPv6:              network.IsIPv6,
		DefaultUDPHolePunch: network.DefaultUDPHolePunch,
		DefaultMTU:          network.DefaultMTU,
		DefaultACL:          network.DefaultACL,
		NameServers:         network.NameServers,
	}
}

func ToModelNetworks(networks []schema.Network) []models.Network {
	var modelNetworks []models.Network
	for _, network := range networks {
		modelNetworks = append(modelNetworks, ToModelNetwork(network))
	}

	return modelNetworks
}
