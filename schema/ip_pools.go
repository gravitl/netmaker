package schema

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/c-robinson/iplib"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ipPoolsTable = "ip_pools_v1"

// IPFamily is the IP version of an address pool.
type IPFamily int

const (
	IPv4 IPFamily = 4
	IPv6 IPFamily = 6
)

// IPPool tracks the allocation cursors of a network's address range, for
// one IP family. Nodes are allocated upwards from the start of the range and
// extclients downwards from the end of it.
//
// The pool row is locked for the duration of an allocation, which serializes
// allocations on a network across all server replicas.
type IPPool struct {
	TenantID  string   `gorm:"primaryKey" json:"tenant_id"`
	NetworkID string   `gorm:"primaryKey" json:"network_id"`
	Network   *Network `gorm:"foreignKey:NetworkID;constraint:OnDelete:CASCADE" json:"network,omitempty"`
	Family    IPFamily `gorm:"primaryKey" json:"family"`
	// NodeCursor is the last address allocated to a node by the cursor, empty
	// if none has been allocated yet.
	NodeCursor string `json:"node_cursor"`
	// ExtCursor is the last address allocated to an extclient by the cursor,
	// empty if none has been allocated yet.
	ExtCursor string    `json:"ext_cursor"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (p *IPPool) TableName() string {
	return ipPoolsTable
}

// Create inserts the pool if it does not exist yet.
func (p *IPPool) Create(ctx context.Context) error {
	return db.FromContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(p).Error
}

// Upsert inserts the pool, or updates its cursors if it exists.
func (p *IPPool) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "network_id"}, {Name: "family"}},
		DoUpdates: clause.AssignmentColumns([]string{"node_cursor", "ext_cursor", "updated_at"}),
	}).Create(p).Error
}

// GetForUpdate fetches the pool and locks its row until the end of the
// transaction in ctx. SQLite does not support row locks, but it only allows
// one writing transaction at a time.
func (p *IPPool) GetForUpdate(ctx context.Context) error {
	return db.FromContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND network_id = ? AND family = ?", p.TenantID, p.NetworkID, p.Family).
		First(p).
		Error
}

// UpdateCursors saves the pool's cursors.
func (p *IPPool) UpdateCursors(ctx context.Context) error {
	return db.FromContext(ctx).Model(&IPPool{}).
		Where("tenant_id = ? AND network_id = ? AND family = ?", p.TenantID, p.NetworkID, p.Family).
		Updates(map[string]any{
			"node_cursor": p.NodeCursor,
			"ext_cursor":  p.ExtCursor,
			"updated_at":  time.Now(),
		}).
		Error
}

func (p *IPPool) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Where(fmt.Sprintf("%s.tenant_id = ?", ipPoolsTable), tenantID).Delete(&IPPool{}).Error
	}
	return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s", ipPoolsTable)).Error
}

// UsableIPRange returns the first and last address that can be allocated in
// the network's range of the given family.
func (n *Network) UsableIPRange(family IPFamily) (first, last netip.Addr, err error) {
	var firstIP, lastIP net.IP
	switch family {
	case IPv4:
		if n.AddressRange == "" {
			return first, last, fmt.Errorf("IPv4 not configured on network %s", n.Name)
		}
		net4 := iplib.Net4FromStr(n.AddressRange)
		if net4.IP() == nil {
			return first, last, fmt.Errorf("invalid IPv4 range %s on network %s", n.AddressRange, n.Name)
		}
		firstIP, lastIP = net4.FirstAddress(), net4.LastAddress()
	case IPv6:
		if n.AddressRange6 == "" {
			return first, last, fmt.Errorf("IPv6 not configured on network %s", n.Name)
		}
		net6 := iplib.Net6FromStr(n.AddressRange6)
		if net6.IP() == nil {
			return first, last, fmt.Errorf("invalid IPv6 range %s on network %s", n.AddressRange6, n.Name)
		}
		// the first and last addresses of the range are not allocated.
		if firstIP, err = net6.NextIP(net6.FirstAddress()); err != nil {
			return first, last, err
		}
		if lastIP, err = net6.PreviousIP(net6.LastAddress()); err != nil {
			return first, last, err
		}
	default:
		return first, last, fmt.Errorf("invalid IP family %d", family)
	}

	first, _ = netip.AddrFromSlice(firstIP)
	last, _ = netip.AddrFromSlice(lastIP)
	return first.Unmap(), last.Unmap(), nil
}

// createNetworkIPPools creates the address pools of a newly created network.
func createNetworkIPPools(tx *gorm.DB, network *Network) error {
	for family, addressRange := range map[IPFamily]string{IPv4: network.AddressRange, IPv6: network.AddressRange6} {
		if addressRange == "" {
			continue
		}
		pool := &IPPool{
			TenantID:  network.TenantID,
			NetworkID: network.ID,
			Family:    family,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(pool).Error; err != nil {
			return err
		}
	}
	return nil
}
