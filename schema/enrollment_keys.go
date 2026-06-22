package schema

import (
	"context"
	"time"

	dbtypes "github.com/gravitl/netmaker/db/types"
	"gorm.io/datatypes"

	"github.com/gravitl/netmaker/db"
)

type EnrollmentKeyType int

const (
	EnrollmentKeyType_TimedExpiry EnrollmentKeyType = iota + 1
	EnrollmentKeyType_LimitedUses
	EnrollmentKeyType_UnlimitedUses
)

type EnrollmentKey struct {
	ID                string                      `gorm:"primaryKey" json:"id"`
	TenantID          string                      `gorm:"default:'';index" json:"tenant_id"`
	Name              string                      `json:"name"`
	Value             string                      `json:"value"`
	Token             string                      `json:"token"`
	Default           bool                        `json:"default"`
	Unlimited         bool                        `json:"unlimited"`
	UsesRemaining     int                         `json:"uses_remaining"`
	Expiration        time.Time                   `json:"expiration"`
	Networks          datatypes.JSONSlice[string] `json:"networks"`
	Tags              datatypes.JSONSlice[string] `json:"tags"`
	GatewayID         *string                     `json:"gateway_id"`
	AutoEgress        bool                        `json:"auto_egress"`
	AutoAssignGateway bool                        `json:"auto_assign_gateway"`
	Type              EnrollmentKeyType           `json:"type"`
	CreatedBy         string                      `json:"created_by"`
	CreatedAt         time.Time                   `json:"created_at"`
	UpdatedAt         time.Time                   `json:"updated_at"`
}

func (e *EnrollmentKey) TableName() string {
	return "enrollment_keys_v1"
}

func (e *EnrollmentKey) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&EnrollmentKey{}).Create(e).Error
}

func (e *EnrollmentKey) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&EnrollmentKey{}).Where("id = ?", e.ID).First(e).Error
}

func (e *EnrollmentKey) GetByValue(ctx context.Context) error {
	return db.FromContext(ctx).Model(&EnrollmentKey{}).Where("value = ?", e.Value).First(e).Error
}

func (e *EnrollmentKey) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(e).Error
}

func (e *EnrollmentKey) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&EnrollmentKey{}).Where("id = ?", e.ID).Delete(e).Error
}

func (e *EnrollmentKey) DeleteByValue(ctx context.Context) error {
	return db.FromContext(ctx).Model(&EnrollmentKey{}).Where("value = ?", e.Value).Delete(e).Error
}

func (e *EnrollmentKey) ListAll(ctx context.Context, options ...dbtypes.Option) ([]EnrollmentKey, error) {
	var keys []EnrollmentKey
	query := db.FromContext(ctx).Model(&EnrollmentKey{})
	for _, opt := range options {
		query = opt(query)
	}
	err := query.Find(&keys).Error
	return keys, err
}

func (e *EnrollmentKey) Count(ctx context.Context, options ...dbtypes.Option) (int, error) {
	var count int64
	query := db.FromContext(ctx).Model(&EnrollmentKey{})
	for _, opt := range options {
		query = opt(query)
	}
	err := query.Count(&count).Error
	return int(count), err
}
