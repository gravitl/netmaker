package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type TenantMembership struct {
	TenantID                   string                                       `gorm:"primaryKey" json:"tenant_id"`
	UserID                     string                                       `gorm:"primaryKey" json:"user_id"`
	RoleID                     UserRoleID                                   `json:"role_id"`
	Groups                     datatypes.JSONType[map[UserGroupID]struct{}] `json:"groups"`
	AuthType                   AuthType                                     `json:"auth_type"`
	ExternalIdentityProviderID string                                       `json:"external_identity_provider_id"`
	Password                   string                                       `json:"password"`
}

func (t *TenantMembership) TableName() string {
	return "tenant_memberships_v1"
}

func (t *TenantMembership) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&TenantMembership{}).Create(t).Error
}

func (t *TenantMembership) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(t).Error
}

func (t *TenantMembership) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&TenantMembership{}).
		Where("tenant_id = ? AND user_id = ?", t.TenantID, t.UserID).
		First(t).
		Error
}

func (t *TenantMembership) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&TenantMembership{}).
		Where("tenant_id = ? AND user_id = ?", t.TenantID, t.UserID).
		Delete(t).
		Error
}

func (t *TenantMembership) UpdateRoleID(ctx context.Context) error {
	return db.FromContext(ctx).Model(&TenantMembership{}).
		Where("tenant_id = ?  AND user_id = ?", t.TenantID, t.UserID).
		Update("role_id", t.RoleID).
		Error
}
