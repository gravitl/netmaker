package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

const (
	OldRemoteAccessTagName = "remote-access-gws"
	GwTagName              = "gateways"
)

type Tag struct {
	ID        TagID     `json:"id"`
	TagName   string    `json:"tag_name"`
	Network   NetworkID `json:"network"`
	ColorCode string    `json:"color_code"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func (t Tag) GetIDFromName() string {
	return fmt.Sprintf("%s.%s", t.Network, t.TagName)
}

// TagEntry is the GORM model for the legacy "tags" key-value table,
// extended with tenant_id and network_id columns for multi-tenancy.
type TagEntry struct {
	Key       string                  `gorm:"primaryKey;column:key"`
	TenantID  string                  `gorm:"column:tenant_id;default:''"`
	NetworkID string                  `gorm:"column:network_id"`
	Value     datatypes.JSONType[Tag] `gorm:"column:value"`
}

func (*TagEntry) TableName() string { return "tags" }

func (e *TagEntry) Create(ctx context.Context) error {
	return db.FromContext(ctx).Create(e).Error
}

// Save does an upsert — insert or replace on primary key conflict.
func (e *TagEntry) Save(ctx context.Context) error {
	return db.FromContext(ctx).Save(e).Error
}

func (e *TagEntry) Get(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", e.Key).First(e).Error
}

func (e *TagEntry) ListAll(ctx context.Context) ([]TagEntry, error) {
	var entries []TagEntry
	err := db.FromContext(ctx).Find(&entries).Error
	return entries, err
}

func (e *TagEntry) ListByNetwork(ctx context.Context) ([]TagEntry, error) {
	var entries []TagEntry
	err := db.FromContext(ctx).Where("network_id = ?", e.NetworkID).Find(&entries).Error
	return entries, err
}

func (e *TagEntry) Count(ctx context.Context) (int64, error) {
	var count int64
	err := db.FromContext(ctx).Model(&TagEntry{}).Count(&count).Error
	return count, err
}

func (e *TagEntry) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", e.Key).Delete(&TagEntry{}).Error
}

func (e *TagEntry) DeleteByNetwork(ctx context.Context) error {
	return db.FromContext(ctx).Where("network_id = ?", e.NetworkID).Delete(&TagEntry{}).Error
}
