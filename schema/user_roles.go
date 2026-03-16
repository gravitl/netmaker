package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type UserRoleID string

const (
	SuperAdminRole UserRoleID = "super-admin"
	AdminRole      UserRoleID = "admin"
	ServiceUser    UserRoleID = "service-user"
	PlatformUser   UserRoleID = "platform-user"
	Auditor        UserRoleID = "auditor"
	NetworkAdmin   UserRoleID = "network-admin"
	NetworkUser    UserRoleID = "network-user"
)

func (r UserRoleID) String() string {
	return string(r)
}

type RsrcType string

func (r RsrcType) String() string {
	return string(r)
}

const (
	HostRsrc           RsrcType = "host"
	RelayRsrc          RsrcType = "relay"
	RemoteAccessGwRsrc RsrcType = "remote_access_gw"
	GatewayRsrc        RsrcType = "gateway"
	ExtClientsRsrc     RsrcType = "extclient"
	InetGwRsrc         RsrcType = "inet_gw"
	EgressGwRsrc       RsrcType = "egress"
	NetworkRsrc        RsrcType = "network"
	EnrollmentKeysRsrc RsrcType = "enrollment_key"
	UserRsrc           RsrcType = "user"
	AclRsrc            RsrcType = "acl"
	TagRsrc            RsrcType = "tag"
	DnsRsrc            RsrcType = "dns"
	NameserverRsrc     RsrcType = "nameserver"
	FailOverRsrc       RsrcType = "fail_over"
	MetricRsrc         RsrcType = "metric"
	PostureCheckRsrc   RsrcType = "posturecheck"
	JitAdminRsrc       RsrcType = "jit_admin"
	JitUserRsrc        RsrcType = "jit_user"
	UserActivityRsrc   RsrcType = "user_activity"
	TrafficFlow        RsrcType = "traffic_flow"
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

type RsrcID string

func (rid RsrcID) String() string {
	return string(rid)
}

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
	AllPostureCheckRsrcID   RsrcID = "all_posturecheck"
	AllNameserverRsrcID     RsrcID = "all_nameserver"
	AllJitAdminRsrcID       RsrcID = "all_jit_admin"
	AllJitUserRsrcID        RsrcID = "all_jit_user"
	AllUserActivityRsrcID   RsrcID = "all_user_activity"
	AllTrafficFlowRsrcID    RsrcID = "all_traffic_flow"
)

type RsrcPermissionScope struct {
	Create    bool `json:"create"`
	Read      bool `json:"read"`
	Update    bool `json:"update"`
	Delete    bool `json:"delete"`
	VPNaccess bool `json:"vpn_access"`
	SelfOnly  bool `json:"self_only"`
}

type ResourceAccess map[RsrcType]map[RsrcID]RsrcPermissionScope

type UserRole struct {
	ID                  UserRoleID                         `gorm:"primaryKey" json:"id"`
	Name                string                             `json:"name"`
	Default             bool                               `json:"default"`
	MetaData            string                             `json:"meta_data"`
	DenyDashboardAccess bool                               `json:"deny_dashboard_access"`
	FullAccess          bool                               `json:"full_access"`
	NetworkID           NetworkID                          `json:"network_id"`
	NetworkLevelAccess  datatypes.JSONType[ResourceAccess] `json:"network_level_access"`
	GlobalLevelAccess   datatypes.JSONType[ResourceAccess] `json:"global_level_access"`
}

func (u *UserRole) TableName() string {
	return "user_roles_v1"
}

func (u *UserRole) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserRole{}).Create(u).Error
}

func (u *UserRole) Exists(ctx context.Context) (bool, error) {
	var exists bool
	err := db.FromContext(ctx).Raw(
		"SELECT EXISTS (SELECT 1 FROM user_roles_v1 WHERE name = ?)",
		u.Name,
	).Scan(&exists).Error
	return exists, err
}

func (u *UserRole) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserRole{}).
		Where("id = ?", u.ID).
		First(u).
		Error
}

func (u *UserRole) ListPlatformRoles(ctx context.Context) ([]UserRole, error) {
	var userRoles []UserRole
	err := db.FromContext(ctx).Model(&UserRole{}).
		Where("network_id = ''").
		Find(&userRoles).
		Error
	return userRoles, err
}

func (u *UserRole) ListNetworkRoles(ctx context.Context) ([]UserRole, error) {
	var userRoles []UserRole
	err := db.FromContext(ctx).Model(&UserRole{}).
		Where("network_id <> ''").
		Find(&userRoles).
		Error
	return userRoles, err
}

func (u *UserRole) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(u).Error
}

func (u *UserRole) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserRole{}).
		Where("id = ?", u.ID).
		Updates(u).
		Error
}

func (u *UserRole) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserRole{}).
		Where("id = ?", u.ID).
		Delete(u).
		Error
}

func (u *UserRole) DeleteNetworkRoles(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserRole{}).
		Where("network_id <> '' AND network_id = ?", u.NetworkID).
		Delete(u).
		Error
}
