package schema

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type NetworkRoles map[NetworkID]map[UserRoleID]struct{}

type UserGroupID string

func (g UserGroupID) String() string {
	return string(g)
}

type UserGroup struct {
	ID                         string                           `gorm:"primaryKey" json:"-"`
	TenantID                   string                           `gorm:"default:'';index;uniqueIndex:udx_user_group_tenant_slug" json:"tenant_id"`
	Slug                       UserGroupID                      `gorm:"uniqueIndex:udx_user_group_tenant_slug" json:"id"`
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

var ErrUserGroupIdentifiersNotProvided = errors.New("user group identifiers not provided")

func (u *UserGroup) TableName() string {
	return userGroupsTable
}

// baseIdentifierQuery scopes the query to the group identified by its ID, or
// by its slug within the tenant in the context.
func (u *UserGroup) baseIdentifierQuery(ctx context.Context) (*gorm.DB, error) {
	tenantID := scope.ID(ctx)
	query := db.FromContext(ctx).Model(&UserGroup{})
	if u.ID != "" {
		query = query.Where(fmt.Sprintf("%s.id = ?", userGroupsTable), u.ID)
		if tenantID != "" {
			query = query.Where(fmt.Sprintf("%s.tenant_id = ?", userGroupsTable), tenantID)
		}
		return query, nil
	}

	if u.Slug == "" {
		return nil, ErrUserGroupIdentifiersNotProvided
	}

	return query.Where(fmt.Sprintf("%s.tenant_id = ? AND %s.slug = ?", userGroupsTable, userGroupsTable), tenantID, u.Slug), nil
}

// Create creates the group. The ID defaults to a new uuid, and the slug to
// the ID.
func (u *UserGroup) Create(ctx context.Context) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	if u.Slug == "" {
		u.Slug = UserGroupID(u.ID)
	}
	return db.FromContext(ctx).Model(&UserGroup{}).Create(u).Error
}

// Get fetches the group by its ID, or by its slug.
func (u *UserGroup) Get(ctx context.Context) error {
	query, err := u.baseIdentifierQuery(ctx)
	if err != nil {
		return err
	}

	var group UserGroup
	err = query.First(&group).Error
	if err != nil {
		return err
	}

	*u = group
	return nil
}

func (u *UserGroup) GetByName(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	return db.FromContext(ctx).Model(&UserGroup{}).
		Where(fmt.Sprintf("name = ? AND %s.tenant_id = ?", userGroupsTable), u.Name, tenantID).
		First(u).
		Error
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

	if tenantID := scope.ID(ctx); tenantID != "" {
		options = append(options, dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", userGroupsTable), tenantID))
	}

	for _, option := range options {
		query = option(query)
	}

	err := query.Find(&userGroups).Error
	return userGroups, err
}

// Update updates the non-zero fields of the group identified by its ID, or by
// its slug.
func (u *UserGroup) Update(ctx context.Context) error {
	query, err := u.baseIdentifierQuery(ctx)
	if err != nil {
		return err
	}

	return query.Omit("id", "tenant_id", "slug").Updates(u).Error
}

// Upsert overwrites the group identified by its ID, or by its slug, creating
// it if it does not exist.
func (u *UserGroup) Upsert(ctx context.Context) error {
	if u.ID == "" {
		existing := &UserGroup{Slug: u.Slug}
		err := existing.Get(ctx)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err != nil {
			return u.Create(ctx)
		}
		u.ID = existing.ID
	}
	if u.Slug == "" {
		u.Slug = UserGroupID(u.ID)
	}
	return db.FromContext(ctx).Save(u).Error
}

// Delete deletes the group identified by its ID, or by its slug.
func (u *UserGroup) Delete(ctx context.Context) error {
	query, err := u.baseIdentifierQuery(ctx)
	if err != nil {
		return err
	}

	return query.Delete(&UserGroup{}).Error
}

func (u *UserGroup) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Where(fmt.Sprintf("%s.tenant_id = ?", userGroupsTable), tenantID).Delete(&UserGroup{}).Error
	}
	return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s", userGroupsTable)).Error
}
