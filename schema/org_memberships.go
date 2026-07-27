package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
)

type OrgMembership struct {
	OrganizationID             string     `gorm:"primaryKey" json:"organization_id"`
	UserID                     string     `gorm:"primaryKey" json:"user_id"`
	RoleID                     UserRoleID `json:"role_id"`
	AuthType                   AuthType   `json:"auth_type"`
	ExternalIdentityProviderID string     `json:"external_identity_provider_id"`
	Password                   string     `json:"password"`
	AccountDisabled            bool       `json:"account_disabled"`
	IsMFAEnabled               bool       `json:"is_mfa_enabled"`
	TOTPSecret                 string     `json:"totp_secret"`
}

func (o *OrgMembership) TableName() string {
	return "org_memberships_v1"
}

func (o *OrgMembership) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&OrgMembership{}).Create(o).Error
}

func (o *OrgMembership) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&OrgMembership{}).
		Where("organization_id = ? AND user_id = ?", o.OrganizationID, o.UserID).
		First(o).
		Error
}

func (o *OrgMembership) GetOwner(ctx context.Context) error {
	return db.FromContext(ctx).Model(&OrgMembership{}).
		Where("organization_id = ? AND role_id = ?", o.OrganizationID, OrgOwner).
		First(o).
		Error
}

func (o *OrgMembership) ListByUserID(ctx context.Context) ([]OrgMembership, error) {
	var memberships []OrgMembership
	err := db.FromContext(ctx).Model(&OrgMembership{}).
		Where("user_id = ?", o.UserID).
		Find(&memberships).
		Error
	return memberships, err
}

func (o *OrgMembership) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(o).Error
}

func (o *OrgMembership) UpdateAccountStatus(ctx context.Context) error {
	return db.FromContext(ctx).Model(&OrgMembership{}).
		Where("organization_id = ? AND user_id = ?", o.OrganizationID, o.UserID).
		Update("account_disabled", o.AccountDisabled).
		Error
}

func (o *OrgMembership) UpdateMFA(ctx context.Context) error {
	return db.FromContext(ctx).Model(&OrgMembership{}).
		Where("organization_id = ? AND user_id = ?", o.OrganizationID, o.UserID).
		Updates(map[string]any{
			"is_mfa_enabled": o.IsMFAEnabled,
			"totp_secret":    o.TOTPSecret,
		}).
		Error
}

func (o *OrgMembership) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&OrgMembership{}).
		Where("organization_id = ? AND user_id = ?", o.OrganizationID, o.UserID).
		Delete(o).
		Error
}
