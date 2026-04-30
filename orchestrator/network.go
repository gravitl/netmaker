package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/c-robinson/iplib"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
)

type NetworkOrchestrator struct{}

func (n *NetworkOrchestrator) AllocateNodeIP(ctx context.Context, network *schema.Network) (net.IP, error) {
	return n.allocateIPv4(ctx, network, false)
}

func (n *NetworkOrchestrator) AllocateExtclientIP(ctx context.Context, network *schema.Network) (net.IP, error) {
	return n.allocateIPv4(ctx, network, true)
}

func (n *NetworkOrchestrator) AllocateNodeIPv6(ctx context.Context, network *schema.Network) (net.IP, error) {
	return n.allocateIPv6(ctx, network, false)
}

func (n *NetworkOrchestrator) AllocateExtclientIPv6(ctx context.Context, network *schema.Network) (net.IP, error) {
	return n.allocateIPv6(ctx, network, true)
}

func (n *NetworkOrchestrator) allocateIPv4(ctx context.Context, network *schema.Network, reverse bool) (net.IP, error) {
	if network.AddressRange == "" {
		return nil, fmt.Errorf("IPv4 not configured on network %s", network.Name)
	}
	if _, _, err := net.ParseCIDR(network.AddressRange); err != nil {
		return nil, err
	}
	return n.findUniqueIPv4DB(ctx, network, reverse)
}

func (n *NetworkOrchestrator) allocateIPv6(ctx context.Context, network *schema.Network, reverse bool) (net.IP, error) {
	if network.AddressRange6 == "" {
		return nil, fmt.Errorf("IPv6 not configured on network %s", network.Name)
	}
	if _, _, err := net.ParseCIDR(network.AddressRange6); err != nil {
		return nil, err
	}
	return n.findUniqueIPv6DB(ctx, network, reverse)
}

func (n *NetworkOrchestrator) findUniqueIPv4DB(ctx context.Context, network *schema.Network, reverse bool) (net.IP, error) {
	net4 := iplib.Net4FromStr(network.AddressRange)
	addr := net4.FirstAddress()
	if reverse {
		addr = net4.LastAddress()
	}

	for {
		if n.isIPv4UniqueDB(ctx, network, addr.String()) {
			return addr, nil
		}
		var err error
		if reverse {
			addr, err = net4.PreviousIP(addr)
		} else {
			addr, err = net4.NextIP(addr)
		}
		if err != nil {
			return nil, errors.New("no unique IPv4 addresses available")
		}
	}
}

func (n *NetworkOrchestrator) findUniqueIPv6DB(ctx context.Context, network *schema.Network, reverse bool) (net.IP, error) {
	net6 := iplib.Net6FromStr(network.AddressRange6)

	var (
		addr net.IP
		err  error
	)
	if reverse {
		addr, err = net6.PreviousIP(net6.LastAddress())
	} else {
		addr, err = net6.NextIP(net6.FirstAddress())
	}
	if err != nil {
		return nil, err
	}

	for {
		if n.isIPv6UniqueDB(ctx, network, addr.String()) {
			return addr, nil
		}
		if reverse {
			addr, err = net6.PreviousIP(addr)
		} else {
			addr, err = net6.NextIP(addr)
		}
		if err != nil {
			return nil, errors.New("no unique IPv6 addresses available")
		}
	}
}

func (n *NetworkOrchestrator) isIPv4UniqueDB(ctx context.Context, network *schema.Network, ip string) bool {
	_, cidr, err := net.ParseCIDR(network.AddressRange)
	if err != nil {
		return true
	}
	cidr.IP = net.ParseIP(ip)
	node := &schema.Node{NetworkID: network.ID, Address: cidr.String()}
	if err := node.GetByNetworkAndAddress(ctx); err == nil {
		return false
	}

	extClients, err := logic.GetNetworkExtClients(network.Name)
	if err != nil {
		return true
	}
	for _, ec := range extClients {
		if ec.Address == ip {
			return false
		}
	}
	return true
}

func (n *NetworkOrchestrator) isIPv6UniqueDB(ctx context.Context, network *schema.Network, ip string) bool {
	_, cidr, err := net.ParseCIDR(network.AddressRange6)
	if err != nil {
		return true
	}
	cidr.IP = net.ParseIP(ip)
	node := &schema.Node{NetworkID: network.ID, Address6: cidr.String()}
	if err := node.GetByNetworkAndAddress6(ctx); err == nil {
		return false
	}

	extClients, err := logic.GetNetworkExtClients(network.Name)
	if err != nil {
		return true
	}
	for _, ec := range extClients {
		if ec.Address6 == ip {
			return false
		}
	}
	return true
}
