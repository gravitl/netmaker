package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

const DefaultSsoStateDuration = time.Minute * 5

type SsoState struct {
	AppName    string    `json:"app_name"`
	Value      string    `json:"value"`
	Expiration time.Time `json:"expiration"`
}

func (s *SsoState) IsExpired() bool {
	return time.Now().After(s.Expiration)
}

type SsoStateEntry struct {
	Key      string                       `gorm:"primaryKey;column:key"`
	TenantID string                       `gorm:"column:tenant_id;default:''"`
	Value    datatypes.JSONType[SsoState] `gorm:"column:value"`
}

func (*SsoStateEntry) TableName() string { return "ssostatecache" }

func (e *SsoStateEntry) Save(ctx context.Context) error {
	return db.FromContext(ctx).Save(e).Error
}

func (e *SsoStateEntry) Get(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", e.Key).First(e).Error
}

func (e *SsoStateEntry) ListAll(ctx context.Context) ([]SsoStateEntry, error) {
	var entries []SsoStateEntry
	err := db.FromContext(ctx).Find(&entries).Error
	return entries, err
}

func (e *SsoStateEntry) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", e.Key).Delete(&SsoStateEntry{}).Error
}
