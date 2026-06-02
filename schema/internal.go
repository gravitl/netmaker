package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
)

const (
	InternalKey_JwtSecret                   = "jwt_secret"
	InternalKey_LicenseValidationPrivateKey = "license_validation_private_key"
	InternalKey_LicenseValidationPublicKey  = "license_validation_public_key"
	InternalKey_OAuthSecret                 = "oauth_secret"
)

type Internal struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"not null"`
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
