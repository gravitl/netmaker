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
