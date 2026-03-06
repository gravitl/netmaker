package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type NetworkRoles map[NetworkID]map[UserRoleID]struct{}

type UserGroupID string

func (g UserGroupID) String() string {
	return string(g)
}

type UserGroup struct {
	ID                         UserGroupID                      `gorm:"primaryKey" json:"id"`
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

func (u *UserGroup) TableName() string {
	return "user_groups_v1"
}

func (u *UserGroup) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserGroup{}).Create(u).Error
}

func (u *UserGroup) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserGroup{}).
		Where("id = ?", u.ID).
		First(u).
		Error
}

func (u *UserGroup) GetByName(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserGroup{}).
		Where("name = ?", u.Name).
		First(u).
		Error
}

func (u *UserGroup) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&UserGroup{}).Count(&count).Error
	return int(count), err
}

func (u *UserGroup) ListAll(ctx context.Context) ([]UserGroup, error) {
	var userGroups []UserGroup
	err := db.FromContext(ctx).Model(&UserGroup{}).Find(&userGroups).Order("name ASC").Error
	return userGroups, err
}

func (u *UserGroup) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserGroup{}).
		Where("id = ?", u.ID).
		Updates(u).
		Error
}

func (u *UserGroup) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(u).Error
}

func (u *UserGroup) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserGroup{}).
		Where("id = ?", u.ID).
		Delete(u).
		Error
}
