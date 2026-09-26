package migrate

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/schema"
)

func migrateV1_8_0(ctx context.Context) error {
	return migrateIPAllocations(ctx)
}

// migrateIPAllocations backfills the address pools and allocations of the
// existing networks from the addresses of their nodes and extclients.
//
// It fails if an address is used more than once in a network, which has to be
// resolved before upgrading.
func migrateIPAllocations(ctx context.Context) error {
	networks, err := (&schema.Network{}).ListAll(ctx)
	if err != nil {
		return err
	}

	networkOrch := orchestrator.GetRepository().NetworkOrchestrator()
	owners := make(map[string]map[netip.Addr]orchestrator.IPOwner, len(networks))
	var duplicates []string
	for i := range networks {
		network := &networks[i]
		networkOwners, networkDuplicates, err := networkOrch.ListIPOwners(ctx, network)
		if err != nil {
			return err
		}
		owners[network.ID] = networkOwners
		for _, duplicate := range networkDuplicates {
			duplicates = append(duplicates, fmt.Sprintf("network %s (tenant %s): %s", network.Name, network.TenantID, duplicate))
		}
	}
	if len(duplicates) > 0 {
		return fmt.Errorf(
			"found IP addresses used by more than one node or extclient, resolve them before upgrading:\n%s",
			strings.Join(duplicates, "\n"),
		)
	}

	for _, network := range networks {
		for _, family := range []schema.IPFamily{schema.IPv4, schema.IPv6} {
			if (family == schema.IPv4 && network.AddressRange == "") ||
				(family == schema.IPv6 && network.AddressRange6 == "") {
				continue
			}
			first, last, err := network.UsableIPRange(family)
			if err != nil {
				return err
			}

			allocated := make(map[netip.Addr]struct{})
			var allocations []schema.IPAllocation
			for addr, owner := range owners[network.ID] {
				if addr.Is4() != (family == schema.IPv4) {
					continue
				}
				if addr.Less(first) || last.Less(addr) {
					logger.Log(0, fmt.Sprintf("skipping IP allocation for %s of %s: out of the range of network %s", addr, owner, network.Name))
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
			}
			pool.NodeCursor, pool.ExtCursor = initialIPPoolCursors(allocated, first, last)
			if err := pool.Upsert(ctx); err != nil {
				return err
			}
			if err := (&schema.IPAllocation{}).CreateAll(ctx, allocations); err != nil {
				return err
			}
		}
	}
	return nil
}

// initialIPPoolCursors places the node cursor at the last address of the
// allocated block at the start of the range, and the extclient cursor at the
// first address of the allocated block at the end of the range, which is
// where the previous allocator would have continued from. The cursors skip
// allocated addresses beyond them anyway.
func initialIPPoolCursors(allocated map[netip.Addr]struct{}, first, last netip.Addr) (nodeCursor, extCursor string) {
	for addr := first; addr.IsValid() && !last.Less(addr); addr = addr.Next() {
		if _, ok := allocated[addr]; !ok {
			break
		}
		nodeCursor = addr.String()
	}
	for addr := last; addr.IsValid() && !addr.Less(first); addr = addr.Prev() {
		if _, ok := allocated[addr]; !ok {
			break
		}
		extCursor = addr.String()
	}
	return nodeCursor, extCursor
}
