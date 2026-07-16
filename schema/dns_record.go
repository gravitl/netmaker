package schema

import (
	"context"
	"fmt"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

type DNSEntryType string

const (
	DNSEntryType_Node   DNSEntryType = "node"
	DNSEntryType_Custom DNSEntryType = "custom"
)

// DNSEntry - a DNS entry represented as struct
type DNSEntry struct {
	Type     DNSEntryType `json:"type"`
	Address  string       `json:"address" validate:"omitempty,ip"`
	Address6 string       `json:"address6" validate:"omitempty,ip"`
	Name     string       `json:"name" validate:"required,name_unique,min=1,max=192,whitespace"`
	Network  string       `json:"network" validate:"network_exists"`
}

type DNSRecord struct {
	Key       string `gorm:"primaryKey"`
	TenantID  string `gorm:"primaryKey;default:''"`
	NetworkID string
	Value     datatypes.JSONType[DNSEntry]
}

const dnsRecordsTable = "dns"

func (*DNSRecord) TableName() string { return dnsRecordsTable }

func (r *DNSRecord) Get(ctx context.Context) error {
	return db.FromContext(ctx).Where(fmt.Sprintf("key = ? AND %s.tenant_id = ?", dnsRecordsTable), r.Key, scope.ID(ctx)).First(r).Error
}

func (r *DNSRecord) Upsert(ctx context.Context) error {
	r.TenantID = scope.ID(ctx)
	return db.FromContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}, {Name: "tenant_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(r).Error
}

func (r *DNSRecord) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Where(fmt.Sprintf("key = ? AND %s.tenant_id = ?", dnsRecordsTable), r.Key, scope.ID(ctx)).Delete(r).Error
}

func (*DNSRecord) List(ctx context.Context) ([]DNSRecord, error) {
	var records []DNSRecord
	query := db.FromContext(ctx).Model(&DNSRecord{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", dnsRecordsTable), tenantID)(query)
	}
	err := query.Find(&records).Error
	return records, err
}

func (*DNSRecord) Count(ctx context.Context) (int, error) {
	var count int64
	query := db.FromContext(ctx).Model(&DNSRecord{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", dnsRecordsTable), tenantID)(query)
	}
	err := query.Count(&count).Error
	return int(count), err
}
