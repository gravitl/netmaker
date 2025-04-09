package converters

import (
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"net"
	"net/netip"
)

func ToSchemaHost(host models.Host) schema.Host {
	interfaces := make([]schema.Interface, len(host.Interfaces))
	for i := range host.Interfaces {
		interfaces[i] = schema.Interface{
			Name:    host.Interfaces[i].Name,
			Address: host.Interfaces[i].Address.String(),
		}
	}

	var turnEndpoint string
	if host.TurnEndpoint != nil {
		turnEndpoint = host.TurnEndpoint.String()
	}

	_nodes := make([]schema.Node, len(host.Nodes))
	for i, nodeID := range host.Nodes {
		_nodes[i] = schema.Node{
			ID: nodeID,
		}
	}

	return schema.Host{
		ID:                  host.ID.String(),
		Name:                host.Name,
		Password:            host.HostPass,
		Version:             host.Version,
		OS:                  host.OS,
		DaemonInstalled:     host.DaemonInstalled,
		AutoUpdate:          host.AutoUpdate,
		Verbosity:           host.Verbosity,
		Debug:               host.Debug,
		IsDocker:            host.IsDocker,
		IsK8S:               host.IsK8S,
		IsStaticPort:        host.IsStaticPort,
		IsStatic:            host.IsStatic,
		IsDefault:           host.IsDefault,
		MacAddress:          host.MacAddress.String(),
		EndpointIP:          host.EndpointIP.String(),
		EndpointIPv6:        host.EndpointIPv6.String(),
		TurnEndpoint:        turnEndpoint,
		NatType:             host.NatType,
		ListenPort:          host.ListenPort,
		WgPublicListenPort:  host.WgPublicListenPort,
		MTU:                 host.MTU,
		FirewallInUse:       host.FirewallInUse,
		IPForwarding:        host.IPForwarding,
		PersistentKeepalive: host.PersistentKeepalive,
		Interfaces:          interfaces,
		DefaultInterface:    host.DefaultInterface,
		Interface:           host.Interface,
		PublicKey:           host.PublicKey.String(),
		TrafficKeyPublic:    host.TrafficKeyPublic,
		Nodes:               _nodes,
	}
}

func ToSchemaHosts(hosts []models.Host) []schema.Host {
	_hosts := make([]schema.Host, len(hosts))
	for i, host := range hosts {
		_hosts[i] = ToSchemaHost(host)
	}

	return _hosts
}

func ToModelHost(_host schema.Host) models.Host {
	var publicKey wgtypes.Key
	if _host.PublicKey != "" {
		publicKey, _ = wgtypes.ParseKey(_host.PublicKey)
	}

	var macAddress net.HardwareAddr
	if _host.MacAddress != "" {
		macAddress, _ = net.ParseMAC(_host.MacAddress)
	}

	nodes := make([]string, len(_host.Nodes))
	for i, node := range _host.Nodes {
		nodes[i] = node.ID
	}

	interfaces := make([]models.Iface, len(_host.Interfaces))
	for i := range _host.Interfaces {
		iface := models.Iface{
			Name: _host.Interfaces[i].Name,
		}

		if _host.Interfaces[i].Address != "" {
			_, address, _ := net.ParseCIDR(_host.Interfaces[i].Address)
			if address != nil {
				iface.Address = *address
				iface.AddressString = address.String()
			}
		}

		interfaces[i] = iface
	}

	var turnEndpoint *netip.AddrPort
	if _host.TurnEndpoint != "" {
		addrPost, _ := netip.ParseAddrPort(_host.TurnEndpoint)
		turnEndpoint = &addrPost
	}

	return models.Host{
		ID:                  uuid.MustParse(_host.ID),
		Verbosity:           _host.Verbosity,
		FirewallInUse:       _host.FirewallInUse,
		Version:             _host.Version,
		IPForwarding:        _host.IPForwarding,
		DaemonInstalled:     _host.DaemonInstalled,
		AutoUpdate:          _host.AutoUpdate,
		HostPass:            _host.Password,
		Name:                _host.Name,
		OS:                  _host.OS,
		Interface:           _host.Interface,
		Debug:               _host.Debug,
		ListenPort:          _host.ListenPort,
		WgPublicListenPort:  _host.WgPublicListenPort,
		MTU:                 _host.MTU,
		PublicKey:           publicKey,
		MacAddress:          macAddress,
		TrafficKeyPublic:    _host.TrafficKeyPublic,
		Nodes:               nodes,
		Interfaces:          interfaces,
		DefaultInterface:    _host.DefaultInterface,
		EndpointIP:          net.ParseIP(_host.EndpointIP),
		EndpointIPv6:        net.ParseIP(_host.EndpointIPv6),
		IsDocker:            _host.IsDocker,
		IsK8S:               _host.IsK8S,
		IsStaticPort:        _host.IsStaticPort,
		IsStatic:            _host.IsStatic,
		IsDefault:           _host.IsDefault,
		NatType:             _host.NatType,
		TurnEndpoint:        turnEndpoint,
		PersistentKeepalive: _host.PersistentKeepalive,
	}
}

func ToModelHosts(_hosts []schema.Host) []models.Host {
	hosts := make([]models.Host, len(_hosts))
	for i, _host := range _hosts {
		hosts[i] = ToModelHost(_host)
	}

	return hosts
}

func ToAPIHost(_host schema.Host) models.ApiHost {
	interfaces := make([]models.ApiIface, len(_host.Interfaces))
	for i := range _host.Interfaces {
		iface := models.ApiIface{
			Name: _host.Interfaces[i].Name,
		}

		if _host.Interfaces[i].Address != "" {
			_, address, _ := net.ParseCIDR(_host.Interfaces[i].Address)
			if address != nil {
				iface.AddressString = address.String()
			}
		}

		interfaces[i] = iface
	}

	nodes := make([]string, len(_host.Nodes))
	for i, node := range _host.Nodes {
		nodes[i] = node.ID
	}

	return models.ApiHost{
		ID:                  _host.ID,
		Verbosity:           _host.Verbosity,
		FirewallInUse:       _host.FirewallInUse,
		Version:             _host.Version,
		Name:                _host.Name,
		OS:                  _host.OS,
		Debug:               _host.Debug,
		IsStaticPort:        _host.IsStaticPort,
		IsStatic:            _host.IsStatic,
		ListenPort:          _host.ListenPort,
		WgPublicListenPort:  _host.WgPublicListenPort,
		MTU:                 _host.MTU,
		Interfaces:          interfaces,
		DefaultInterface:    _host.DefaultInterface,
		EndpointIP:          _host.EndpointIP,
		EndpointIPv6:        _host.EndpointIPv6,
		PublicKey:           _host.PublicKey,
		MacAddress:          _host.MacAddress,
		Nodes:               nodes,
		IsDefault:           _host.IsDefault,
		NatType:             _host.NatType,
		PersistentKeepalive: int(_host.PersistentKeepalive.Seconds()),
		AutoUpdate:          _host.AutoUpdate,
	}
}

func ToAPIHosts(_hosts []schema.Host) []models.ApiHost {
	hosts := make([]models.ApiHost, len(_hosts))
	for i, _host := range _hosts {
		hosts[i] = ToAPIHost(_host)
	}

	return hosts
}
