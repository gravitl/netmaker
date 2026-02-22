package schema

import (
	"context"
	"errors"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type NetworkRoles map[models.NetworkID]map[models.UserRoleID]struct{}

type UserGroup struct {
	ID                         models.UserGroupID               `gorm:"primaryKey" json:"id"`
	Name                       string                           `gorm:"unique" json:"name"`
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

func (u *UserGroup) ListAll(ctx context.Context) ([]UserGroup, error) {
	var userGroups []UserGroup
	err := db.FromContext(ctx).Model(&UserGroup{}).Find(&userGroups).Error
	return userGroups, err
}

func (u *UserGroup) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserGroup{}).
		Where("id = ?", u.ID).
		Updates(u).
		Error
}

func (u *UserGroup) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Transaction(func(tx *gorm.DB) error {
		var currUserGroup UserGroup
		err := tx.Model(&UserGroup{}).
			Where("id = ?", u.ID).
			First(&currUserGroup).
			Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return tx.Create(u).Error
			}

			return err
		}

		return tx.Model(&UserGroup{}).Updates(u).Error
	})
}

func (u *UserGroup) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserGroup{}).
		Where("id = ?", u.ID).
		Delete(u).
		Error
}
