package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/scope"
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
	OrgOwner       UserRoleID = "org-owner"
	OrgAdmin       UserRoleID = "org-admin"
)

func (r UserRoleID) String() string {
	return string(r)
}

type RsrcType string

func (r RsrcType) String() string {
	return string(r)
}

const (
	HostRsrc            RsrcType = "host"
	RelayRsrc           RsrcType = "relay"
	RemoteAccessGwRsrc  RsrcType = "remote_access_gw"
	GatewayRsrc         RsrcType = "gateway"
	ExtClientsRsrc      RsrcType = "extclient"
	InetGwRsrc          RsrcType = "inet_gw"
	EgressGwRsrc        RsrcType = "egress"
	NetworkRsrc         RsrcType = "network"
	EnrollmentKeysRsrc  RsrcType = "enrollment_key"
	UserRsrc            RsrcType = "user"
	AclRsrc             RsrcType = "acl"
	TagRsrc             RsrcType = "tag"
	DnsRsrc             RsrcType = "dns"
	NameserverRsrc      RsrcType = "nameserver"
	FailOverRsrc        RsrcType = "fail_over"
	MetricRsrc          RsrcType = "metric"
	PostureCheckRsrc    RsrcType = "posturecheck"
	JitAdminRsrc        RsrcType = "jit_admin"
	JitUserRsrc         RsrcType = "jit_user"
	UserActivityRsrc    RsrcType = "user_activity"
	NetworkActivityRsrc RsrcType = "network_activity"
	ActivityRsrc        RsrcType = "activity"
	TrafficFlow         RsrcType = "traffic_flow"
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
	AllHostRsrcID            RsrcID = "all_host"
	AllRelayRsrcID           RsrcID = "all_relay"
	AllRemoteAccessGwRsrcID  RsrcID = "all_remote_access_gw"
	AllExtClientsRsrcID      RsrcID = "all_extclients"
	AllInetGwRsrcID          RsrcID = "all_inet_gw"
	AllEgressGwRsrcID        RsrcID = "all_egress"
	AllNetworkRsrcID         RsrcID = "all_network"
	AllEnrollmentKeysRsrcID  RsrcID = "all_enrollment_key"
	AllUserRsrcID            RsrcID = "all_user"
	AllDnsRsrcID             RsrcID = "all_dns"
	AllFailOverRsrcID        RsrcID = "all_fail_over"
	AllAclsRsrcID            RsrcID = "all_acl"
	AllTagsRsrcID            RsrcID = "all_tag"
	AllPostureCheckRsrcID    RsrcID = "all_posturecheck"
	AllNameserverRsrcID      RsrcID = "all_nameserver"
	AllJitAdminRsrcID        RsrcID = "all_jit_admin"
	AllJitUserRsrcID         RsrcID = "all_jit_user"
	AllUserActivityRsrcID    RsrcID = "all_user_activity"
	AllNetworkActivityRsrcID RsrcID = "all_network_activity"
	AllActivityRsrcID        RsrcID = "all_activity"
	AllTrafficFlowRsrcID     RsrcID = "all_traffic_flow"
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
	OrgGlobalAccess     bool                               `json:"org_global_access"`
	TenantGlobalAccess  bool                               `json:"tenant_global_access"`
	NetworkID           NetworkID                          `json:"network_id"`
	NetworkLevelAccess  datatypes.JSONType[ResourceAccess] `json:"network_level_access"`
	GlobalLevelAccess   datatypes.JSONType[ResourceAccess] `json:"global_level_access"`
}

func (u *UserRole) TableName() string {
	return "user_roles_v1"
}

func ScopeUserRoleID(tenantID string, id UserRoleID) UserRoleID {
	if tenantID == "" || id == "" {
		return id
	}
	return UserRoleID(TenantScopedKey(tenantID, id.String()))
}

func UnscopeUserRoleID(tenantID string, id UserRoleID) UserRoleID {
	if tenantID == "" || id == "" {
		return id
	}
	return UserRoleID(StripTenantKey(tenantID, id.String()))
}

func (u *UserRole) Create(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	logicalID := u.ID
	if u.NetworkID != "" {
		u.ID = ScopeUserRoleID(tenantID, logicalID)
	}
	err := db.FromContext(ctx).Model(&UserRole{}).Create(u).Error
	u.ID = logicalID
	return err
}

func (u *UserRole) GetPlatformRole(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserRole{}).
		Where("id = ? AND network_id = ''", u.ID).
		First(u).
		Error
}

func (u *UserRole) GetNetworkRole(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	logicalID := u.ID
	err := db.FromContext(ctx).Model(&UserRole{}).
		Where("id = ? AND network_id <> ''", ScopeUserRoleID(tenantID, logicalID)).
		First(u).
		Error
	if err != nil {
		return err
	}
	u.ID = logicalID
	return nil
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
	tenantID := scope.ID(ctx)
	query := db.FromContext(ctx).Model(&UserRole{}).Where("network_id <> ''")
	if tenantID != "" {
		query = query.Where("id LIKE ?", TenantScopedKey(tenantID, "")+"%")
	}
	var userRoles []UserRole
	err := query.Find(&userRoles).Error
	for i := range userRoles {
		userRoles[i].ID = UnscopeUserRoleID(tenantID, userRoles[i].ID)
	}
	return userRoles, err
}

func (u *UserRole) Upsert(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	logicalID := u.ID
	if u.NetworkID != "" {
		u.ID = ScopeUserRoleID(tenantID, logicalID)
	}
	err := db.FromContext(ctx).Save(u).Error
	u.ID = logicalID
	return err
}

func (u *UserRole) Update(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	logicalID := u.ID
	candidates := []UserRoleID{logicalID}
	if scopedID := ScopeUserRoleID(tenantID, logicalID); scopedID != logicalID {
		candidates = append(candidates, scopedID)
	}
	// Omit "id" so this never rewrites a network role's tenant-scoped
	// physical ID down to its bare logical ID.
	err := db.FromContext(ctx).Model(&UserRole{}).
		Where("id IN ?", candidates).
		Omit("id").
		Updates(u).
		Error
	u.ID = logicalID
	return err
}

func (u *UserRole) Delete(ctx context.Context) error {
	tenantID := scope.ID(ctx)
	logicalID := u.ID
	candidates := []UserRoleID{logicalID}
	if scopedID := ScopeUserRoleID(tenantID, logicalID); scopedID != logicalID {
		candidates = append(candidates, scopedID)
	}
	return db.FromContext(ctx).Model(&UserRole{}).
		Where("id IN ?", candidates).
		Delete(u).
		Error
}

func (u *UserRole) DeleteNetworkRoles(ctx context.Context) error {
	return db.FromContext(ctx).Model(&UserRole{}).
		Where("network_id <> '' AND network_id = ?", u.NetworkID).
		Delete(u).
		Error
}

func (u *UserRole) DeleteAllForNetworks(ctx context.Context, networkIDs []string) error {
	if len(networkIDs) == 0 {
		return nil
	}
	return db.FromContext(ctx).Model(&UserRole{}).
		Where("network_id IN ?", networkIDs).
		Delete(&UserRole{}).
		Error
}
