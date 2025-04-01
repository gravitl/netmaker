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
	var interfaces []schema.Interface
	for i := range host.Interfaces {
		interfaces = append(interfaces, schema.Interface{
			HostID:  host.ID.String(),
			Name:    host.Interfaces[i].Name,
			Address: host.Interfaces[i].Address.String(),
		})
	}

	var turnEndpoint string
	if host.TurnEndpoint != nil {
		turnEndpoint = host.TurnEndpoint.String()
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
		TrafficKeyPublic:    string(host.TrafficKeyPublic),
	}
}

func ToSchemaHosts(hosts []models.Host) []schema.Host {
	var schemaHosts []schema.Host
	for _, host := range hosts {
		schemaHosts = append(schemaHosts, ToSchemaHost(host))
	}

	return schemaHosts
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

	var interfaces []models.Iface
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

		interfaces = append(interfaces, iface)
	}

	var turnEndpoint *netip.AddrPort
	if _host.TurnEndpoint != "" {
		addrPost, _ := netip.ParseAddrPort(_host.TurnEndpoint)
		turnEndpoint = &addrPost
	}

	return models.Host{
		ID:                 uuid.MustParse(_host.ID),
		Verbosity:          _host.Verbosity,
		FirewallInUse:      _host.FirewallInUse,
		Version:            _host.Version,
		IPForwarding:       _host.IPForwarding,
		DaemonInstalled:    _host.DaemonInstalled,
		AutoUpdate:         _host.AutoUpdate,
		HostPass:           _host.Password,
		Name:               _host.Name,
		OS:                 _host.OS,
		Interface:          _host.Interface,
		Debug:              _host.Debug,
		ListenPort:         _host.ListenPort,
		WgPublicListenPort: _host.WgPublicListenPort,
		MTU:                _host.MTU,
		PublicKey:          publicKey,
		MacAddress:         macAddress,
		TrafficKeyPublic:   []byte(_host.TrafficKeyPublic),
		// TODO: Set Value.
		Nodes:               nil,
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
