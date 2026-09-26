package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"time"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
	"gorm.io/gorm"
)

var (
	// ErrIPPoolExhausted is returned when a network has no address left to
	// allocate.
	ErrIPPoolExhausted = errors.New("no IP addresses available in the network")
	// ErrNoOrphanedIP is returned when a network has no orphaned address to
	// reallocate.
	ErrNoOrphanedIP = errors.New("no orphaned IP addresses in the network")
	// ErrIPAlreadyAllocated is returned when claiming an address that is in
	// use.
	ErrIPAlreadyAllocated = errors.New("IP address is already allocated")
	// ErrIPPoolNotFound is returned when a network has no address pool, e.g.
	// if the IP allocations migration has not run yet.
	ErrIPPoolNotFound = errors.New("IP address pool not found for the network")
)

// NetworkOrchestrator allocates the addresses of nodes and extclients.
//
// Allocations are tracked in the DB (ip_pools_v1, ip_allocations_v1), so they
// are consistent across server replicas: every allocation locks the network's
// pool row, and the allocations table's unique index guarantees that an address
// is never handed out twice.
//
// Nodes are allocated from the start of the network's range and extclients
// from its end. Released (orphaned) addresses are reused first, the one nearest
// to the peer's end of the range; otherwise addresses are allocated by cursors,
// nodes upwards and extclients downwards.
//
// Addresses must be allocated (or claimed) before the node or extclient using
// them is saved, and released when it is deleted, so that every address in use
// is attached. Attached addresses left unused, e.g. by a failed save, are
// orphaned by ReconcileIPAllocations.
type NetworkOrchestrator struct{}

// AllocateNodeIP allocates an IPv4 address on the network to the node with
// the given ID.
func (n *NetworkOrchestrator) AllocateNodeIP(ctx context.Context, network *schema.Network, nodeID string) (net.IP, error) {
	return n.allocate(ctx, network, schema.IPv4, schema.IPOwnerNode, nodeID)
}

// AllocateExtclientIP allocates an IPv4 address on the network to the
// extclient with the given ID.
func (n *NetworkOrchestrator) AllocateExtclientIP(ctx context.Context, network *schema.Network, extClientID string) (net.IP, error) {
	return n.allocate(ctx, network, schema.IPv4, schema.IPOwnerExtClient, extClientID)
}

// AllocateNodeIPv6 allocates an IPv6 address on the network to the node with
// the given ID.
func (n *NetworkOrchestrator) AllocateNodeIPv6(ctx context.Context, network *schema.Network, nodeID string) (net.IP, error) {
	return n.allocate(ctx, network, schema.IPv6, schema.IPOwnerNode, nodeID)
}

// AllocateExtclientIPv6 allocates an IPv6 address on the network to the
// extclient with the given ID.
func (n *NetworkOrchestrator) AllocateExtclientIPv6(ctx context.Context, network *schema.Network, extClientID string) (net.IP, error) {
	return n.allocate(ctx, network, schema.IPv6, schema.IPOwnerExtClient, extClientID)
}

// ClaimIP allocates the given address (plain or in CIDR notation) on the
// network to the given owner, e.g. when a custom address is set on a node.
//
// Returns ErrIPAlreadyAllocated if the address is in use.
func (n *NetworkOrchestrator) ClaimIP(ctx context.Context, network *schema.Network, ip string, ownerType schema.IPOwnerType, ownerID string) error {
	addr, err := n.parseAddr(ip)
	if err != nil {
		return err
	}
	family := n.addrFamily(addr)
	first, last, err := network.UsableIPRange(family)
	if err != nil {
		return err
	}
	if addr.Less(first) || last.Less(addr) {
		return fmt.Errorf("IP address %s is not in the range of network %s", addr, network.Name)
	}
	return db.FromContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := db.WithDB(ctx, tx)

		// lock the pool, so that the address is not allocated concurrently.
		if err := n.lockPool(txCtx, network, family); err != nil {
			return err
		}

		allocation := &schema.IPAllocation{
			TenantID:  network.TenantID,
			NetworkID: network.ID,
		}
		allocation.SetAddress(addr)
		err := allocation.Get(txCtx)
		switch {
		case err == nil && allocation.State == schema.IPAttached:
			return ErrIPAlreadyAllocated
		case err == nil:
			allocation.OwnerType = ownerType
			allocation.OwnerID = ownerID
			return allocation.Attach(txCtx)
		case errors.Is(err, gorm.ErrRecordNotFound):
			allocation.Family = family
			allocation.State = schema.IPAttached
			allocation.OwnerType = ownerType
			allocation.OwnerID = ownerID
			return allocation.Create(txCtx)
		default:
			return err
		}
	})
}

// ReleaseIP releases the given address (plain or in CIDR notation) on the
// network, so that it can be reallocated. The address is only released if it
// is allocated to the given owner.
func (n *NetworkOrchestrator) ReleaseIP(ctx context.Context, network *schema.Network, ip string, ownerType schema.IPOwnerType, ownerID string) error {
	addr, err := n.parseAddr(ip)
	if err != nil {
		return err
	}
	allocation := &schema.IPAllocation{
		TenantID:  network.TenantID,
		NetworkID: network.ID,
		OwnerType: ownerType,
		OwnerID:   ownerID,
	}
	allocation.SetAddress(addr)
	return allocation.Release(ctx)
}

// allocate allocates an address of the given family on the network to the
// given owner, in its own transaction.
//
// TODO: allocations should share the transaction of the node or extclient
// being created, so that a failed or interrupted create does not leave the
// address attached (until the reconciler orphans it). Until then, callers
// release the address if the create fails.
func (n *NetworkOrchestrator) allocate(ctx context.Context, network *schema.Network, family schema.IPFamily, ownerType schema.IPOwnerType, ownerID string) (ip net.IP, err error) {
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
		addr, err = n.allocateOrphaned(txCtx, pool, ownerType, ownerID)
		if errors.Is(err, ErrNoOrphanedIP) {
			addr, err = n.allocateFromCursor(txCtx, network, pool, ownerType, ownerID)
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
func (n *NetworkOrchestrator) allocateFromCursor(ctx context.Context, network *schema.Network, pool *schema.IPPool, ownerType schema.IPOwnerType, ownerID string) (netip.Addr, error) {
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
		allocation.OwnerID = ownerID
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
func (n *NetworkOrchestrator) allocateOrphaned(ctx context.Context, pool *schema.IPPool, ownerType schema.IPOwnerType, ownerID string) (netip.Addr, error) {
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
	allocation.OwnerType = ownerType
	allocation.OwnerID = ownerID
	return addr, allocation.Attach(ctx)
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

// ipAllocationGracePeriod is how long an attached address can go without a
// node or extclient using it before ReconcileIPAllocations orphans it. It
// covers the time between allocating an address and saving the peer using it.
const ipAllocationGracePeriod = 10 * time.Minute

// ipAllocationReconcileInterval is how often the address allocations are
// reconciled. Unused attached addresses only occur when a node or extclient
// fails to save and its address can't be released, and allocations keep
// working as long as the network has addresses left, so it can be long.
const ipAllocationReconcileInterval = 6 * time.Hour

// IPOwner is the node or extclient using an address.
type IPOwner struct {
	Type schema.IPOwnerType
	ID   string
}

func (o IPOwner) String() string {
	return fmt.Sprintf("%s %s", o.Type, o.ID)
}

// ListIPOwners lists the addresses used by the network's nodes and
// extclients. Addresses used more than once are reported in duplicates.
func (n *NetworkOrchestrator) ListIPOwners(ctx context.Context, network *schema.Network) (owners map[netip.Addr]IPOwner, duplicates []string, err error) {
	owners = make(map[netip.Addr]IPOwner)
	add := func(address string, owner IPOwner) {
		addr, err := n.parseAddr(address)
		if err != nil {
			return
		}
		if existing, ok := owners[addr]; ok {
			duplicates = append(duplicates, fmt.Sprintf("%s is used by %s and %s", addr, existing, owner))
			return
		}
		owners[addr] = owner
	}

	nodes, err := (&schema.Node{}).ListAll(ctx, dbtypes.WithFilter("nodes_v1.network_id", network.ID))
	if err != nil {
		return nil, nil, err
	}
	for _, node := range nodes {
		owner := IPOwner{Type: schema.IPOwnerNode, ID: node.ID}
		add(node.Address, owner)
		add(node.Address6, owner)
	}

	// extclient records only hold the network's name, within their tenant.
	records, err := (&schema.ExtClientRecord{TenantID: network.TenantID}).ListByNetwork(ctx, network.Name)
	if err != nil {
		return nil, nil, err
	}
	for _, record := range records {
		extClient := record.Value.Data()
		// TODO(nm-360): use the extclient's ID as the owner of its addresses.
		owner := IPOwner{Type: schema.IPOwnerExtClient, ID: extClient.ClientID}
		add(extClient.Address, owner)
		add(extClient.Address6, owner)
	}

	sort.Strings(duplicates)
	return owners, duplicates, nil
}

// hasUnusedIPAllocations reports whether the network has more attached
// addresses than its nodes and extclients use. Each node and extclient has an
// address from each of the network's ranges.
func (n *NetworkOrchestrator) hasUnusedIPAllocations(ctx context.Context, network *schema.Network) (bool, error) {
	nodes, err := (&schema.Node{}).Count(ctx, dbtypes.WithFilter("nodes_v1.network_id", network.ID))
	if err != nil {
		return false, err
	}
	extClients, err := (&schema.ExtClientRecord{TenantID: network.TenantID}).CountByNetwork(ctx, network.Name)
	if err != nil {
		return false, err
	}
	attached, err := (&schema.IPAllocation{TenantID: network.TenantID, NetworkID: network.ID}).CountAttached(ctx)
	if err != nil {
		return false, err
	}

	families := 0
	if network.AddressRange != "" {
		families++
	}
	if network.AddressRange6 != "" {
		families++
	}
	return attached > (nodes+extClients)*families, nil
}

// ReconcileIPAllocations orphans the network's attached addresses that are not
// used by any node or extclient, once they are older than
// ipAllocationGracePeriod, so that they can be reallocated.
//
// An address is always allocated before the node or extclient using it is
// saved, so a node or extclient never uses an address that is not attached.
// But if saving the node or extclient fails (and the address is not released),
// the address is left attached without being used.
func (n *NetworkOrchestrator) ReconcileIPAllocations(ctx context.Context, network *schema.Network) error {
	unused, err := n.hasUnusedIPAllocations(ctx, network)
	if err != nil {
		return err
	}
	if !unused {
		return nil
	}

	owners, _, err := n.ListIPOwners(ctx, network)
	if err != nil {
		return err
	}
	allocations, err := (&schema.IPAllocation{TenantID: network.TenantID, NetworkID: network.ID}).ListByNetwork(ctx)
	if err != nil {
		return err
	}

	staleBefore := time.Now().Add(-ipAllocationGracePeriod)
	for _, allocation := range allocations {
		if allocation.State != schema.IPAttached || !allocation.UpdatedAt.Before(staleBefore) {
			continue
		}
		addr := allocation.Address()
		if _, used := owners[addr]; used {
			continue
		}
		if err := allocation.ReleaseStale(ctx, staleBefore); err != nil {
			slog.Error("failed to orphan unused IP address", "network", network.Name, "address", addr, "error", err)
		}
	}
	return nil
}

// StartIPAllocationHook starts reconciling the address allocations of all
// networks periodically.
func (n *NetworkOrchestrator) StartIPAllocationHook() {
	logic.HookManagerCh <- models.HookDetails{
		ID:       "ip-allocation-hook",
		Hook:     logic.WrapHook(n.reconcileAllIPAllocations),
		Interval: ipAllocationReconcileInterval,
	}
}

// reconcileAllIPAllocations reconciles the address allocations of the
// networks of all tenants.
func (n *NetworkOrchestrator) reconcileAllIPAllocations() error {
	ctx := db.WithContext(context.TODO())
	networks, err := (&schema.Network{}).ListAll(ctx)
	if err != nil {
		return err
	}
	for i := range networks {
		if err := n.ReconcileIPAllocations(ctx, &networks[i]); err != nil {
			slog.Error("failed to reconcile IP allocations", "network", networks[i].Name, "tenant", networks[i].TenantID, "error", err)
		}
	}
	return nil
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
