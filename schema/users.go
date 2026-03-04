package schema

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type AuthType string

var (
	BasicAuth AuthType = "basic_auth"
	OAuth     AuthType = "oauth"
)

var (
	ErrUserIdentifiersNotProvided = errors.New("user identifiers not provided")
)

type User struct {
	ID                         string     `gorm:"primaryKey" json:"id"`
	Username                   string     `gorm:"unique" json:"username"`
	DisplayName                string     `json:"display_name"`
	PlatformRoleID             UserRoleID `json:"platform_role_id"`
	ExternalIdentityProviderID string     `json:"external_identity_provider_id"`
	AccountDisabled            bool       `json:"account_disabled"`
	AuthType                   AuthType   `json:"auth_type"`
	Password                   string     `json:"password"`
	IsMFAEnabled               bool       `json:"is_mfa_enabled"`
	TOTPSecret                 string     `json:"totp_secret"`
	// NOTE: json tag is different from field name to ensure compatibility with the older model.
	LastLoginAt time.Time `json:"last_login_time"`
	// NOTE: json tag is different from field name to ensure compatibility with the older model.
	UserGroups datatypes.JSONType[map[UserGroupID]struct{}] `json:"user_group_ids"`
	CreatedBy  string                                       `json:"created_by"`
	CreatedAt  time.Time                                    `json:"created_at"`
	UpdatedAt  time.Time                                    `json:"updated_at"`
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
	err := db.FromContext(ctx).Model(&User{}).Find(&users).Order("username ASC").Error
	return users, err
}

func (u *User) Update(ctx context.Context) error {
	if u.ID == "" && u.Username == "" {
		return ErrUserIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&User{}).
		Where("id = ? OR username = ?", u.ID, u.Username).
		Updates(u).
		Error
}

func (u *User) UpdateAccountStatus(ctx context.Context) error {
	if u.ID == "" && u.Username == "" {
		return ErrUserIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&User{}).
		Where("id = ? OR username = ?", u.ID, u.Username).
		Updates(map[string]any{
			"account_disabled": u.AccountDisabled,
		}).
		Error
}

func (u *User) UpdateMFA(ctx context.Context) error {
	if u.ID == "" && u.Username == "" {
		return ErrUserIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&User{}).
		Where("id = ? OR username = ?", u.ID, u.Username).
		Updates(map[string]any{
			"is_mfa_enabled": u.IsMFAEnabled,
			"totp_secret":    u.TOTPSecret,
		}).
		Error
}

func (u *User) Delete(ctx context.Context) error {
	if u.ID == "" && u.Username == "" {
		return ErrUserIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&User{}).
		Where("id = ? OR username = ?", u.ID, u.Username).
		Delete(u).
		Error
}
