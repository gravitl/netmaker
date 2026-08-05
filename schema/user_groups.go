package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
)

type NetworkRoles map[NetworkID]map[UserRoleID]struct{}

type UserGroupID string

func (g UserGroupID) String() string {
	return string(g)
}

type UserGroup struct {
	ID                         UserGroupID                      `gorm:"primaryKey" json:"id"`
	TenantID                   string                           `gorm:"default:'';index" json:"tenant_id"`
	Name                       string                           `json:"name"`
	Default                    bool                             `json:"default"`
	ExternalIdentityProviderID string                           `json:"external_identity_provider_id"`
	NetworkRoles               datatypes.JSONType[NetworkRoles] `json:"network_roles"`
	ColorCode                  string                           `json:"color_code"`
	MetaData                   string                           `json:"meta_data"`
	CreatedBy                  string                           `json:"created_by"`
	CreatedAt                  time.Time                        `json:"created_at"`
	UpdatedAt                  time.Time                        `json:"updated_at"`
}

const userGroupsTable = "user_groups_v1"

func (u *UserGroup) TableName() string {
	return userGroupsTable
}

func ScopeUserGroupID(tenantID string, id UserGroupID) UserGroupID {
	if tenantID == "" || id == "" {
		return id
	}
	if _, err := uuid.Parse(id.String()); err == nil {
		return id
	}
	return UserGroupID(TenantScopedKey(tenantID, id.String()))
}

func UnscopeUserGroupID(tenantID string, id UserGroupID) UserGroupID {
	if tenantID == "" || id == "" {
		return id
	}
	return UserGroupID(StripTenantKey(tenantID, id.String()))
}

func (u *UserGroup) Create(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	logicalID := u.ID
	u.ID = ScopeUserGroupID(tenantID, logicalID)
	err := db.FromContext(ctx).Model(&UserGroup{}).Create(u).Error
	u.ID = logicalID
	return err
}

func (u *UserGroup) Get(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	logicalID := u.ID
	u.ID = ScopeUserGroupID(tenantID, logicalID)
	err := db.FromContext(ctx).Model(&UserGroup{}).
		Where(fmt.Sprintf("id = ? AND %s.tenant_id = ?", userGroupsTable), u.ID, tenantID).
		First(u).
		Error
	if err != nil {
		return err
	}
	u.ID = logicalID
	return nil
}

func (u *UserGroup) GetByName(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	err := db.FromContext(ctx).Model(&UserGroup{}).
		Where(fmt.Sprintf("name = ? AND %s.tenant_id = ?", userGroupsTable), u.Name, tenantID).
		First(u).
		Error
	if err != nil {
		return err
	}
	u.ID = UnscopeUserGroupID(tenantID, u.ID)
	return nil
}

func (u *UserGroup) Count(ctx context.Context, options ...dbtypes.Option) (int, error) {
	var count int64
	query := db.FromContext(ctx).Model(&UserGroup{})

	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", userGroupsTable), tenantID))
	}

	for _, option := range options {
		query = option(query)
	}

	err := query.Count(&count).Error
	return int(count), err
}

func (u *UserGroup) ListAll(ctx context.Context, options ...dbtypes.Option) ([]UserGroup, error) {
	var userGroups []UserGroup
	query := db.FromContext(ctx).Model(&UserGroup{})

	tenantID := scope.ID(ctx)
	if tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", userGroupsTable), tenantID))
	}

	for _, option := range options {
		query = option(query)
	}

	err := query.Find(&userGroups).Error
	for i := range userGroups {
		userGroups[i].ID = UnscopeUserGroupID(tenantID, userGroups[i].ID)
	}
	return userGroups, err
}

func (u *UserGroup) Update(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	logicalID := u.ID
	u.ID = ScopeUserGroupID(tenantID, logicalID)
	err := db.FromContext(ctx).Model(&UserGroup{}).
		Where(fmt.Sprintf("id = ? AND %s.tenant_id = ?", userGroupsTable), u.ID, tenantID).
		Updates(u).
		Error
	u.ID = logicalID
	return err
}

func (u *UserGroup) Upsert(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	logicalID := u.ID
	u.ID = ScopeUserGroupID(tenantID, logicalID)
	err := db.FromContext(ctx).Save(u).Error
	u.ID = logicalID
	return err
}

func (u *UserGroup) Delete(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	return db.FromContext(ctx).Model(&UserGroup{}).
		Where(fmt.Sprintf("id = ? AND %s.tenant_id = ?", userGroupsTable), ScopeUserGroupID(tenantID, u.ID), tenantID).
		Delete(&UserGroup{}).
		Error
}

func (u *UserGroup) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Where(fmt.Sprintf("%s.tenant_id = ?", userGroupsTable), tenantID).Delete(&UserGroup{}).Error
	}
	return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s", userGroupsTable)).Error
}
