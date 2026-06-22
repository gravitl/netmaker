package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type DNSEntryType string

const (
	DNSEntryType_Node   = "node"
	DNSEntryType_Custom = "custom"
)

// DNSEntry - a DNS entry represented as struct
type DNSEntry struct {
	Type     DNSEntryType `json:"type"`
	Address  string       `json:"address" validate:"omitempty,ip"`
	Address6 string       `json:"address6" validate:"omitempty,ip"`
	Name     string       `json:"name" validate:"required,name_unique,min=1,max=192,whitespace"`
	Network  string       `json:"network" validate:"network_exists"`
}

// DNS is the GORM model for the legacy "dns" key-value table, extended with
// tenant_id and network_id columns for multi-tenancy.
type DNS struct {
	Key       string `gorm:"primaryKey"`
	TenantID  string `gorm:"default:''"`
	NetworkID string
	Value     datatypes.JSONType[DNSEntry]
}

func (*DNS) TableName() string { return "dns" }

func (d *DNS) Create(ctx context.Context) error {
	return db.FromContext(ctx).Create(d).Error
}

func (d *DNS) Get(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", d.Key).First(d).Error
}

func (d *DNS) ListAll(ctx context.Context) ([]DNS, error) {
	var entries []DNS
	err := db.FromContext(ctx).Find(&entries).Error
	return entries, err
}

func (d *DNS) ListByNetwork(ctx context.Context) ([]DNS, error) {
	var entries []DNS
	err := db.FromContext(ctx).Where("network_id = ?", d.NetworkID).Find(&entries).Error
	return entries, err
}

func (d *DNS) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", d.Key).Delete(&DNS{}).Error
}

func (d *DNS) DeleteByNetwork(ctx context.Context) error {
	return db.FromContext(ctx).Where("network_id = ?", d.NetworkID).Delete(&DNS{}).Error
}
