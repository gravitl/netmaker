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
	return n.allocate(ctx, network, schema.IPv4, schema.IPOwnerNode)
}

func (n *NetworkOrchestrator) AllocateExtclientIP(ctx context.Context, network *schema.Network) (net.IP, error) {
	return n.allocate(ctx, network, schema.IPv4, schema.IPOwnerExtClient)
}

func (n *NetworkOrchestrator) AllocateNodeIPv6(ctx context.Context, network *schema.Network) (net.IP, error) {
	return n.allocate(ctx, network, schema.IPv6, schema.IPOwnerNode)
}

func (n *NetworkOrchestrator) AllocateExtclientIPv6(ctx context.Context, network *schema.Network) (net.IP, error) {
	return n.allocate(ctx, network, schema.IPv6, schema.IPOwnerExtClient)
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

// ReleaseIP releases the given address (plain or in CIDR notation) on the
// network, so that it can be reallocated.
func (n *NetworkOrchestrator) ReleaseIP(ctx context.Context, network *schema.Network, ip string) error {
	addr, err := n.parseAddr(ip)
	if err != nil {
		return err
	}
	allocation := &schema.IPAllocation{
		TenantID:  network.TenantID,
		NetworkID: network.ID,
	}
	allocation.SetAddress(addr)
	return allocation.Release(ctx)
}

// allocate allocates an address of the given family on the network, in its own
// transaction.
//
// TODO: allocations should share the transaction of the node or extclient
// being created, so that a failed or interrupted create does not leave the
// address attached (until the reconciler orphans it). Until then, callers
// release the address if the create fails.
func (n *NetworkOrchestrator) allocate(ctx context.Context, network *schema.Network, family schema.IPFamily, ownerType schema.IPOwnerType) (ip net.IP, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to allocate IPv%d address on network %s: %w", family, network.Name, err)
		}
	}()

	var addr netip.Addr
	err = db.FromContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := db.WithDB(ctx, tx)

		pool := &schema.IPPool{
			TenantID:  network.TenantID,
			NetworkID: network.ID,
			Family:    family,
		}
		if err := pool.GetForUpdate(txCtx); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrIPPoolNotFound
			}
			return err
		}

		var err error
		addr, err = n.allocateOrphaned(txCtx, pool, ownerType)
		if errors.Is(err, ErrNoOrphanedIP) {
			addr, err = n.allocateFromCursor(txCtx, network, pool, ownerType)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return addr.AsSlice(), nil
}

// allocateFromCursor allocates the next address after the node cursor (or
// before the extclient cursor), skipping addresses that are already allocated
// (e.g. claimed custom addresses). The cursors never cross each other.
// Caller must hold the pool lock.
func (n *NetworkOrchestrator) allocateFromCursor(ctx context.Context, network *schema.Network, pool *schema.IPPool, ownerType schema.IPOwnerType) (netip.Addr, error) {
	first, last, err := network.UsableIPRange(pool.Family)
	if err != nil {
		return netip.Addr{}, err
	}
	isNode := ownerType == schema.IPOwnerNode

	// nodes go from the node cursor up to the extclient cursor, extclients
	// from the extclient cursor down to the node cursor.
	start, bound := first, last
	cursor, otherCursor := pool.NodeCursor, pool.ExtCursor
	step, stepBack := netip.Addr.Next, netip.Addr.Prev
	if !isNode {
		start, bound = last, first
		cursor, otherCursor = pool.ExtCursor, pool.NodeCursor
		step, stepBack = netip.Addr.Prev, netip.Addr.Next
	}
	if cursor != "" {
		addr, err := n.parseCursor(cursor)
		if err != nil {
			return netip.Addr{}, err
		}
		start = step(addr)
	}
	if otherCursor != "" {
		addr, err := n.parseCursor(otherCursor)
		if err != nil {
			return netip.Addr{}, err
		}
		bound = stepBack(addr)
	}

	// withinBound reports whether addr has not gone past bound.
	withinBound := func(addr netip.Addr) bool {
		if !addr.IsValid() {
			return false
		}
		if isNode {
			return !bound.Less(addr)
		}
		return !addr.Less(bound)
	}
	moveCursor := func(addr netip.Addr) {
		if isNode {
			pool.NodeCursor = addr.String()
		} else {
			pool.ExtCursor = addr.String()
		}
	}

	var skipped netip.Addr
	for addr := start; withinBound(addr); addr = step(addr) {
		allocation := &schema.IPAllocation{
			TenantID:  pool.TenantID,
			NetworkID: pool.NetworkID,
		}
		allocation.SetAddress(addr)
		exists, err := allocation.Exists(ctx)
		if err != nil {
			return netip.Addr{}, err
		}
		if exists {
			skipped = addr
			continue
		}

		allocation.Family = pool.Family
		allocation.State = schema.IPAttached
		allocation.OwnerType = ownerType
		if err := allocation.Create(ctx); err != nil {
			return netip.Addr{}, err
		}
		moveCursor(addr)
		return addr, pool.UpdateCursors(ctx)
	}

	// don't walk over the skipped addresses again on the next allocation.
	if skipped.IsValid() {
		moveCursor(skipped)
		if err := pool.UpdateCursors(ctx); err != nil {
			return netip.Addr{}, err
		}
	}
	return netip.Addr{}, ErrIPPoolExhausted
}

// allocateOrphaned reallocates the orphaned address nearest to the start of
// the range for a node, or to its end for an extclient.
// Caller must hold the pool lock.
func (n *NetworkOrchestrator) allocateOrphaned(ctx context.Context, pool *schema.IPPool, ownerType schema.IPOwnerType) (netip.Addr, error) {
	allocation := &schema.IPAllocation{
		TenantID:  pool.TenantID,
		NetworkID: pool.NetworkID,
		Family:    pool.Family,
	}
	if err := allocation.GetFirstOrphaned(ctx, ownerType == schema.IPOwnerExtClient); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return netip.Addr{}, ErrNoOrphanedIP
		}
		return netip.Addr{}, err
	}
	addr := allocation.Address()
	if !addr.IsValid() {
		return netip.Addr{}, fmt.Errorf("invalid orphaned IP address %v", allocation.RawAddress)
	}
	return addr, allocation.Attach(ctx, ownerType)
}

// lockPool locks the network's pool of the given family until the end of the
// transaction in ctx.
func (n *NetworkOrchestrator) lockPool(ctx context.Context, network *schema.Network, family schema.IPFamily) error {
	pool := &schema.IPPool{
		TenantID:  network.TenantID,
		NetworkID: network.ID,
		Family:    family,
	}
	err := pool.GetForUpdate(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrIPPoolNotFound
	}
	return err
}

// parseAddr parses an address given either as a plain IP or in CIDR notation.
func (n *NetworkOrchestrator) parseAddr(address string) (netip.Addr, error) {
	if prefix, err := netip.ParsePrefix(address); err == nil {
		return prefix.Addr().Unmap(), nil
	}
	addr, err := netip.ParseAddr(address)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid IP address %q", address)
	}
	return addr.Unmap(), nil
}

func (n *NetworkOrchestrator) parseCursor(cursor string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(cursor)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid IP pool cursor %q: %w", cursor, err)
	}
	return addr, nil
}

func (n *NetworkOrchestrator) addrFamily(addr netip.Addr) schema.IPFamily {
	if addr.Is4() {
		return schema.IPv4
	}
	return schema.IPv6
}
