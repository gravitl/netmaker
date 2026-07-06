package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
)

type OrgMembership struct {
	OrganizationID string     `gorm:"primaryKey" json:"organization_id"`
	UserID         string     `gorm:"primaryKey" json:"user_id"`
	RoleID         UserRoleID `json:"role_id"`
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
