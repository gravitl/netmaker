package schema

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DNSEntryType string

const (
	DNSEntryType_Node   DNSEntryType = "node"
	DNSEntryType_Custom DNSEntryType = "custom"
)

const dnsEntriesTable = "dns_v1"

var ErrDNSEntryIdentifiersNotProvided = errors.New("dns entry identifiers not provided")

// DNSEntry is a custom dns entry of a network.
type DNSEntry struct {
	ID       string `gorm:"primaryKey" json:"id"`
	TenantID string `gorm:"default:'';uniqueIndex:udx_dns_entry_tenant_network_name" json:"tenant_id"`
	// NetworkID has no foreign key constraint, dns entries are deleted
	// explicitly on network deletion.
	NetworkID string    `gorm:"not null;index;uniqueIndex:udx_dns_entry_tenant_network_name" json:"network_id"`
	Network   *Network  `gorm:"foreignKey:NetworkID;constraint:-" json:"network,omitempty"`
	Name      string    `gorm:"not null;uniqueIndex:udx_dns_entry_tenant_network_name" json:"name"`
	Address   string    `json:"address"`
	Address6  string    `json:"address6"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (e *DNSEntry) TableName() string {
	return dnsEntriesTable
}

// baseIdentifierQuery scopes the query to the dns entry identified by its ID, or
// by its name in the network identified by NetworkID.
func (e *DNSEntry) baseIdentifierQuery(ctx context.Context) (*gorm.DB, error) {
	query := db.FromContext(ctx).Model(&DNSEntry{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = query.Where(fmt.Sprintf("%s.tenant_id = ?", dnsEntriesTable), tenantID)
	}

	if e.ID != "" {
		return query.Where(fmt.Sprintf("%s.id = ?", dnsEntriesTable), e.ID), nil
	}

	if e.Name == "" || e.NetworkID == "" {
		return nil, ErrDNSEntryIdentifiersNotProvided
	}

	return query.Where(
		fmt.Sprintf("%s.network_id = ? AND %s.name = ?", dnsEntriesTable, dnsEntriesTable),
		e.NetworkID, e.Name,
	), nil
}

// networkQuery scopes the query to the dns entries of the network identified by
// NetworkID, or by Network.Name within the tenant in the context. The latter
// joins the network, and populates the Network of the results.
func (e *DNSEntry) networkQuery(ctx context.Context) (*gorm.DB, error) {
	tenantID := scope.ID(ctx)
	query := db.FromContext(ctx).Model(&DNSEntry{})
	if tenantID != "" {
		query = query.Where(fmt.Sprintf("%s.tenant_id = ?", dnsEntriesTable), tenantID)
	}

	if e.NetworkID != "" {
		return query.Where(fmt.Sprintf("%s.network_id = ?", dnsEntriesTable), e.NetworkID), nil
	}

	// network names are only unique within a tenant.
	if e.Network == nil || e.Network.Name == "" || tenantID == "" {
		return nil, ErrDNSEntryIdentifiersNotProvided
	}

	condition := db.FromContext(ctx).Session(&gorm.Session{NewDB: true}).Where(&Network{Name: e.Network.Name})
	return query.InnerJoins("Network", condition), nil
}

func (e *DNSEntry) Create(ctx context.Context) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}

	return db.FromContext(ctx).Model(&DNSEntry{}).Omit(clause.Associations).Create(e).Error
}

// Get fetches the dns entry by its ID, by its name in the network identified by
// NetworkID, or by its name in the network identified by Network.Name.
func (e *DNSEntry) Get(ctx context.Context) error {
	var query *gorm.DB
	var err error
	if e.ID == "" && e.NetworkID == "" {
		if e.Name == "" {
			return ErrDNSEntryIdentifiersNotProvided
		}

		query, err = e.networkQuery(ctx)
		if err == nil {
			query = query.Where(fmt.Sprintf("%s.name = ?", dnsEntriesTable), e.Name)
		}
	} else {
		query, err = e.baseIdentifierQuery(ctx)
	}
	if err != nil {
		return err
	}

	var entry DNSEntry
	err = query.First(&entry).Error
	if err != nil {
		return err
	}

	*e = entry
	return nil
}

// Update overwrites all the fields of the dns entry identified by its ID, except
// the tenant and the creation time. It returns gorm.ErrRecordNotFound if the
// dns entry does not exist.
func (e *DNSEntry) Update(ctx context.Context) error {
	if e.ID == "" {
		return ErrDNSEntryIdentifiersNotProvided
	}

	result := db.FromContext(ctx).Model(&DNSEntry{}).
		Where(fmt.Sprintf("%s.id = ?", dnsEntriesTable), e.ID).
		Select("*").
		Omit("id", "tenant_id", "created_at", clause.Associations).
		Updates(e)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete deletes the dns entry. It returns gorm.ErrRecordNotFound if the dns entry does
// not exist.
func (e *DNSEntry) Delete(ctx context.Context) error {
	query, err := e.baseIdentifierQuery(ctx)
	if err != nil {
		return err
	}

	result := query.Delete(&DNSEntry{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListByNetwork lists the dns entries of the network identified by NetworkID, or by
// Network.Name.
func (e *DNSEntry) ListByNetwork(ctx context.Context, options ...dbtypes.Option) ([]DNSEntry, error) {
	query, err := e.networkQuery(ctx)
	if err != nil {
		return nil, err
	}

	for _, opt := range options {
		query = opt(query)
	}

	var entries []DNSEntry
	err = query.Find(&entries).Error
	return entries, err
}

// DeleteByNetwork deletes the dns entries of the network identified by NetworkID.
func (e *DNSEntry) DeleteByNetwork(ctx context.Context) error {
	if e.NetworkID == "" {
		return ErrDNSEntryIdentifiersNotProvided
	}

	query := db.FromContext(ctx).Where(fmt.Sprintf("%s.network_id = ?", dnsEntriesTable), e.NetworkID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = query.Where(fmt.Sprintf("%s.tenant_id = ?", dnsEntriesTable), tenantID)
	}

	return query.Delete(&DNSEntry{}).Error
}

func (e *DNSEntry) ListAll(ctx context.Context, options ...dbtypes.Option) ([]DNSEntry, error) {
	query := db.FromContext(ctx).Model(&DNSEntry{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", dnsEntriesTable), tenantID))
	}

	for _, opt := range options {
		query = opt(query)
	}

	var entries []DNSEntry
	err := query.Find(&entries).Error
	return entries, err
}

func (e *DNSEntry) Count(ctx context.Context, options ...dbtypes.Option) (int, error) {
	query := db.FromContext(ctx).Model(&DNSEntry{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", dnsEntriesTable), tenantID))
	}

	for _, opt := range options {
		query = opt(query)
	}

	var count int64
	err := query.Count(&count).Error
	return int(count), err
}

func (e *DNSEntry) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s WHERE tenant_id = ?", dnsEntriesTable), tenantID).Error
	}

	return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s", dnsEntriesTable)).Error
}
