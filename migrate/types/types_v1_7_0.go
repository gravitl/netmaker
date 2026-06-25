package types

import (
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

// LegacyUser holds the columns that existed on users_v1 before membership tables
// were introduced. Used only during the v1.7.0 migration to seed tenant_memberships_v1.
type LegacyUser struct {
	ID             string                                              `gorm:"column:id"`
	PlatformRoleID schema.UserRoleID                                   `gorm:"column:platform_role_id"`
	UserGroups     datatypes.JSONType[map[schema.UserGroupID]struct{}] `gorm:"column:user_group_ids"`
}

func (LegacyUser) TableName() string {
	return "users_v1"
}
