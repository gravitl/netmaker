package models

import (
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
)

type NetworkID string
type RsrcType string
type RsrcID string
type UserRole string
type UserGroupID string

func (r RsrcType) String() string {
	return string(r)
}

func (rid RsrcID) String() string {
	return string(rid)
}

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
	DnsRsrc            RsrcType = "dns"
	FailOverRsrc       RsrcType = "fail_over"
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
	AllAclsRsrcID           RsrcID = "all_acls"
)

// Pre-Defined User Roles

const (
	SuperAdminRole UserRole = "super_admin"
	AdminRole      UserRole = "admin"
	ServiceUser    UserRole = "user"
	NetworkAdmin   UserRole = "network_admin"
	NetworkUser    UserRole = "network_user"
)

func (r UserRole) String() string {
	return string(r)
}

func (g UserGroupID) String() string {
	return string(g)
}

type RsrcPermissionScope struct {
	Create    bool `json:"create"`
	Read      bool `json:"read"`
	Update    bool `json:"update"`
	Delete    bool `json:"delete"`
	VPNaccess bool `json:"vpn_access"`
}

type UserRolePermissionTemplate struct {
	ID                  UserRole                                    `json:"id"`
	Default             bool                                        `json:"default"`
	DenyDashboardAccess bool                                        `json:"deny_dashboard_access"`
	FullAccess          bool                                        `json:"full_access"`
	NetworkID           string                                      `json:"network_id"`
	NetworkLevelAccess  map[RsrcType]map[RsrcID]RsrcPermissionScope `json:"network_level_access"`
	GlobalLevelAccess   map[RsrcType]map[RsrcID]RsrcPermissionScope `json:"global_level_access"`
}

type UserGroup struct {
	ID           string                 `json:"id"`
	NetworkRoles map[NetworkID]UserRole `json:"network_roles"`
	MetaData     string                 `json:"meta_data"`
}

// User struct - struct for Users
type User struct {
	UserName       string                              `json:"username" bson:"username" validate:"min=3,max=40,in_charset|email"`
	Password       string                              `json:"password" bson:"password" validate:"required,min=5"`
	IsAdmin        bool                                `json:"isadmin" bson:"isadmin"`
	IsSuperAdmin   bool                                `json:"issuperadmin"`
	RemoteGwIDs    map[string]struct{}                 `json:"remote_gw_ids"`
	UserGroups     map[UserGroupID]struct{}            `json:"user_group_ids"`
	PlatformRoleID UserRole                            `json:"platform_role_id"`
	NetworkRoles   map[NetworkID]map[UserRole]struct{} `json:"network_roles"`
	LastLoginTime  time.Time                           `json:"last_login_time"`
}

// ReturnUser - return user struct
type ReturnUser struct {
	UserName       string                   `json:"username"`
	IsAdmin        bool                     `json:"isadmin"`
	IsSuperAdmin   bool                     `json:"issuperadmin"`
	RemoteGwIDs    map[string]struct{}      `json:"remote_gw_ids"`
	UserGroups     map[UserGroupID]struct{} `json:"user_group_ids"`
	PlatformRoleID string                   `json:"platform_role_id"`
	NetworkRoles   map[NetworkID]UserRole   `json:"network_roles"`
	LastLoginTime  time.Time                `json:"last_login_time"`
}

// UserAuthParams - user auth params struct
type UserAuthParams struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

// UserClaims - user claims struct
type UserClaims struct {
	IsAdmin      bool
	IsSuperAdmin bool
	UserName     string
	jwt.RegisteredClaims
}
