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
	"gorm.io/gorm"
)

type AuthType string

var (
	BasicAuth AuthType = "basic_auth"
	OAuth     AuthType = "oauth"
	Inherited AuthType = "inherited"
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
	ErrScopeNotProvider           = errors.New("scope not provider")
)

type User struct {
	ID                         string     `gorm:"primaryKey" json:"id"`
	Username                   string     `gorm:"unique" json:"username"`
	DisplayName                string     `json:"display_name"`
	PlatformRoleID             UserRoleID `gorm:"-" json:"platform_role_id,omitempty"`
	ExternalIdentityProviderID string     `gorm:"-" json:"external_identity_provider_id"`
	AccountDisabled            bool       `gorm:"-" json:"account_disabled"`
	AuthType                   AuthType   `gorm:"-" json:"auth_type"`
	Password                   string     `gorm:"-" json:"password"`
	IsMFAEnabled               bool       `gorm:"-" json:"is_mfa_enabled"`
	TOTPSecret                 string     `gorm:"-" json:"totp_secret"`
	EmailValidated             bool       `json:"email_validated"`
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

// userWithOrgMembership is a flattened scan target for queries that JOIN org_memberships_v1.
type userWithOrgMembership struct {
	User
	MemberRoleID                     UserRoleID `gorm:"column:member_role_id"`
	MemberAuthType                   AuthType   `gorm:"column:member_auth_type"`
	MemberExternalIdentityProviderID string     `gorm:"column:member_external_identity_provider_id"`
	MemberPassword                   string     `gorm:"column:member_password"`
	MemberAccountDisabled            bool       `gorm:"column:member_account_disabled"`
	MemberIsMFAEnabled               bool       `gorm:"column:member_is_mfa_enabled"`
	MemberTOTPSecret                 string     `gorm:"column:member_totp_secret"`
}

// userWithMembership is a flattened scan target for queries that JOIN tenant_memberships_v1.
type userWithMembership struct {
	User
	MemberRoleID                     UserRoleID                                   `gorm:"column:member_role_id"`
	MemberGroups                     datatypes.JSONType[map[UserGroupID]struct{}] `gorm:"column:member_groups"`
	MemberAuthType                   AuthType                                     `gorm:"column:member_auth_type"`
	MemberExternalIdentityProviderID string                                       `gorm:"column:member_external_identity_provider_id"`
	MemberPassword                   string                                       `gorm:"column:member_password"`
	MemberAccountDisabled            bool                                         `gorm:"column:member_account_disabled"`
	MemberIsMFAEnabled               bool                                         `gorm:"column:member_is_mfa_enabled"`
	MemberTOTPSecret                 string                                       `gorm:"column:member_totp_secret"`
}

func (u *User) SuperAdminExists(ctx context.Context) (bool, error) {
	tenantID := scope.ID(ctx)
	if tenantID == "" {
		return false, ErrTenantIDNotProvided
	}

	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM tenant_memberships_v1 WHERE tenant_id = ? AND role_id = ?)",
		tenantID,
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
	return u.get(ctx, false)
}

func (u *User) GetWithMembership(ctx context.Context) error {
	return u.get(ctx, true)
}

func (u *User) get(ctx context.Context, requireMembership bool) error {
	if u.ID == "" && u.Username == "" {
		return ErrUserIdentifiersNotProvided
	}

	joinType := "LEFT"
	if requireMembership {
		joinType = "INNER"
	}

	if scope.Level(ctx) == scope.OrgScope {
		orgID := scope.ID(ctx)
		var row userWithOrgMembership
		err := db.FromContext(ctx).
			Table("users_v1").
			Select("users_v1.*, om.role_id AS member_role_id, om.auth_type AS member_auth_type, om.external_identity_provider_id AS member_external_identity_provider_id, om.password AS member_password, om.account_disabled AS member_account_disabled, om.is_mfa_enabled AS member_is_mfa_enabled, om.totp_secret AS member_totp_secret").
			Joins(joinType+" JOIN org_memberships_v1 om ON om.user_id = users_v1.id AND om.organization_id = ?", orgID).
			Where("users_v1.id = ? OR users_v1.username = ?", u.ID, u.Username).
			First(&row).
			Error
		if err != nil {
			return err
		}
		*u = row.User
		u.PlatformRoleID = row.MemberRoleID
		u.AuthType = row.MemberAuthType
		u.ExternalIdentityProviderID = row.MemberExternalIdentityProviderID
		u.Password = row.MemberPassword
		u.AccountDisabled = row.MemberAccountDisabled
		u.IsMFAEnabled = row.MemberIsMFAEnabled
		u.TOTPSecret = row.MemberTOTPSecret
		return nil
	}

	if scope.Level(ctx) == scope.TenantScope {
		tenantID := scope.ID(ctx)
		var row userWithMembership
		err := db.FromContext(ctx).
			Table("users_v1").
			Select("users_v1.*, tm.role_id AS member_role_id, tm.groups AS member_groups, tm.auth_type AS member_auth_type, tm.external_identity_provider_id AS member_external_identity_provider_id, tm.password AS member_password, tm.account_disabled AS member_account_disabled, tm.is_mfa_enabled AS member_is_mfa_enabled, tm.totp_secret AS member_totp_secret").
			Joins(joinType+" JOIN tenant_memberships_v1 tm ON tm.user_id = users_v1.id AND tm.tenant_id = ?", tenantID).
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
		u.AccountDisabled = row.MemberAccountDisabled
		u.IsMFAEnabled = row.MemberIsMFAEnabled
		u.TOTPSecret = row.MemberTOTPSecret
		return nil
	}

	if requireMembership {
		return ErrScopeNotProvider
	}

	return db.FromContext(ctx).Model(&User{}).
		Where("id = ? OR username = ?", u.ID, u.Username).
		First(u).
		Error
}

func (u *User) GetByExternalID(ctx context.Context) error {
	if scope.ID(ctx) == "" {
		return ErrScopeNotProvider
	}

	if u.ExternalIdentityProviderID == "" {
		return ErrUserIdentifiersNotProvided
	}

	if scope.Level(ctx) == scope.OrgScope {
		var row userWithOrgMembership
		orgID := scope.ID(ctx)
		err := db.FromContext(ctx).
			Table("users_v1").
			Select("users_v1.*, om.role_id AS member_role_id, om.auth_type AS member_auth_type, om.external_identity_provider_id AS member_external_identity_provider_id, om.password AS member_password, om.account_disabled AS member_account_disabled").
			Joins("LEFT JOIN org_memberships_v1 om ON om.user_id = users_v1.id AND om.organization_id = ?", orgID).
			Where("om.external_identity_provider_id = ?", u.ExternalIdentityProviderID).
			First(&row).
			Error
		if err != nil {
			return err
		}
		*u = row.User
		u.PlatformRoleID = row.MemberRoleID
		u.AuthType = row.MemberAuthType
		u.ExternalIdentityProviderID = row.MemberExternalIdentityProviderID
		u.Password = row.MemberPassword
		u.AccountDisabled = row.MemberAccountDisabled
		return nil
	}

	var row userWithMembership
	tenantID := scope.ID(ctx)
	err := db.FromContext(ctx).
		Table("users_v1").
		Select("users_v1.*, tm.role_id AS member_role_id, tm.groups AS member_groups, tm.auth_type AS member_auth_type, tm.external_identity_provider_id AS member_external_identity_provider_id, tm.password AS member_password, tm.account_disabled AS member_account_disabled, tm.is_mfa_enabled AS member_is_mfa_enabled, tm.totp_secret AS member_totp_secret").
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
	u.AccountDisabled = row.MemberAccountDisabled
	u.IsMFAEnabled = row.MemberIsMFAEnabled
	u.TOTPSecret = row.MemberTOTPSecret
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
		Select("users_v1.*, tm.role_id AS member_role_id, tm.groups AS member_groups, tm.auth_type AS member_auth_type, tm.external_identity_provider_id AS member_external_identity_provider_id, tm.password AS member_password, tm.account_disabled AS member_account_disabled, tm.is_mfa_enabled AS member_is_mfa_enabled, tm.totp_secret AS member_totp_secret").
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
	u.AccountDisabled = row.MemberAccountDisabled
	u.IsMFAEnabled = row.MemberIsMFAEnabled
	u.TOTPSecret = row.MemberTOTPSecret
	return nil
}

func (u *User) Count(ctx context.Context, options ...dbtypes.Option) (int, error) {
	return u.count(ctx, false, options...)
}

func (u *User) CountWithMembership(ctx context.Context, options ...dbtypes.Option) (int, error) {
	return u.count(ctx, true, options...)
}

func (u *User) count(ctx context.Context, requireMembership bool, options ...dbtypes.Option) (int, error) {
	joinType := "LEFT"
	if requireMembership {
		joinType = "INNER"
	}

	var count int64
	var query *gorm.DB

	switch scope.Level(ctx) {
	case scope.OrgScope:
		orgID := scope.ID(ctx)
		query = db.FromContext(ctx).
			Table("users_v1").
			Joins(joinType+" JOIN org_memberships_v1 om ON om.user_id = users_v1.id AND om.organization_id = ?", orgID)
	case scope.TenantScope:
		tenantID := scope.ID(ctx)
		query = db.FromContext(ctx).
			Table("users_v1").
			Joins(joinType+" JOIN tenant_memberships_v1 tm ON tm.user_id = users_v1.id AND tm.tenant_id = ?", tenantID)
	default:
		if requireMembership {
			return 0, ErrScopeNotProvider
		}
		query = db.FromContext(ctx).Model(&User{})
	}

	for _, option := range options {
		query = option(query)
	}

	err := query.Count(&count).Error
	return int(count), err
}

func (u *User) ListAll(ctx context.Context, options ...dbtypes.Option) ([]User, error) {
	return u.listAll(ctx, false, options...)
}

func (u *User) ListAllWithMembership(ctx context.Context, options ...dbtypes.Option) ([]User, error) {
	return u.listAll(ctx, true, options...)
}

func (u *User) listAll(ctx context.Context, requireMembership bool, options ...dbtypes.Option) ([]User, error) {
	joinType := "LEFT"
	if requireMembership {
		joinType = "INNER"
	}

	if scope.Level(ctx) == scope.OrgScope {
		orgID := scope.ID(ctx)
		query := db.FromContext(ctx).
			Table("users_v1").
			Select("users_v1.*, om.role_id AS member_role_id, om.auth_type AS member_auth_type, om.external_identity_provider_id AS member_external_identity_provider_id, om.password AS member_password, om.account_disabled AS member_account_disabled, om.is_mfa_enabled AS member_is_mfa_enabled, om.totp_secret AS member_totp_secret").
			Joins(joinType+" JOIN org_memberships_v1 om ON om.user_id = users_v1.id AND om.organization_id = ?", orgID)

		for _, option := range options {
			query = option(query)
		}

		var rows []userWithOrgMembership
		if err := query.Find(&rows).Error; err != nil {
			return nil, err
		}

		users := make([]User, len(rows))
		for i, row := range rows {
			users[i] = row.User
			users[i].PlatformRoleID = row.MemberRoleID
			users[i].AuthType = row.MemberAuthType
			users[i].ExternalIdentityProviderID = row.MemberExternalIdentityProviderID
			users[i].Password = row.MemberPassword
			users[i].AccountDisabled = row.MemberAccountDisabled
			users[i].IsMFAEnabled = row.MemberIsMFAEnabled
			users[i].TOTPSecret = row.MemberTOTPSecret
		}
		return users, nil
	}

	if scope.Level(ctx) == scope.TenantScope {
		tenantID := scope.ID(ctx)
		query := db.FromContext(ctx).
			Table("users_v1").
			Select("users_v1.*, tm.role_id AS member_role_id, tm.groups AS member_groups, tm.auth_type AS member_auth_type, tm.external_identity_provider_id AS member_external_identity_provider_id, tm.password AS member_password, tm.account_disabled AS member_account_disabled, tm.is_mfa_enabled AS member_is_mfa_enabled, tm.totp_secret AS member_totp_secret").
			Joins(joinType+" JOIN tenant_memberships_v1 tm ON tm.user_id = users_v1.id AND tm.tenant_id = ?", tenantID)

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
			users[i].AccountDisabled = row.MemberAccountDisabled
			users[i].IsMFAEnabled = row.MemberIsMFAEnabled
			users[i].TOTPSecret = row.MemberTOTPSecret
		}
		return users, nil
	}

	if requireMembership {
		return nil, ErrScopeNotProvider
	}

	var users []User
	query := db.FromContext(ctx).Model(&User{})
	for _, option := range options {
		query = option(query)
	}
	err := query.Find(&users).Error
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
	scopeID := scope.ID(ctx)
	if scopeID == "" {
		return ErrScopeNotProvider
	}

	if scope.Level(ctx) == scope.OrgScope {
		return (&OrgMembership{
			OrganizationID:  scopeID,
			UserID:          u.ID,
			AccountDisabled: u.AccountDisabled,
		}).UpdateAccountStatus(ctx)
	}

	return (&TenantMembership{
		TenantID:        scopeID,
		UserID:          u.ID,
		AccountDisabled: u.AccountDisabled,
	}).UpdateAccountStatus(ctx)
}

func (u *User) UpdateMFA(ctx context.Context) error {
	scopeID := scope.ID(ctx)
	if scopeID == "" {
		return ErrScopeNotProvider
	}

	if scope.Level(ctx) == scope.OrgScope {
		return (&OrgMembership{
			OrganizationID: scopeID,
			UserID:         u.ID,
			IsMFAEnabled:   u.IsMFAEnabled,
			TOTPSecret:     u.TOTPSecret,
		}).UpdateMFA(ctx)
	}

	tm := &TenantMembership{TenantID: scopeID, UserID: u.ID}
	err := tm.Get(ctx)
	if err != nil {
		return err
	}

	if tm.AuthType == Inherited {
		tenant := &Tenant{ID: scopeID}
		err = tenant.Get(ctx)
		if err != nil {
			return err
		}

		return (&OrgMembership{
			OrganizationID: tenant.OrganizationID,
			UserID:         u.ID,
			IsMFAEnabled:   u.IsMFAEnabled,
			TOTPSecret:     u.TOTPSecret,
		}).UpdateMFA(ctx)
	}

	return (&TenantMembership{
		TenantID:     scopeID,
		UserID:       u.ID,
		IsMFAEnabled: u.IsMFAEnabled,
		TOTPSecret:   u.TOTPSecret,
	}).UpdateMFA(ctx)
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

func (u *User) UpsertMembership(ctx context.Context) error {
	scopeID := scope.ID(ctx)
	if scopeID == "" {
		return ErrScopeNotProvider
	}

	if scope.Level(ctx) == scope.OrgScope {
		return (&OrgMembership{
			OrganizationID:             scopeID,
			UserID:                     u.ID,
			RoleID:                     u.PlatformRoleID,
			AuthType:                   u.AuthType,
			ExternalIdentityProviderID: u.ExternalIdentityProviderID,
			Password:                   u.Password,
			AccountDisabled:            u.AccountDisabled,
			IsMFAEnabled:               u.IsMFAEnabled,
			TOTPSecret:                 u.TOTPSecret,
		}).Upsert(ctx)
	}

	return (&TenantMembership{
		TenantID:                   scopeID,
		UserID:                     u.ID,
		RoleID:                     u.PlatformRoleID,
		Groups:                     u.UserGroups,
		AuthType:                   u.AuthType,
		ExternalIdentityProviderID: u.ExternalIdentityProviderID,
		Password:                   u.Password,
		AccountDisabled:            u.AccountDisabled,
		IsMFAEnabled:               u.IsMFAEnabled,
		TOTPSecret:                 u.TOTPSecret,
	}).Upsert(ctx)
}

func (u *User) DeleteMembership(ctx context.Context) error {
	scopeID := scope.ID(ctx)
	if scopeID == "" {
		return ErrScopeNotProvider
	}

	if scope.Level(ctx) == scope.OrgScope {
		return (&OrgMembership{OrganizationID: scopeID, UserID: u.ID}).Delete(ctx)
	}

	return (&TenantMembership{TenantID: scopeID, UserID: u.ID}).Delete(ctx)
}
