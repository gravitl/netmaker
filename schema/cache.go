package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type CacheValue struct {
	Network    string    `json:"network,omitempty"`
	Value      string    `json:"value"`
	Host       Host      `json:"host"`
	Pass       string    `json:"pass,omitempty"`
	User       string    `json:"user,omitempty"`
	ALL        bool      `json:"all,omitempty"`
	Expiration time.Time `json:"expiration"`
}

type CacheEntry struct {
	Key      string                         `gorm:"primaryKey;column:key"`
	TenantID string                         `gorm:"column:tenant_id;default:''"`
	Value    datatypes.JSONType[CacheValue] `gorm:"column:value"`
}

func (*CacheEntry) TableName() string { return "cache" }

func (e *CacheEntry) Save(ctx context.Context) error {
	return db.FromContext(ctx).Save(e).Error
}

func (e *CacheEntry) Get(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", e.Key).First(e).Error
}

func (e *CacheEntry) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", e.Key).Delete(&CacheEntry{}).Error
}
