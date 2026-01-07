package schema

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
)

const (
	SuperAdminRole = "super-admin"
)

var (
	ErrUserIdentifiersNotProvided = errors.New("user identifiers not provided")
)

type User struct {
	ID                         string `gorm:"primaryKey"`
	Username                   string `gorm:"unique"`
	DisplayName                string
	PlatformRoleID             string
	ExternalIdentityProviderID string
	AccountDisabled            bool
	AuthType                   string
	Password                   string
	IsMFAEnabled               bool
	TOTPSecret                 string
	LastLoginAt                time.Time
	CreatedBy                  string
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

func (u *User) TableName() string {
	return "users_v1"
}

func (u *User) SuperAdminExists(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM users_v1 WHERE platform_role_id = ?)",
		SuperAdminRole,
	).Scan(&exists).Error
	return exists, err
}

func (u *User) Create(ctx context.Context) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}

	return db.FromContext(ctx).Model(&User{}).Create(u).Error
}

func (u *User) Get(ctx context.Context) error {
	if u.ID == "" && u.Username == "" {
		return ErrUserIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&User{}).
		Where("id = ? OR username = ?", u.ID, u.Username).
		First(u).
		Error
}

func (u *User) GetSuperAdmin(ctx context.Context) error {
	return db.FromContext(ctx).Model(u).
		Where("platform_role_id = ?", SuperAdminRole).
		First(u).
		Error
}

func (u *User) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&User{}).Count(&count).Error
	return int(count), err
}

func (u *User) ListAll(ctx context.Context) ([]User, error) {
	var users []User
	err := db.FromContext(ctx).Model(&User{}).Find(&users).Error
	return users, err
}

func (u *User) Update(ctx context.Context) error {
	if u.ID == "" && u.Username == "" {
		return ErrUserIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&User{}).
		Where("id = ? OR username = ?", u.ID, u.Username).
		Updates(u).Error
}

func (u *User) Delete(ctx context.Context) error {
	if u.ID == "" && u.Username == "" {
		return ErrUserIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&User{}).
		Where("id = ? OR username = ?", u.ID, u.Username).
		Delete(u).Error
}

func (u *User) Enable(ctx context.Context) error {
	if u.ID == "" && u.Username == "" {
		return ErrUserIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&User{}).
		Where("id = ? OR username = ?", u.ID, u.Username).
		Updates(map[string]any{
			"account_disabled": false,
		}).Error
}

func (u *User) DisableMFA(ctx context.Context) error {
	if u.ID == "" && u.Username == "" {
		return ErrUserIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&User{}).
		Where("id = ? OR username = ?", u.ID, u.Username).
		Updates(map[string]any{
			"is_mfa_enabled": false,
			"totp_secret":    "",
		}).Error
}
