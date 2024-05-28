package models

import (
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
)

type NetworkID string
type RsrcType string
type RsrcID string
type UserRole string

const (
	HostRsrc           RsrcType = "host"
	RelayRsrc          RsrcType = "relay"
	RemoteAccessGwRsrc RsrcType = "remote_access_gw"
	InetGwRsrc         RsrcType = "inet_gw"
	EgressGwRsrc       RsrcType = "egress"
	NetworkRsrc        RsrcType = "networks"
	EnrollmentKeysRsrc RsrcType = "enrollment_key"
	UserRsrc           RsrcType = "user"
	AclRsrc            RsrcType = "acl"
)

const (
	AllHostRsrcID           RsrcID = "all_host"
	AllRelayRsrcID          RsrcID = "all_relay"
	AllRemoteAccessGwRsrcID RsrcID = "all_remote_access_gw"
	AllInetGwRsrcID         RsrcID = "all_inet_gw"
	AllEgressGwRsrcID       RsrcID = "all_egress"
	AllNetworkRsrcID        RsrcID = "all_network"
	AllEnrollmentKeysRsrcID RsrcID = "all_enrollment_key"
	AllUserRsrcID           RsrcID = "all_user"
)

// Pre-Defined User Roles

const (
	SuperAdminRole UserRole = "super_admin"
	AdminRole      UserRole = "admin"
	NetworkAdmin   UserRole = "network_admin"
	NetworkUser    UserRole = "network_user"
)

func (r UserRole) String() string {
	return string(r)
}

type RsrcPermissionScope struct {
	Create bool `json:"create"`
	Read   bool `json:"read"`
	Update bool `json:"update"`
	Delete bool `json:"delete"`
}

type NetworkAccessControls struct {
	NetworkID                   string                                      `json:"network_id"`
	FullAccess                  bool                                        `json:"full_access"`
	NetworkRsrcPermissionsScope map[RsrcType]map[RsrcID]RsrcPermissionScope `json:"network_permissions_list"`
}

type DashboardAccessControls struct {
	FullAccess          bool                                        `json:"full_access"`
	DenyDashboardAccess bool                                        `json:"deny_dashboard_access"`
	NetworkLevelAccess  map[NetworkID]NetworkAccessControls         `json:"network_access_controls"`
	GlobalLevelAccess   map[RsrcType]map[RsrcID]RsrcPermissionScope `json:"global_level_access"`
}

type UserRolePermissionTemplate struct {
	ID            UserRole                `json:"id"`
	Default       bool                    `json:"default"`
	DashBoardAcls DashboardAccessControls `json:"dashboard_access_controls"`
}

type UserGroup struct {
	ID                 string                     `json:"id"`
	PermissionTemplate UserRolePermissionTemplate `json:"role_permission_template"`
	MetaData           string                     `json:"meta_data"`
}

// User struct - struct for Users
type User struct {
	UserName           string                     `json:"username" bson:"username" validate:"min=3,max=40,in_charset|email"`
	Password           string                     `json:"password" bson:"password" validate:"required,min=5"`
	IsAdmin            bool                       `json:"isadmin" bson:"isadmin"`
	IsSuperAdmin       bool                       `json:"issuperadmin"`
	RemoteGwIDs        map[string]struct{}        `json:"remote_gw_ids"`
	GroupID            string                     `json:"group_id"`
	PermissionTemplate UserRolePermissionTemplate `json:"role_permission_template"`
	LastLoginTime      time.Time                  `json:"last_login_time"`
}

// ReturnUser - return user struct
type ReturnUser struct {
	UserName      string              `json:"username"`
	IsAdmin       bool                `json:"isadmin"`
	IsSuperAdmin  bool                `json:"issuperadmin"`
	RemoteGwIDs   map[string]struct{} `json:"remote_gw_ids"`
	LastLoginTime time.Time           `json:"last_login_time"`
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
