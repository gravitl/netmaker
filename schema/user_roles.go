package schema

import (
	"context"
	"errors"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ResourceAccess map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope

type UserRole struct {
	ID                  models.UserRoleID                  `gorm:"primaryKey" json:"id"`
	Name                string                             `gorm:"unique" json:"name"`
	Default             bool                               `json:"default"`
	MetaData            string                             `json:"meta_data"`
	DenyDashboardAccess bool                               `json:"deny_dashboard_access"`
	FullAccess          bool                               `json:"full_access"`
	NetworkID           models.NetworkID                   `json:"network_id"`
	NetworkLevelAccess  datatypes.JSONType[ResourceAccess] `json:"network_level_access"`
	GlobalLevelAccess   datatypes.JSONType[ResourceAccess] `json:"global_level_access"`
}

func (u *UserRole) TableName() string {
	return "user_roles_v1"
}

func (u *UserRole) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserRole{}).Create(u).Error
}

func (u *UserRole) Exists(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM user_roles_v1 WHERE name = ?)",
		u.Name,
	).Scan(&exists).Error
	return exists, err
}

func (u *UserRole) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserRole{}).
		Where("id = ?", u.ID).
		First(u).
		Error
}

func (u *UserRole) ListPlatformRoles(ctx context.Context) ([]UserRole, error) {
	var userRoles []UserRole
	err := db.FromContext(ctx).Model(&UserRole{}).
		Where("network_id = ''").
		Find(&userRoles).
		Error
	return userRoles, err
}

func (u *UserRole) ListNetworkRoles(ctx context.Context) ([]UserRole, error) {
	var userRoles []UserRole
	err := db.FromContext(ctx).Model(&UserRole{}).
		Where("network_id <> ''").
		Find(&userRoles).
		Error
	return userRoles, err
}

func (u *UserRole) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Transaction(func(tx *gorm.DB) error {
		var currUserRole UserRole
		err := tx.Model(&UserRole{}).
			Where("id = ?", u.ID).
			First(&currUserRole).
			Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return tx.Create(u).Error
			}

			return err
		}

		return tx.Model(&UserRole{}).Updates(u).Error
	})
}

func (u *UserRole) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserRole{}).
		Where("id = ?", u.ID).
		Updates(u).
		Error
}

func (u *UserRole) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserRole{}).
		Where("id = ?", u.ID).
		Delete(u).
		Error
}

func (u *UserRole) DeleteNetworkRoles(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserRole{}).
		Where("network_id <> '' && network_id = ?", u.NetworkID).
		Delete(u).
		Error
}
