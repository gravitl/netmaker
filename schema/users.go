package schema

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
)

type AuthType string

var (
	BasicAuth AuthType = "basic_auth"
	OAuth     AuthType = "oauth"
)

type Theme string

const (
	Dark   Theme = "dark"
	Light  Theme = "light"
	System Theme = "system"
)

var (
	ErrUserIdentifiersNotProvided = errors.New("user identifiers not provided")
	ErrTenantIDNotProvided        = errors.New("tenant ID not provided")
)

type User struct {
	ID                         string     `gorm:"primaryKey" json:"id"`
	Username                   string     `gorm:"unique" json:"username"`
	DisplayName                string     `json:"display_name"`
	PlatformRoleID             UserRoleID `gorm:"-" json:"platform_role_id,omitempty"`
	ExternalIdentityProviderID string     `gorm:"-" json:"external_identity_provider_id"`
	AccountDisabled            bool       `json:"account_disabled"`
	AuthType                   AuthType   `gorm:"-" json:"auth_type"`
	Password                   string     `gorm:"-" json:"password"`
	IsMFAEnabled               bool       `json:"is_mfa_enabled"`
	TOTPSecret                 string     `json:"totp_secret"`
	// NOTE: json tag is different from field name to ensure compatibility with the older model.
	LastLoginAt time.Time `json:"last_login_time"`
	// NOTE: json tag is different from field name to ensure compatibility with the older model.
	UserGroups    datatypes.JSONType[map[UserGroupID]struct{}] `gorm:"-" json:"user_group_ids,omitempty"`
	Theme         Theme                                        `json:"theme"`
	TextSize      string                                       `json:"text_size"`
	ReducedMotion bool                                         `json:"reduced_motion"`
	CreatedBy     string                                       `json:"created_by"`
	CreatedAt     time.Time                                    `json:"created_at"`
	UpdatedAt     time.Time                                    `json:"updated_at"`
}

func (u *User) TableName() string {
	return "users_v1"
}

// userWithMembership is a flattened scan target for queries that JOIN tenant_memberships_v1.
type userWithMembership struct {
	User
	MemberRoleID                     UserRoleID                                   `gorm:"column:member_role_id"`
	MemberGroups                     datatypes.JSONType[map[UserGroupID]struct{}] `gorm:"column:member_groups"`
	MemberAuthType                   AuthType                                     `gorm:"column:member_auth_type"`
	MemberExternalIdentityProviderID string                                       `gorm:"column:member_external_identity_provider_id"`
	MemberPassword                   string                                       `gorm:"column:member_password"`
}

func (u *User) SuperAdminExists(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM tenant_memberships_v1 WHERE role_id = ?)",
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

	tenantID := scope.ID(ctx)
	if tenantID == "" {
		return db.FromContext(ctx).Model(&User{}).
			Where("id = ? OR username = ?", u.ID, u.Username).
			First(u).
			Error
	}

	var row userWithMembership
	err := db.FromContext(ctx).
		Table("users_v1").
		Select("users_v1.*, tm.role_id AS member_role_id, tm.groups AS member_groups, tm.auth_type AS member_auth_type, tm.external_identity_provider_id AS member_external_identity_provider_id, tm.password AS member_password").
		Joins("LEFT JOIN tenant_memberships_v1 tm ON tm.user_id = users_v1.id AND tm.tenant_id = ?", tenantID).
		Where("users_v1.id = ? OR users_v1.username = ?", u.ID, u.Username).
		First(&row).
		Error
	if err != nil {
		return err
	}
	*u = row.User
	u.PlatformRoleID = row.MemberRoleID
	u.UserGroups = row.MemberGroups
	u.AuthType = row.MemberAuthType
	u.ExternalIdentityProviderID = row.MemberExternalIdentityProviderID
	u.Password = row.MemberPassword
	return nil
}

func (u *User) GetByExternalID(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	if tenantID == "" {
		return ErrTenantIDNotProvided
	}

	if u.ExternalIdentityProviderID == "" {
		return ErrUserIdentifiersNotProvided
	}

	var row userWithMembership
	err := db.FromContext(ctx).
		Table("users_v1").
		Select("users_v1.*, tm.role_id AS member_role_id, tm.groups AS member_groups, tm.auth_type AS member_auth_type, tm.external_identity_provider_id AS member_external_identity_provider_id, tm.password AS member_password").
		Joins("LEFT JOIN tenant_memberships_v1 tm ON tm.user_id = users_v1.id AND tm.tenant_id = ?", tenantID).
		Where("tm.external_identity_provider_id = ?", u.ExternalIdentityProviderID).
		First(&row).
		Error
	if err != nil {
		return err
	}
	*u = row.User
	u.PlatformRoleID = row.MemberRoleID
	u.UserGroups = row.MemberGroups
	u.AuthType = row.MemberAuthType
	u.ExternalIdentityProviderID = row.MemberExternalIdentityProviderID
	u.Password = row.MemberPassword
	return nil
}

func (u *User) GetSuperAdmin(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	if tenantID == "" {
		return ErrTenantIDNotProvided
	}

	var row userWithMembership
	err := db.FromContext(ctx).
		Table("users_v1").
		Select("users_v1.*, tm.role_id AS member_role_id, tm.groups AS member_groups, tm.auth_type AS member_auth_type, tm.external_identity_provider_id AS member_external_identity_provider_id, tm.password AS member_password").
		Joins("INNER JOIN tenant_memberships_v1 tm ON tm.user_id = users_v1.id AND tm.tenant_id = ?", tenantID).
		Where("tm.role_id = ?", SuperAdminRole).
		First(&row).
		Error
	if err != nil {
		return err
	}
	*u = row.User
	u.PlatformRoleID = row.MemberRoleID
	u.UserGroups = row.MemberGroups
	u.AuthType = row.MemberAuthType
	u.ExternalIdentityProviderID = row.MemberExternalIdentityProviderID
	u.Password = row.MemberPassword
	return nil
}

func (u *User) Count(ctx context.Context, options ...dbtypes.Option) (int, error) {
	var count int64
	query := db.FromContext(ctx).Model(&User{})

	for _, option := range options {
		query = option(query)
	}

	err := query.Count(&count).Error
	return int(count), err
}

func (u *User) ListAll(ctx context.Context, options ...dbtypes.Option) ([]User, error) {
	tenantID := scope.ID(ctx)

	if tenantID == "" {
		var users []User
		query := db.FromContext(ctx).Model(&User{})
		for _, option := range options {
			query = option(query)
		}
		err := query.Find(&users).Error
		return users, err
	}

	query := db.FromContext(ctx).
		Table("users_v1").
		Select("users_v1.*, tm.role_id AS member_role_id, tm.groups AS member_groups, tm.auth_type AS member_auth_type, tm.external_identity_provider_id AS member_external_identity_provider_id, tm.password AS member_password").
		Joins("LEFT JOIN tenant_memberships_v1 tm ON tm.user_id = users_v1.id AND tm.tenant_id = ?", tenantID)

	for _, option := range options {
		query = option(query)
	}

	var rows []userWithMembership
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}

	users := make([]User, len(rows))
	for i, row := range rows {
		users[i] = row.User
		users[i].PlatformRoleID = row.MemberRoleID
		users[i].UserGroups = row.MemberGroups
		users[i].AuthType = row.MemberAuthType
		users[i].ExternalIdentityProviderID = row.MemberExternalIdentityProviderID
		users[i].Password = row.MemberPassword
	}
	return users, nil
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

func (u *User) UpdateUserSettings(ctx context.Context) error {
	if u.ID == "" && u.Username == "" {
		return ErrUserIdentifiersNotProvided
	}

	return db.FromContext(ctx).Model(&User{}).
		Where("id = ? OR username = ?", u.ID, u.Username).
		Updates(map[string]any{
			"theme":          u.Theme,
			"text_size":      u.TextSize,
			"reduced_motion": u.ReducedMotion,
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

// UpsertMembership persists PlatformRoleID and UserGroups to tenant_memberships_v1
// using the tenant ID from the scope context. No-op when no tenant is in scope.
func (u *User) UpsertMembership(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	if tenantID == "" {
		return nil
	}

	return (&TenantMembership{
		TenantID:                   tenantID,
		UserID:                     u.ID,
		RoleID:                     u.PlatformRoleID,
		Groups:                     u.UserGroups,
		AuthType:                   u.AuthType,
		ExternalIdentityProviderID: u.ExternalIdentityProviderID,
		Password:                   u.Password,
	}).Upsert(ctx)
}

// DeleteMembership removes the user's membership from the tenant in scope.
// No-op when no tenant is in scope.
func (u *User) DeleteMembership(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	if tenantID == "" {
		return nil
	}

	return (&TenantMembership{TenantID: tenantID, UserID: u.ID}).Delete(ctx)
}
