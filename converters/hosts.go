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

func ToModelHost(host schema.Host) models.Host {
	var publicKey wgtypes.Key
	if host.PublicKey != "" {
		publicKey, _ = wgtypes.ParseKey(host.PublicKey)
	}

	var macAddress net.HardwareAddr
	if host.MacAddress != "" {
		macAddress, _ = net.ParseMAC(host.MacAddress)
	}

	var interfaces []models.Iface
	for i := range host.Interfaces {
		iface := models.Iface{
			Name: host.Interfaces[i].Name,
		}

		if host.Interfaces[i].Address != "" {
			_, address, _ := net.ParseCIDR(host.Interfaces[i].Address)
			if address != nil {
				iface.Address = *address
				iface.AddressString = address.String()
			}
		}

		interfaces = append(interfaces, iface)
	}

	var turnEndpoint *netip.AddrPort
	if host.TurnEndpoint != "" {
		addrPost, _ := netip.ParseAddrPort(host.TurnEndpoint)
		turnEndpoint = &addrPost
	}

	return models.Host{
		ID:                 uuid.MustParse(host.ID),
		Verbosity:          host.Verbosity,
		FirewallInUse:      host.FirewallInUse,
		Version:            host.Version,
		IPForwarding:       host.IPForwarding,
		DaemonInstalled:    host.DaemonInstalled,
		AutoUpdate:         host.AutoUpdate,
		HostPass:           host.Password,
		Name:               host.Name,
		OS:                 host.OS,
		Interface:          host.Interface,
		Debug:              host.Debug,
		ListenPort:         host.ListenPort,
		WgPublicListenPort: host.WgPublicListenPort,
		MTU:                host.MTU,
		PublicKey:          publicKey,
		MacAddress:         macAddress,
		TrafficKeyPublic:   []byte(host.TrafficKeyPublic),
		// TODO: Set Value.
		Nodes:               nil,
		Interfaces:          interfaces,
		DefaultInterface:    host.DefaultInterface,
		EndpointIP:          net.ParseIP(host.EndpointIP),
		EndpointIPv6:        net.ParseIP(host.EndpointIPv6),
		IsDocker:            host.IsDocker,
		IsK8S:               host.IsK8S,
		IsStaticPort:        host.IsStaticPort,
		IsStatic:            host.IsStatic,
		IsDefault:           host.IsDefault,
		NatType:             host.NatType,
		TurnEndpoint:        turnEndpoint,
		PersistentKeepalive: host.PersistentKeepalive,
	}
}
