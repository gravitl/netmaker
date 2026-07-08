package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

const DefaultSsoStateDuration = time.Minute * 5

// SsoState - holds SSO sign-in session data
type SsoState struct {
	AppName    string    `json:"app_name"`
	Value      string    `json:"value"`
	Expiration time.Time `json:"expiration"`
}

func (s *SsoState) IsExpired() bool { return time.Now().After(s.Expiration) }

type SsoStateRecord struct {
	Key      string `gorm:"primaryKey"`
	TenantID string `gorm:"default:'';index"`
	Value    datatypes.JSONType[SsoState]
}

func (*SsoStateRecord) TableName() string { return "ssostatecache" }

func (r *SsoStateRecord) Get(ctx context.Context) error {
	return db.FromContext(ctx).First(r).Error
}

func (r *SsoStateRecord) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(r).Error
}

func (r *SsoStateRecord) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Delete(r).Error
}

func (*SsoStateRecord) List(ctx context.Context) ([]SsoStateRecord, error) {
	var records []SsoStateRecord
	err := db.FromContext(ctx).Find(&records).Error
	return records, err
}

func (*SsoStateRecord) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&SsoStateRecord{}).Count(&count).Error
	return int(count), err
}
