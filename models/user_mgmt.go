package models

import (
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
)

type NetworkID string
type RsrcID string
type UserRole string

const (
	HostRsrcID           RsrcID = "all_host"
	RelayRsrcID          RsrcID = "all_relay"
	RemoteAccessGwRsrcID RsrcID = "all_remote_access_gw"
	InetGwRsrcID         RsrcID = "all_inet_gw"
	EgressGwRsrcID       RsrcID = "all_egress"
	NetworkRsrcID        RsrcID = "all_network"
	EnrollmentKeysRsrcID RsrcID = "all_enrollment_key"
	UserRsrcID           RsrcID = "all_user"
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

type RsrcPermissions struct {
	Create bool `json:"create"`
	Read   bool `json:"read"`
	Update bool `json:"update"`
	Delete bool `json:"delete"`
}

type NetworkAccessControls struct {
	NetworkID                  string                     `json:"network_id"`
	FullAccess                 bool                       `json:"full_access"`
	NetworkRsrcPermissionsList map[RsrcID]RsrcPermissions `json:"network_permissions_list"`
}

type DashboardAccessControls struct {
	FullAccess          bool                                `json:"full_access"`
	DenyDashboardAccess bool                                `json:"deny_dashboard_access"`
	NetworkLevelAccess  map[NetworkID]NetworkAccessControls `json:"network_access_controls"`
	GlobalLevelAccess   map[RsrcID]RsrcPermissions          `json:"global_level_access"`
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
