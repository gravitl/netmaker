package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
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
// Nodes and extclients are allocated from the start of the network's range.
// Released (orphaned) addresses are reused first, the lowest one; otherwise
// addresses are allocated upwards by the pool's cursor.
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
	addr, err := n.ParseAddr(ip)
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
	err = n.claimIP(ctx, network, addr, family, ownerType, ownerID)
	if errors.Is(err, ErrIPPoolNotFound) {
		if ensureErr := n.ensureIPPool(ctx, network, family); ensureErr != nil {
			return ensureErr
		}
		err = n.claimIP(ctx, network, addr, family, ownerType, ownerID)
	}
	return err
}

func (n *NetworkOrchestrator) claimIP(ctx context.Context, network *schema.Network, addr netip.Addr, family schema.IPFamily, ownerType schema.IPOwnerType, ownerID string) error {
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
	addr, err := n.ParseAddr(ip)
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

	addr, err := n.allocateOnce(ctx, network, family, ownerType, ownerID)
	if errors.Is(err, ErrIPPoolNotFound) {
		if ensureErr := n.ensureIPPool(ctx, network, family); ensureErr != nil {
			return nil, ensureErr
		}
		addr, err = n.allocateOnce(ctx, network, family, ownerType, ownerID)
	}
	if err != nil {
		return nil, err
	}
	return addr.AsSlice(), nil
}

func (n *NetworkOrchestrator) allocateOnce(ctx context.Context, network *schema.Network, family schema.IPFamily, ownerType schema.IPOwnerType, ownerID string) (netip.Addr, error) {
	var addr netip.Addr
	err := db.FromContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	return addr, err
}

// ensureIPPool creates the network's address pool when it is missing, and
// records addresses already used by nodes and extclients so they are not
// allocated again. Networks created before address pools, or upgraded after
// the v1.8.0 migration already completed, have no pool row.
func (n *NetworkOrchestrator) ensureIPPool(ctx context.Context, network *schema.Network, family schema.IPFamily) error {
	owners, duplicates, err := n.ListIPOwners(ctx, network)
	if err != nil {
		return err
	}
	if len(duplicates) > 0 {
		return fmt.Errorf(
			"found IP addresses used by more than one node or extclient:\n%s",
			strings.Join(duplicates, "\n"),
		)
	}

	first, last, err := network.UsableIPRange(family)
	if err != nil {
		return err
	}

	allocated := make(map[netip.Addr]struct{})
	var allocations []schema.IPAllocation
	for addr, owner := range owners {
		if addr.Is4() != (family == schema.IPv4) {
			continue
		}
		if addr.Less(first) || last.Less(addr) {
			slog.Warn("skipping IP allocation outside the network range", "address", addr, "owner", owner, "network", network.Name)
			continue
		}
		allocated[addr] = struct{}{}
		allocation := schema.IPAllocation{
			TenantID:  network.TenantID,
			NetworkID: network.ID,
			Family:    family,
			State:     schema.IPAttached,
			OwnerType: owner.Type,
			OwnerID:   owner.ID,
		}
		allocation.SetAddress(addr)
		allocations = append(allocations, allocation)
	}

	pool := &schema.IPPool{
		TenantID:  network.TenantID,
		NetworkID: network.ID,
		Family:    family,
		Cursor:    ipPoolCursor(allocated, first, last),
	}
	if err := pool.Create(ctx); err != nil {
		return err
	}
	return (&schema.IPAllocation{}).CreateAll(ctx, allocations)
}

// ipPoolCursor places the cursor at the last address of the allocated block
// at the start of the range. Every address up to the cursor must have an
// allocation. Addresses allocated beyond the cursor are skipped on lookup.
func ipPoolCursor(allocated map[netip.Addr]struct{}, first, last netip.Addr) (cursor string) {
	for addr := first; addr.IsValid() && !last.Less(addr); addr = addr.Next() {
		if _, ok := allocated[addr]; !ok {
			break
		}
		cursor = addr.String()
	}
	return cursor
}

// ipAllocationWindow is how many allocated addresses allocateFromCursor
// fetches at a time, while looking for the next address that isn't allocated.
const ipAllocationWindow = 256

// allocateFromCursor allocates the next address after the pool's cursor,
// skipping addresses that are already allocated (e.g. claimed custom
// addresses). Caller must hold the pool lock.
func (n *NetworkOrchestrator) allocateFromCursor(ctx context.Context, network *schema.Network, pool *schema.IPPool, ownerType schema.IPOwnerType, ownerID string) (netip.Addr, error) {
	first, last, err := network.UsableIPRange(pool.Family)
	if err != nil {
		return netip.Addr{}, err
	}
	addr := first
	if pool.Cursor != "" {
		cursor, err := n.parseCursor(pool.Cursor)
		if err != nil {
			return netip.Addr{}, err
		}
		if !cursor.Less(first) {
			addr = cursor.Next()
		}
	}

	// withinRange reports whether addr has not gone past the end of the range.
	withinRange := func(addr netip.Addr) bool {
		return addr.IsValid() && !last.Less(addr)
	}

	var skipped netip.Addr
	for withinRange(addr) {
		// fetch the allocated addresses from addr onwards, a window at a time.
		allocated, err := (&schema.IPAllocation{
			TenantID:  pool.TenantID,
			NetworkID: pool.NetworkID,
			Family:    pool.Family,
		}).ListAddressesBetween(ctx, addr, last, ipAllocationWindow)
		if err != nil {
			return netip.Addr{}, err
		}

		// skip the run of allocated addresses starting at addr.
		run := 0
		for ; run < ipAllocationWindow; run++ {
			if _, ok := allocated[addr]; !ok {
				break
			}
			skipped = addr
			addr = addr.Next()
		}
		if run == ipAllocationWindow {
			// the whole window is allocated, look at the next one.
			continue
		}
		if !withinRange(addr) {
			break
		}

		// addr is not allocated.
		allocation := &schema.IPAllocation{
			TenantID:  pool.TenantID,
			NetworkID: pool.NetworkID,
			Family:    pool.Family,
			State:     schema.IPAttached,
			OwnerType: ownerType,
			OwnerID:   ownerID,
		}
		allocation.SetAddress(addr)
		if err := allocation.Create(ctx); err != nil {
			return netip.Addr{}, err
		}
		pool.Cursor = addr.String()
		return addr, pool.UpdateCursor(ctx)
	}

	// don't walk over the skipped addresses again on the next allocation.
	if skipped.IsValid() {
		pool.Cursor = skipped.String()
		if err := pool.UpdateCursor(ctx); err != nil {
			return netip.Addr{}, err
		}
	}
	return netip.Addr{}, ErrIPPoolExhausted
}

// allocateOrphaned reallocates the lowest orphaned address.
// Caller must hold the pool lock.
func (n *NetworkOrchestrator) allocateOrphaned(ctx context.Context, pool *schema.IPPool, ownerType schema.IPOwnerType, ownerID string) (netip.Addr, error) {
	allocation := &schema.IPAllocation{
		TenantID:  pool.TenantID,
		NetworkID: pool.NetworkID,
		Family:    pool.Family,
	}
	if err := allocation.GetFirstOrphaned(ctx); err != nil {
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
		addr, err := n.ParseAddr(address)
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

	extClients, err := (&schema.Extclient{NetworkID: network.ID}).ListByNetwork(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, extClient := range extClients {
		owner := IPOwner{Type: schema.IPOwnerExtClient, ID: extClient.ID}
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
	extClients, err := (&schema.Extclient{NetworkID: network.ID}).CountByNetwork(ctx)
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

// SyncIPAllocations creates a missing address pool for every network and
// records addresses already used by nodes and extclients. An existing pool
// cursor is left where it is. A network with duplicate addresses is logged
// and skipped.
func (n *NetworkOrchestrator) SyncIPAllocations(ctx context.Context) {
	networks, err := (&schema.Network{}).ListAll(db.WithContext(context.Background()))
	if err != nil {
		slog.Error("failed to list networks for IP allocation sync", "error", err)
		return
	}
	for i := range networks {
		network := &networks[i]
		netCtx := scope.WithContext(db.WithContext(ctx), scope.TenantScope, network.TenantID)
		if err := n.syncNetworkIPAllocations(netCtx, network); err != nil {
			slog.Error("failed to sync IP allocations", "network", network.Name, "tenant", network.TenantID, "error", err)
		}
	}
}

// syncNetworkIPAllocations ensures the network's pools exist and that every
// in-use address has an allocation. The cursor of a pool that already exists
// is not changed.
func (n *NetworkOrchestrator) syncNetworkIPAllocations(ctx context.Context, network *schema.Network) error {
	owners, duplicates, err := n.ListIPOwners(ctx, network)
	if err != nil {
		return err
	}
	if len(duplicates) > 0 {
		return fmt.Errorf("duplicate IP addresses:\n%s", strings.Join(duplicates, "\n"))
	}

	for _, family := range []schema.IPFamily{schema.IPv4, schema.IPv6} {
		if (family == schema.IPv4 && network.AddressRange == "") || (family == schema.IPv6 && network.AddressRange6 == "") {
			continue
		}
		pool := &schema.IPPool{
			TenantID:  network.TenantID,
			NetworkID: network.ID,
			Family:    family,
		}
		err := pool.Get(ctx)
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := n.ensureIPPool(ctx, network, family); err != nil {
				return err
			}
			slog.Info("created missing IP pool", "network", network.Name, "tenant", network.TenantID, "family", family)
		case err != nil:
			return err
		default:
			if err := n.recordUsedAddresses(ctx, network, family, owners); err != nil {
				return err
			}
		}
	}
	return nil
}

// recordUsedAddresses inserts an allocation for each in-use address of the
// family that has none, and reattaches an orphaned allocation that a node or
// extclient still uses. It does not move the pool cursor.
func (n *NetworkOrchestrator) recordUsedAddresses(ctx context.Context, network *schema.Network, family schema.IPFamily, owners map[netip.Addr]IPOwner) error {
	first, last, err := network.UsableIPRange(family)
	if err != nil {
		return err
	}
	existing, err := (&schema.IPAllocation{TenantID: network.TenantID, NetworkID: network.ID}).ListByNetwork(ctx)
	if err != nil {
		return err
	}
	byAddr := make(map[netip.Addr]schema.IPAllocation, len(existing))
	for _, allocation := range existing {
		if allocation.Family != family {
			continue
		}
		addr := allocation.Address()
		if addr.IsValid() {
			byAddr[addr] = allocation
		}
	}

	var missing []schema.IPAllocation
	for addr, owner := range owners {
		if addr.Is4() != (family == schema.IPv4) {
			continue
		}
		if addr.Less(first) || last.Less(addr) {
			continue
		}
		current, ok := byAddr[addr]
		if !ok {
			allocation := schema.IPAllocation{
				TenantID:  network.TenantID,
				NetworkID: network.ID,
				Family:    family,
				State:     schema.IPAttached,
				OwnerType: owner.Type,
				OwnerID:   owner.ID,
			}
			allocation.SetAddress(addr)
			missing = append(missing, allocation)
			continue
		}
		if current.State == schema.IPAttached && (current.OwnerType != owner.Type || current.OwnerID != owner.ID) {
			slog.Warn("IP address in use does not match its allocation", "network", network.Name, "address", addr, "owner", owner, "allocated_to", current.OwnerType, "allocated_id", current.OwnerID)
			continue
		}
		if current.State == schema.IPOrphaned {
			current.OwnerType = owner.Type
			current.OwnerID = owner.ID
			if err := current.Attach(ctx); err != nil {
				return err
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	slog.Info("recorded IP addresses already in use", "network", network.Name, "family", family, "count", len(missing))
	return (&schema.IPAllocation{}).CreateAll(ctx, missing)
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

// ParseAddr parses an address given either as a plain IP or in CIDR notation.
func (n *NetworkOrchestrator) ParseAddr(address string) (netip.Addr, error) {
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
