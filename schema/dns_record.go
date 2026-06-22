package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
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
	TenantID  string `gorm:"default:'';index"`
	NetworkID string
	Value     datatypes.JSONType[DNSEntry]
}

func (*DNSRecord) TableName() string { return "dns" }

func (r *DNSRecord) Get(ctx context.Context) error {
	return db.FromContext(ctx).First(r).Error
}

func (r *DNSRecord) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(r).Error
}

func (r *DNSRecord) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Delete(r).Error
}

func (*DNSRecord) List(ctx context.Context) ([]DNSRecord, error) {
	var records []DNSRecord
	err := db.FromContext(ctx).Find(&records).Error
	return records, err
}

func (*DNSRecord) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&DNSRecord{}).Count(&count).Error
	return int(count), err
}
