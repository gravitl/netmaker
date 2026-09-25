package schema

import (
	"context"
	"fmt"
	"time"

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
	TenantID  string            `gorm:"primaryKey" json:"tenant_id"`
	NetworkID string            `gorm:"primaryKey" json:"network_id"`
	Network   *Network          `gorm:"foreignKey:NetworkID;constraint:OnDelete:CASCADE" json:"network,omitempty"`
	Address   string            `gorm:"primaryKey" json:"address"`
	Family    IPFamily          `gorm:"index:idx_ip_allocations_state,priority:1" json:"family"`
	State     IPAllocationState `gorm:"index:idx_ip_allocations_state,priority:2" json:"state"`
	OwnerType IPOwnerType       `json:"owner_type"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `gorm:"index:idx_ip_allocations_state,priority:3" json:"updated_at"`
}

func (a *IPAllocation) TableName() string {
	return ipAllocationsTable
}

func (a *IPAllocation) Create(ctx context.Context) error {
	return db.FromContext(ctx).Create(a).Error
}

// CreateAll inserts the allocations, skipping the ones that already exist.
func (a *IPAllocation) CreateAll(ctx context.Context, allocations []IPAllocation) error {
	if len(allocations) == 0 {
		return nil
	}
	return db.FromContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(allocations, 500).Error
}

func (a *IPAllocation) Get(ctx context.Context) error {
	return db.FromContext(ctx).
		Where("tenant_id = ? AND network_id = ? AND address = ?", a.TenantID, a.NetworkID, a.Address).
		First(a).
		Error
}

// Exists reports whether the address has been allocated, in any state.
func (a *IPAllocation) Exists(ctx context.Context) (bool, error) {
	var count int64
	err := db.FromContext(ctx).Model(&IPAllocation{}).
		Where("tenant_id = ? AND network_id = ? AND address = ?", a.TenantID, a.NetworkID, a.Address).
		Count(&count).
		Error
	return count > 0, err
}

// GetOldestOrphaned fetches the address of the network and family that has
// been orphaned the longest, so that released addresses are reused as late
// as possible.
func (a *IPAllocation) GetOldestOrphaned(ctx context.Context) error {
	return db.FromContext(ctx).
		Where("tenant_id = ? AND network_id = ? AND family = ? AND state = ?", a.TenantID, a.NetworkID, a.Family, IPOrphaned).
		Order("updated_at ASC, address ASC").
		First(a).
		Error
}

// Attach marks the address as in use by the given kind of peer.
func (a *IPAllocation) Attach(ctx context.Context, ownerType IPOwnerType) error {
	a.State = IPAttached
	a.OwnerType = ownerType
	a.UpdatedAt = time.Now()
	return db.FromContext(ctx).Model(&IPAllocation{}).
		Where("tenant_id = ? AND network_id = ? AND address = ?", a.TenantID, a.NetworkID, a.Address).
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
		Where("tenant_id = ? AND network_id = ? AND address = ? AND state = ?", a.TenantID, a.NetworkID, a.Address, IPAttached).
		Updates(map[string]any{
			"state":      IPOrphaned,
			"updated_at": time.Now(),
		}).
		Error
}

// ReleaseByNetworkName is like Release, but identifies the network by its
// name within the allocation's tenant.
func (a *IPAllocation) ReleaseByNetworkName(ctx context.Context, networkName string) error {
	networkIDs := db.FromContext(ctx).Model(&Network{}).
		Select("id").
		Where("name = ? AND tenant_id = ?", networkName, a.TenantID)
	return db.FromContext(ctx).Model(&IPAllocation{}).
		Where("tenant_id = ? AND network_id IN (?) AND address = ? AND state = ?", a.TenantID, networkIDs, a.Address, IPAttached).
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
		Where("tenant_id = ? AND network_id = ? AND address = ? AND state = ? AND updated_at < ?", a.TenantID, a.NetworkID, a.Address, IPAttached, updatedBefore).
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
