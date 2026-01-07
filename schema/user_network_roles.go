package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
)

type UserNetworkRole struct {
	UserID    string `gorm:"primaryKey"`
	NetworkID string `gorm:"primaryKey"`
	RoleID    string
}

func (u *UserNetworkRole) TableName() string {
	return "user_network_roles_v1"
}

func (u *UserNetworkRole) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserNetworkRole{}).Create(u).Error
}

func (u *UserNetworkRole) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserNetworkRole{}).
		Where("user_id = ? AND network_id = ?", u.UserID, u.NetworkID).
		First(u).
		Error
}

func (u *UserNetworkRole) ListAllNetworkRoles(ctx context.Context) ([]UserNetworkRole, error) {
	var roles []UserNetworkRole
	err := db.FromContext(ctx).Model(&UserNetworkRole{}).
		Where("user_id = ?", u.UserID).
		Find(&roles).
		Error
	return roles, err
}

func (u *UserNetworkRole) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserNetworkRole{}).Delete(u).Error
}
