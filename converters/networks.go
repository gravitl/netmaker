package converters

import (
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func ToSchemaNetwork(network models.Network) schema.Network {
	return schema.Network{
		ID:                  "",
		Name:                network.NetID,
		AddressRange:        network.AddressRange,
		AddressRange6:       network.AddressRange6,
		NameServers:         network.NameServers,
		DefaultKeepAlive:    time.Duration(network.DefaultKeepalive) * time.Second,
		DefaultACL:          network.DefaultACL,
		DefaultMTU:          network.DefaultMTU,
		AutoJoin:            network.AutoJoin,
		AutoRemove:          network.AutoRemove,
		AutoRemoveTags:      network.AutoRemoveTags,
		AutoRemoveThreshold: time.Duration(network.AutoRemoveThreshold) * time.Minute,
		NodesUpdatedAt:      time.Unix(network.NodesLastModified, 0),
		CreatedBy:           network.CreatedBy,
		CreatedAt:           network.CreatedAt,
		UpdatedAt:           time.Unix(network.NetworkLastModified, 0),
	}
}

func ToSchemaNetworks(networks []models.Network) []schema.Network {
	_networks := make([]schema.Network, len(networks))
	for i, network := range networks {
		_networks[i] = ToSchemaNetwork(network)
	}

	return _networks
}

func ToModelNetwork(_network schema.Network) models.Network {
	isIPv4 := "no"
	if _network.AddressRange != "" {
		isIPv4 = "yes"
	}

	isIPv6 := "no"
	if _network.AddressRange6 != "" {
		isIPv6 = "yes"
	}

	return models.Network{
		AddressRange:        _network.AddressRange,
		AddressRange6:       _network.AddressRange6,
		NetID:               _network.Name,
		NodesLastModified:   _network.NodesUpdatedAt.Unix(),
		NetworkLastModified: _network.UpdatedAt.Unix(),
		DefaultKeepalive:    int32(_network.DefaultKeepAlive.Seconds()),
		IsIPv4:              isIPv4,
		IsIPv6:              isIPv6,
		DefaultMTU:          _network.DefaultMTU,
		DefaultACL:          _network.DefaultACL,
		NameServers:         _network.NameServers,
		AutoJoin:            _network.AutoJoin,
		AutoRemove:          _network.AutoRemove,
		AutoRemoveTags:      _network.AutoRemoveTags,
		AutoRemoveThreshold: int(_network.AutoRemoveThreshold.Minutes()),
		CreatedBy:           _network.CreatedBy,
	}
}

func ToModelNetworks(_networks []schema.Network) []models.Network {
	networks := make([]models.Network, len(_networks))
	for i, network := range _networks {
		networks[i] = ToModelNetwork(network)
	}

	return networks
}
