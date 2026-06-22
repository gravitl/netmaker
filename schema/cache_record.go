package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

// CacheValue - the cache object for a network
type CacheValue struct {
	Network    string    `json:"network,omitempty"`
	Value      string    `json:"value"`
	Host       Host      `json:"host"`
	Pass       string    `json:"pass,omitempty"`
	User       string    `json:"user,omitempty"`
	ALL        bool      `json:"all,omitempty"`
	Expiration time.Time `json:"expiration"`
}

type CacheRecord struct {
	Key      string `gorm:"primaryKey"`
	TenantID string `gorm:"default:'';index"`
	Value    datatypes.JSONType[CacheValue]
}

func (*CacheRecord) TableName() string { return "cache" }

func (r *CacheRecord) Get(ctx context.Context) error {
	return db.FromContext(ctx).First(r).Error
}

func (r *CacheRecord) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(r).Error
}

func (r *CacheRecord) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Delete(r).Error
}

func (*CacheRecord) List(ctx context.Context) ([]CacheRecord, error) {
	var records []CacheRecord
	err := db.FromContext(ctx).Find(&records).Error
	return records, err
}

func (*CacheRecord) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&CacheRecord{}).Count(&count).Error
	return int(count), err
}
