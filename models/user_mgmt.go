package models

import (
	"fmt"
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

func GetRAGRoleName(netID, hostName string) UserRole {
	return UserRole(fmt.Sprintf("netID-%s-rag-%s", netID, hostName))
}

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
	AllAclsRsrcID           RsrcID = "all_acls"
)

// Pre-Defined User Roles

const (
	SuperAdminRole UserRole = "super_admin"
	AdminRole      UserRole = "admin"
	ServiceUser    UserRole = "service_user"
	PlatformUser   UserRole = "platform_user"
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
	SelfOnly  bool `json:"self_only"`
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

type CreateGroupReq struct {
	Group   UserGroup `json:"user_group"`
	Members []string  `json:"members"`
}

type UserGroup struct {
	ID           UserGroupID                         `json:"id"`
	PlatformRole UserRole                            `json:"platform_role"`
	NetworkRoles map[NetworkID]map[UserRole]struct{} `json:"network_roles"`
	MetaData     string                              `json:"meta_data"`
}

// User struct - struct for Users
type User struct {
	UserName       string                              `json:"username" bson:"username" validate:"min=3,max=40,in_charset|email"`
	Password       string                              `json:"password" bson:"password" validate:"required,min=5"`
	IsAdmin        bool                                `json:"isadmin" bson:"isadmin"` // deprecated
	IsSuperAdmin   bool                                `json:"issuperadmin"`           // deprecated
	RemoteGwIDs    map[string]struct{}                 `json:"remote_gw_ids"`          // deprecated
	UserGroups     map[UserGroupID]struct{}            `json:"user_group_ids"`
	PlatformRoleID UserRole                            `json:"platform_role_id"`
	NetworkRoles   map[NetworkID]map[UserRole]struct{} `json:"network_roles"`
	LastLoginTime  time.Time                           `json:"last_login_time"`
}

type ReturnUserWithRolesAndGroups struct {
	ReturnUser
	PlatformRole UserRolePermissionTemplate `json:"platform_role"`
}

// ReturnUser - return user struct
type ReturnUser struct {
	UserName       string                              `json:"username"`
	IsAdmin        bool                                `json:"isadmin"`
	IsSuperAdmin   bool                                `json:"issuperadmin"`
	RemoteGwIDs    map[string]struct{}                 `json:"remote_gw_ids"` // deprecated
	UserGroups     map[UserGroupID]struct{}            `json:"user_group_ids"`
	PlatformRoleID UserRole                            `json:"platform_role_id"`
	NetworkRoles   map[NetworkID]map[UserRole]struct{} `json:"network_roles"`
	LastLoginTime  time.Time                           `json:"last_login_time"`
}

// UserAuthParams - user auth params struct
type UserAuthParams struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

// UserClaims - user claims struct
type UserClaims struct {
	Role     UserRole
	UserName string
	jwt.RegisteredClaims
}

type InviteUsersReq struct {
	UserEmails []string `json:"user_emails"`
	Groups     []UserGroupID
}

// UserInvite - model for user invite
type UserInvite struct {
	Email      string        `json:"email"`
	Groups     []UserGroupID `json:"groups"`
	InviteCode string        `json:"invite_code"`
	InviteURL  string        `json:"invite_url"`
}
