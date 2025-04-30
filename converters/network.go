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
	var _networks []schema.Network
	for _, network := range networks {
		_networks = append(_networks, ToSchemaNetwork(network))
	}

	return _networks
}

func ToModelNetwork(_network schema.Network) models.Network {
	return models.Network{
		AddressRange:        _network.AddressRange,
		AddressRange6:       _network.AddressRange6,
		NetID:               _network.ID,
		NodesLastModified:   _network.NodesLastModified.Unix(),
		NetworkLastModified: _network.NetworkLastModified.Unix(),
		DefaultInterface:    _network.DefaultInterface,
		DefaultListenPort:   _network.DefaultListenPort,
		NodeLimit:           _network.NodeLimit,
		DefaultPostDown:     _network.DefaultPostDown,
		DefaultKeepalive:    _network.DefaultKeepalive,
		AllowManualSignUp:   _network.AllowManualSignUp,
		IsIPv4:              _network.IsIPv4,
		IsIPv6:              _network.IsIPv6,
		DefaultUDPHolePunch: _network.DefaultUDPHolePunch,
		DefaultMTU:          _network.DefaultMTU,
		DefaultACL:          _network.DefaultACL,
		NameServers:         _network.NameServers,
	}
}

func ToModelNetworks(_networks []schema.Network) []models.Network {
	var modelNetworks []models.Network
	for _, network := range _networks {
		modelNetworks = append(modelNetworks, ToModelNetwork(network))
	}

	return modelNetworks
}
