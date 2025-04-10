package models

import (
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
)

type NetworkID string
type RsrcType string
type RsrcID string
type UserRoleID string
type UserGroupID string
type AuthType string
type TokenType string

var (
	BasicAuth AuthType = "basic_auth"
	OAuth     AuthType = "oauth"
)

func (r RsrcType) String() string {
	return string(r)
}

func (rid RsrcID) String() string {
	return string(rid)
}

func GetRAGRoleName(netID, hostName string) string {
	return fmt.Sprintf("netID-%s-rag-%s", netID, hostName)
}

func GetRAGRoleID(netID, hostID string) UserRoleID {
	return UserRoleID(fmt.Sprintf("netID-%s-rag-%s", netID, hostID))
}

func (t TokenType) String() string {
	return string(t)
}

var (
	UserIDTokenType TokenType = "user_id_token"
	AccessTokenType TokenType = "access_token"
)

var RsrcTypeMap = map[RsrcType]struct{}{
	HostRsrc:           {},
	RelayRsrc:          {},
	RemoteAccessGwRsrc: {},
	ExtClientsRsrc:     {},
	InetGwRsrc:         {},
	EgressGwRsrc:       {},
	NetworkRsrc:        {},
	EnrollmentKeysRsrc: {},
	UserRsrc:           {},
	AclRsrc:            {},
	DnsRsrc:            {},
	FailOverRsrc:       {},
}

const AllNetworks NetworkID = "all_networks"
const (
	HostRsrc           RsrcType = "hosts"
	RelayRsrc          RsrcType = "relays"
	RemoteAccessGwRsrc RsrcType = "remote_access_gw"
	ExtClientsRsrc     RsrcType = "extclients"
	InetGwRsrc         RsrcType = "inet_gw"
	EgressGwRsrc       RsrcType = "egress"
	NetworkRsrc        RsrcType = "networks"
	EnrollmentKeysRsrc RsrcType = "enrollment_key"
	UserRsrc           RsrcType = "users"
	AclRsrc            RsrcType = "acl"
	TagRsrc            RsrcType = "tag"
	DnsRsrc            RsrcType = "dns"
	FailOverRsrc       RsrcType = "fail_over"
	MetricRsrc         RsrcType = "metrics"
)

const (
	AllHostRsrcID           RsrcID = "all_host"
	AllRelayRsrcID          RsrcID = "all_relay"
	AllRemoteAccessGwRsrcID RsrcID = "all_remote_access_gw"
	AllExtClientsRsrcID     RsrcID = "all_extclients"
	AllInetGwRsrcID         RsrcID = "all_inet_gw"
	AllEgressGwRsrcID       RsrcID = "all_egress"
	AllNetworkRsrcID        RsrcID = "all_network"
	AllEnrollmentKeysRsrcID RsrcID = "all_enrollment_key"
	AllUserRsrcID           RsrcID = "all_user"
	AllDnsRsrcID            RsrcID = "all_dns"
	AllFailOverRsrcID       RsrcID = "all_fail_over"
	AllAclsRsrcID           RsrcID = "all_acl"
	AllTagsRsrcID           RsrcID = "all_tag"
)

// Pre-Defined User Roles

const (
	SuperAdminRole UserRoleID = "super-admin"
	AdminRole      UserRoleID = "admin"
	ServiceUser    UserRoleID = "service-user"
	PlatformUser   UserRoleID = "platform-user"
	NetworkAdmin   UserRoleID = "network-admin"
	NetworkUser    UserRoleID = "network-user"
)

func (r UserRoleID) String() string {
	return string(r)
}

func (g UserGroupID) String() string {
	return string(g)
}

func (n NetworkID) String() string {
	return string(n)
}

type RsrcPermissionScope struct {
	Create    bool `json:"create"`
	Read      bool `json:"read"`
	Update    bool `json:"update"`
	Delete    bool `json:"delete"`
	VPNaccess bool `json:"vpn_access"`
	SelfOnly  bool `json:"self_only"`
}

type UserRolePermissionTemplate struct {
	ID                  UserRoleID                                  `json:"id"`
	Name                string                                      `json:"name"`
	Default             bool                                        `json:"default"`
	MetaData            string                                      `json:"meta_data"`
	DenyDashboardAccess bool                                        `json:"deny_dashboard_access"`
	FullAccess          bool                                        `json:"full_access"`
	NetworkID           NetworkID                                   `json:"network_id"`
	NetworkLevelAccess  map[RsrcType]map[RsrcID]RsrcPermissionScope `json:"network_level_access"`
	GlobalLevelAccess   map[RsrcType]map[RsrcID]RsrcPermissionScope `json:"global_level_access"`
}

type CreateGroupReq struct {
	Group   UserGroup `json:"user_group"`
	Members []string  `json:"members"`
}

type UserGroup struct {
	ID           UserGroupID                           `json:"id"`
	Default      bool                                  `json:"default"`
	Name         string                                `json:"name"`
	NetworkRoles map[NetworkID]map[UserRoleID]struct{} `json:"network_roles"`
	MetaData     string                                `json:"meta_data"`
}

// User struct - struct for Users
type User struct {
	UserName                   string                                `json:"username" bson:"username" validate:"min=3,in_charset|email"`
	ExternalIdentityProviderID string                                `json:"external_identity_provider_id"`
	Password                   string                                `json:"password" bson:"password" validate:"required,min=5"`
	IsAdmin                    bool                                  `json:"isadmin" bson:"isadmin"` // deprecated
	IsSuperAdmin               bool                                  `json:"issuperadmin"`           // deprecated
	RemoteGwIDs                map[string]struct{}                   `json:"remote_gw_ids"`          // deprecated
	AuthType                   AuthType                              `json:"auth_type"`
	UserGroups                 map[UserGroupID]struct{}              `json:"user_group_ids"`
	PlatformRoleID             UserRoleID                            `json:"platform_role_id"`
	NetworkRoles               map[NetworkID]map[UserRoleID]struct{} `json:"network_roles"`
	LastLoginTime              time.Time                             `json:"last_login_time"`
}

type ReturnUserWithRolesAndGroups struct {
	ReturnUser
	PlatformRole UserRolePermissionTemplate `json:"platform_role"`
	UserGroups   map[UserGroupID]UserGroup  `json:"user_group_ids"`
}

// ReturnUser - return user struct
type ReturnUser struct {
	UserName       string                                `json:"username"`
	IsAdmin        bool                                  `json:"isadmin"`
	IsSuperAdmin   bool                                  `json:"issuperadmin"`
	AuthType       AuthType                              `json:"auth_type"`
	RemoteGwIDs    map[string]struct{}                   `json:"remote_gw_ids"` // deprecated
	UserGroups     map[UserGroupID]struct{}              `json:"user_group_ids"`
	PlatformRoleID UserRoleID                            `json:"platform_role_id"`
	NetworkRoles   map[NetworkID]map[UserRoleID]struct{} `json:"network_roles"`
	LastLoginTime  time.Time                             `json:"last_login_time"`
}

// UserAuthParams - user auth params struct
type UserAuthParams struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

// UserClaims - user claims struct
type UserClaims struct {
	Role           UserRoleID
	UserName       string
	Api            string
	TokenType      TokenType
	RacAutoDisable bool
	jwt.RegisteredClaims
}

type InviteUsersReq struct {
	UserEmails     []string                              `json:"user_emails"`
	PlatformRoleID string                                `json:"platform_role_id"`
	UserGroups     map[UserGroupID]struct{}              `json:"user_group_ids"`
	NetworkRoles   map[NetworkID]map[UserRoleID]struct{} `json:"network_roles"`
}

// UserInvite - model for user invite
type UserInvite struct {
	Email          string                                `json:"email"`
	PlatformRoleID string                                `json:"platform_role_id"`
	UserGroups     map[UserGroupID]struct{}              `json:"user_group_ids"`
	NetworkRoles   map[NetworkID]map[UserRoleID]struct{} `json:"network_roles"`
	InviteCode     string                                `json:"invite_code"`
	InviteURL      string                                `json:"invite_url"`
}
