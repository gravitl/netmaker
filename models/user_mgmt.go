package models

import (
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gravitl/netmaker/schema"
)

type TokenType string

func GetRAGRoleName(netID, hostName string) string {
	return fmt.Sprintf("netID-%s-rag-%s", netID, hostName)
}

func GetRAGRoleID(netID, hostID string) schema.UserRoleID {
	return schema.UserRoleID(fmt.Sprintf("netID-%s-rag-%s", netID, hostID))
}

func (t TokenType) String() string {
	return string(t)
}

var (
	UserIDTokenType TokenType = "user_id_token"
	AccessTokenType TokenType = "access_token"
)

// Pre-Defined User Roles

// User struct - struct for Users
type User struct {
	UserName                   string                                              `json:"username" bson:"username" validate:"min=3,in_charset|email"`
	ExternalIdentityProviderID string                                              `json:"external_identity_provider_id"`
	IsMFAEnabled               bool                                                `json:"is_mfa_enabled"`
	TOTPSecret                 string                                              `json:"totp_secret"`
	DisplayName                string                                              `json:"display_name"`
	AccountDisabled            bool                                                `json:"account_disabled"`
	Password                   string                                              `json:"password" bson:"password" validate:"required,min=5"`
	IsAdmin                    bool                                                `json:"isadmin" bson:"isadmin"` // deprecated
	IsSuperAdmin               bool                                                `json:"issuperadmin"`           // deprecated
	RemoteGwIDs                map[string]struct{}                                 `json:"remote_gw_ids"`          // deprecated
	AuthType                   schema.AuthType                                     `json:"auth_type"`
	UserGroups                 map[schema.UserGroupID]struct{}                     `json:"user_group_ids"`
	PlatformRoleID             schema.UserRoleID                                   `json:"platform_role_id"`
	NetworkRoles               map[schema.NetworkID]map[schema.UserRoleID]struct{} `json:"network_roles"`
	LastLoginTime              time.Time                                           `json:"last_login_time"`
	CreatedBy                  string                                              `json:"created_by"`
	CreatedAt                  time.Time                                           `json:"created_at"`
	UpdatedAt                  time.Time                                           `json:"updated_at"`
}

// ReturnUser - return user struct
type ReturnUser struct {
	UserName                   string                                              `json:"username"`
	ExternalIdentityProviderID string                                              `json:"external_identity_provider_id"`
	IsMFAEnabled               bool                                                `json:"is_mfa_enabled"`
	DisplayName                string                                              `json:"display_name"`
	AccountDisabled            bool                                                `json:"account_disabled"`
	IsAdmin                    bool                                                `json:"isadmin"`
	IsSuperAdmin               bool                                                `json:"issuperadmin"`
	AuthType                   schema.AuthType                                     `json:"auth_type"`
	RemoteGwIDs                map[string]struct{}                                 `json:"remote_gw_ids"` // deprecated
	UserGroups                 map[schema.UserGroupID]struct{}                     `json:"user_group_ids"`
	PlatformRoleID             schema.UserRoleID                                   `json:"platform_role_id"`
	NetworkRoles               map[schema.NetworkID]map[schema.UserRoleID]struct{} `json:"network_roles"`
	LastLoginTime              time.Time                                           `json:"last_login_time"`
	NumAccessTokens            int                                                 `json:"num_access_tokens"`
	CreatedBy                  string                                              `json:"created_by"`
	CreatedAt                  time.Time                                           `json:"created_at"`
	UpdatedAt                  time.Time                                           `json:"updated_at"`
}

// UserAuthParams - user auth params struct
type UserAuthParams struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

// UserIdentityValidationRequest - user identity validation request struct
type UserIdentityValidationRequest struct {
	Password string `json:"password"`
}

// UserIdentityValidationResponse - user identity validation response struct
type UserIdentityValidationResponse struct {
	IdentityValidated bool `json:"identity_validated"`
}

type UserTOTPVerificationParams struct {
	OTPAuthURL          string `json:"otp_auth_url"`
	OTPAuthURLSignature string `json:"otp_auth_url_signature"`
	TOTP                string `json:"totp"`
}

// UserClaims - user claims struct
type UserClaims struct {
	Role           schema.UserRoleID
	UserName       string
	Api            string
	TokenType      TokenType
	RacAutoDisable bool
	jwt.RegisteredClaims
}

type InviteUsersReq struct {
	UserEmails     []string                                            `json:"user_emails"`
	PlatformRoleID string                                              `json:"platform_role_id"`
	UserGroups     map[schema.UserGroupID]struct{}                     `json:"user_group_ids"`
	NetworkRoles   map[schema.NetworkID]map[schema.UserRoleID]struct{} `json:"network_roles"`
}

// UserInvite - model for user invite
type UserInvite struct {
	Email          string                                              `json:"email"`
	PlatformRoleID string                                              `json:"platform_role_id"`
	UserGroups     map[schema.UserGroupID]struct{}                     `json:"user_group_ids"`
	NetworkRoles   map[schema.NetworkID]map[schema.UserRoleID]struct{} `json:"network_roles"`
	InviteCode     string                                              `json:"invite_code"`
	InviteURL      string                                              `json:"invite_url"`
}

// UserMapping - user ip map with groups
type UserMapping struct {
	User   string   `json:"user"`
	Groups []string `json:"groups"`
}

// UserIPMap maintains the mapping of IP addresses to users and groups
type UserIPMap struct {
	Mappings map[string]UserMapping `json:"mappings"`
}
