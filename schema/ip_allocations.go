package schema

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/gorm/clause"
)

const ipAllocationsTable = "ip_allocations_v1"

// IPAllocationState is the state of an allocated address.
type IPAllocationState string

const (
	// IPAttached means the address is in use by a node or extclient.
	IPAttached IPAllocationState = "attached"
	// IPOrphaned means the address was released and can be reallocated.
	IPOrphaned IPAllocationState = "orphaned"
)

// IPOwnerType is the kind of peer an address is allocated to.
type IPOwnerType string

const (
	IPOwnerNode      IPOwnerType = "node"
	IPOwnerExtClient IPOwnerType = "extclient"
)

// IPAllocation is an address that has been handed out from a network's range.
// Released addresses are kept as orphaned, so that they can be reallocated
// without searching the range for free addresses.
type IPAllocation struct {
	ID        string   `gorm:"primaryKey" json:"id"`
	TenantID  string   `gorm:"uniqueIndex:udx_ip_allocation_tenant_network_address,priority:1;index:idx_ip_allocation_orphaned,priority:1" json:"tenant_id"`
	NetworkID string   `gorm:"uniqueIndex:udx_ip_allocation_tenant_network_address,priority:2;index:idx_ip_allocation_orphaned,priority:2" json:"network_id"`
	Network   *Network `gorm:"foreignKey:NetworkID;constraint:OnDelete:CASCADE" json:"network,omitempty"`
	// RawAddress is the address in its 4 or 16 byte form, so that the
	// addresses of a family are ordered numerically. Use Address and
	// SetAddress to access it.
	RawAddress []byte            `gorm:"column:address;uniqueIndex:udx_ip_allocation_tenant_network_address,priority:3;index:idx_ip_allocation_orphaned,priority:5" json:"-"`
	Family     IPFamily          `gorm:"index:idx_ip_allocation_orphaned,priority:3" json:"family"`
	State      IPAllocationState `gorm:"index:idx_ip_allocation_orphaned,priority:4" json:"state"`
	OwnerType  IPOwnerType       `json:"owner_type"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

func (a *IPAllocation) TableName() string {
	return ipAllocationsTable
}

// Address returns the allocated address, or the zero address if it is not
// set or malformed.
func (a *IPAllocation) Address() netip.Addr {
	addr, ok := netip.AddrFromSlice(a.RawAddress)
	if !ok {
		return netip.Addr{}
	}
	return addr
}

// SetAddress sets the allocated address.
func (a *IPAllocation) SetAddress(addr netip.Addr) {
	a.RawAddress = addr.Unmap().AsSlice()
}

// validate sets the ID of a new allocation, and checks its address.
func (a *IPAllocation) validate() error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if !a.Address().IsValid() {
		return fmt.Errorf("invalid IP allocation address %v", a.RawAddress)
	}
	return nil
}

func (a *IPAllocation) Create(ctx context.Context) error {
	if err := a.validate(); err != nil {
		return err
	}
	return db.FromContext(ctx).Create(a).Error
}

// CreateAll inserts the allocations, skipping the ones that already exist.
func (a *IPAllocation) CreateAll(ctx context.Context, allocations []IPAllocation) error {
	if len(allocations) == 0 {
		return nil
	}
	for i := range allocations {
		if err := allocations[i].validate(); err != nil {
			return err
		}
	}
	return db.FromContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "network_id"}, {Name: "address"}},
		DoNothing: true,
	}).CreateInBatches(allocations, 500).Error
}

func (a *IPAllocation) Get(ctx context.Context) error {
	return db.FromContext(ctx).
		Where("tenant_id = ? AND network_id = ? AND address = ?", a.TenantID, a.NetworkID, a.RawAddress).
		First(a).
		Error
}

// Exists reports whether the address has been allocated, in any state.
func (a *IPAllocation) Exists(ctx context.Context) (bool, error) {
	var count int64
	err := db.FromContext(ctx).Model(&IPAllocation{}).
		Where("tenant_id = ? AND network_id = ? AND address = ?", a.TenantID, a.NetworkID, a.RawAddress).
		Count(&count).
		Error
	return count > 0, err
}

// GetFirstOrphaned fetches the orphaned address of the network and family
// that is nearest to the start of the range, or to its end if fromEnd is set.
func (a *IPAllocation) GetFirstOrphaned(ctx context.Context, fromEnd bool) error {
	order := "address ASC"
	if fromEnd {
		order = "address DESC"
	}
	return db.FromContext(ctx).
		Where("tenant_id = ? AND network_id = ? AND family = ? AND state = ?", a.TenantID, a.NetworkID, a.Family, IPOrphaned).
		Order(order).
		First(a).
		Error
}

// Attach marks the address as in use by the given kind of peer.
func (a *IPAllocation) Attach(ctx context.Context, ownerType IPOwnerType) error {
	a.State = IPAttached
	a.OwnerType = ownerType
	a.UpdatedAt = time.Now()
	return db.FromContext(ctx).Model(&IPAllocation{}).
		Where("tenant_id = ? AND network_id = ? AND address = ?", a.TenantID, a.NetworkID, a.RawAddress).
		Updates(map[string]any{
			"state":      a.State,
			"owner_type": a.OwnerType,
			"updated_at": a.UpdatedAt,
		}).
		Error
}

// Release marks the address as orphaned, so that it can be reallocated.
func (a *IPAllocation) Release(ctx context.Context) error {
	return db.FromContext(ctx).Model(&IPAllocation{}).
		Where("tenant_id = ? AND network_id = ? AND address = ? AND state = ?", a.TenantID, a.NetworkID, a.RawAddress, IPAttached).
		Updates(map[string]any{
			"state":      IPOrphaned,
			"updated_at": time.Now(),
		}).
		Error
}

// ReleaseStale is like Release, but only releases the address if it has not
// been updated since the given time, e.g. reattached concurrently.
func (a *IPAllocation) ReleaseStale(ctx context.Context, updatedBefore time.Time) error {
	return db.FromContext(ctx).Model(&IPAllocation{}).
		Where("tenant_id = ? AND network_id = ? AND address = ? AND state = ? AND updated_at < ?", a.TenantID, a.NetworkID, a.RawAddress, IPAttached, updatedBefore).
		Updates(map[string]any{
			"state":      IPOrphaned,
			"updated_at": time.Now(),
		}).
		Error
}

// ListByNetwork lists the allocations of the network, in any state.
func (a *IPAllocation) ListByNetwork(ctx context.Context) ([]IPAllocation, error) {
	var allocations []IPAllocation
	err := db.FromContext(ctx).
		Where("tenant_id = ? AND network_id = ?", a.TenantID, a.NetworkID).
		Find(&allocations).
		Error
	return allocations, err
}

func (a *IPAllocation) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Where(fmt.Sprintf("%s.tenant_id = ?", ipAllocationsTable), tenantID).Delete(&IPAllocation{}).Error
	}
	return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s", ipAllocationsTable)).Error
}
