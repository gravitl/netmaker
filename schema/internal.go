package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
)

const (
	InternalKey_ServerID                        = "server_id"
	InternalKey_JwtSecret                       = "jwt_secret"
	InternalKey_OAuthSecret                     = "oauth_secret"
	InternalKey_MqPrivateKey                    = "mq_private_key"
	InternalKey_MqPublicKey                     = "mq_public_key"
	InternalKey_LicenseValidationPrivateKey     = "license_validation_private_key"
	InternalKey_LicenseValidationPublicKey      = "license_validation_public_key"
	InternalKey_LicenseValidationCachedResponse = "license_validation_cached_response"
	InternalKey_TelemetryLastReportedAt         = "telemetry_last_reported_at"
)

type Internal struct {
	Key      string `gorm:"primaryKey"`
	TenantID string `gorm:"default:'';index"`
	Value    string `gorm:"not null"`
}

func (i *Internal) TableName() string {
	return "__internal__"
}

func (i *Internal) Set(ctx context.Context) error {
	return db.FromContext(ctx).Save(i).Error
}

func (i *Internal) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Internal{}).
		Where("key = ?", i.Key).
		First(i).
		Error
}

func (i *Internal) Reset(ctx context.Context) error {
	return db.FromContext(ctx).Model(&Internal{}).
		Where("key = ?", i.Key).
		Delete(i).
		Error
}
