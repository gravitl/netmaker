package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/c-robinson/iplib"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
)

type NetworkOrchestrator struct {
	addressLock  sync.RWMutex
	address6Lock sync.RWMutex
	pendingIPv4  map[string]map[string]struct{}
	pendingIPv6  map[string]map[string]struct{}
}

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
	n.addressLock.Lock()
	defer n.addressLock.Unlock()

	if network.AddressRange == "" {
		return nil, fmt.Errorf("IPv4 not configured on network %s", network.Name)
	}
	if _, _, err := net.ParseCIDR(network.AddressRange); err != nil {
		return nil, err
	}
	return n.findUniqueIPv4DB(ctx, network, reverse)
}

func (n *NetworkOrchestrator) allocateIPv6(ctx context.Context, network *schema.Network, reverse bool) (net.IP, error) {
	n.address6Lock.Lock()
	defer n.address6Lock.Unlock()

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
		pendingTaken := !servercfg.IsHA() && n.isIPv4PendingReserved(network.ID, addr.String())
		if !pendingTaken && n.isIPv4UniqueInDB(ctx, network, addr.String()) {
			if !servercfg.IsHA() {
				n.reserveIPv4(network.ID, addr.String())
			}
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
		pendingTaken := !servercfg.IsHA() && n.isIPv6PendingReserved(network.ID, addr.String())
		if !pendingTaken && n.isIPv6UniqueInDB(ctx, network, addr.String()) {
			if !servercfg.IsHA() {
				n.reserveIPv6(network.ID, addr.String())
			}
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

// isIPv4PendingReserved reports whether ip is reserved in pendingIPv4.
// Caller must hold addressLock (read or write).
func (n *NetworkOrchestrator) isIPv4PendingReserved(networkID, ip string) bool {
	if pending, ok := n.pendingIPv4[networkID]; ok {
		if _, reserved := pending[ip]; reserved {
			return true
		}
	}
	return false
}

// isIPv6PendingReserved reports whether ip is reserved in pendingIPv6.
// Caller must hold address6Lock (read or write).
func (n *NetworkOrchestrator) isIPv6PendingReserved(networkID, ip string) bool {
	if pending, ok := n.pendingIPv6[networkID]; ok {
		if _, reserved := pending[ip]; reserved {
			return true
		}
	}
	return false
}

func (n *NetworkOrchestrator) IsIPv4Unique(ctx context.Context, network *schema.Network, ip string) bool {
	if !servercfg.IsHA() {
		n.addressLock.RLock()
		pendingReserved := n.isIPv4PendingReserved(network.ID, ip)
		n.addressLock.RUnlock()
		if pendingReserved {
			return false
		}
	}
	return n.isIPv4UniqueInDB(ctx, network, ip)
}

func (n *NetworkOrchestrator) isIPv4UniqueInDB(ctx context.Context, network *schema.Network, ip string) bool {
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

func (n *NetworkOrchestrator) reserveIPv4(networkID, ip string) {
	if n.pendingIPv4 == nil {
		n.pendingIPv4 = make(map[string]map[string]struct{})
	}
	if n.pendingIPv4[networkID] == nil {
		n.pendingIPv4[networkID] = make(map[string]struct{})
	}
	n.pendingIPv4[networkID][ip] = struct{}{}
}

func (n *NetworkOrchestrator) reserveIPv6(networkID, ip string) {
	if n.pendingIPv6 == nil {
		n.pendingIPv6 = make(map[string]map[string]struct{})
	}
	if n.pendingIPv6[networkID] == nil {
		n.pendingIPv6[networkID] = make(map[string]struct{})
	}
	n.pendingIPv6[networkID][ip] = struct{}{}
}

func (n *NetworkOrchestrator) FreeIPv4Reservation(networkID, ip string) {
	if servercfg.IsHA() {
		return
	}
	n.addressLock.Lock()
	defer n.addressLock.Unlock()
	if n.pendingIPv4 != nil {
		delete(n.pendingIPv4[networkID], ip)
	}
}

func (n *NetworkOrchestrator) FreeIPv6Reservation(networkID, ip string) {
	if servercfg.IsHA() {
		return
	}
	n.address6Lock.Lock()
	defer n.address6Lock.Unlock()
	if n.pendingIPv6 != nil {
		delete(n.pendingIPv6[networkID], ip)
	}
}

func (n *NetworkOrchestrator) IsIPv6Unique(ctx context.Context, network *schema.Network, ip string) bool {
	if !servercfg.IsHA() {
		n.address6Lock.RLock()
		pendingReserved := n.isIPv6PendingReserved(network.ID, ip)
		n.address6Lock.RUnlock()
		if pendingReserved {
			return false
		}
	}
	return n.isIPv6UniqueInDB(ctx, network, ip)
}

func (n *NetworkOrchestrator) isIPv6UniqueInDB(ctx context.Context, network *schema.Network, ip string) bool {
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
