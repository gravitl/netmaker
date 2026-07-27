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
	TenantID  string `gorm:"default:''"`
	NetworkID string
	Value     datatypes.JSONType[DNSEntry]
}

const dnsRecordsTable = "dns"

func (*DNSRecord) TableName() string { return dnsRecordsTable }

func (r *DNSRecord) Get(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	logicalKey := r.Key
	r.Key = TenantScopedKey(tenantID, logicalKey)
	err := db.FromContext(ctx).Where("key = ?", r.Key).First(r).Error
	if err != nil {
		r.Key = logicalKey
		return err
	}
	r.Key = logicalKey
	return nil
}

func (r *DNSRecord) Upsert(ctx context.Context) error {
	r.TenantID = scope.ID(ctx)
	rec := *r
	rec.Key = TenantScopedKey(r.TenantID, r.Key)
	return db.FromContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&rec).Error
}

func (r *DNSRecord) Delete(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	return db.FromContext(ctx).Where("key = ?", TenantScopedKey(tenantID, r.Key)).Delete(&DNSRecord{}).Error
}

func (*DNSRecord) List(ctx context.Context) ([]DNSRecord, error) {
	var records []DNSRecord
	query := db.FromContext(ctx).Model(&DNSRecord{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", dnsRecordsTable), tenantID)(query)
	}
	err := query.Find(&records).Error
	for i := range records {
		records[i].Key = StripTenantKey(records[i].TenantID, records[i].Key)
	}
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
