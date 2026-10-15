package schema

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
	"gorm.io/gorm"
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
	ID string `gorm:"primaryKey" json:"-"`
	// Scope is global for org and platform roles, which are shared by all the
	// orgs and tenants, and tenant for network roles.
	Scope scope.Scope `gorm:"default:0;uniqueIndex:udx_user_role_scope_slug" json:"-"`
	// ScopeID is the tenant of network roles, empty for global roles.
	ScopeID string `gorm:"default:'';index;uniqueIndex:udx_user_role_scope_slug" json:"-"`
	// Slug is the id the role is referred to by, e.g. super-admin for platform
	// roles, or <network>-network-admin for default network roles. It's
	// nullable in the database, since it's added to existing tables before
	// being populated by the v1.8.0 migration.
	Slug                UserRoleID                         `gorm:"uniqueIndex:udx_user_role_scope_slug" json:"id"`
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

const userRolesTable = "user_roles_v1"

var ErrUserRoleIdentifiersNotProvided = errors.New("user role identifiers not provided")

func (u *UserRole) TableName() string {
	return userRolesTable
}

// setScope scopes network roles to the tenant in the context, and org and
// platform roles globally.
func (u *UserRole) setScope(ctx context.Context) {
	if u.NetworkID != "" {
		u.Scope = scope.TenantScope
		u.ScopeID = scope.ID(ctx)
		return
	}
	u.Scope = scope.GlobalScope
	u.ScopeID = ""
}

// platformRoleQuery scopes the query to the org or platform role identified
// by its ID, or by its slug.
func (u *UserRole) platformRoleQuery(ctx context.Context) (*gorm.DB, error) {
	query := db.FromContext(ctx).Model(&UserRole{}).
		Where(fmt.Sprintf("%s.network_id = '' AND %s.scope = ? AND %s.scope_id = ''", userRolesTable, userRolesTable, userRolesTable), scope.GlobalScope)
	if u.ID != "" {
		return query.Where(fmt.Sprintf("%s.id = ?", userRolesTable), u.ID), nil
	}
	if u.Slug == "" {
		return nil, ErrUserRoleIdentifiersNotProvided
	}
	return query.Where(fmt.Sprintf("%s.slug = ?", userRolesTable), u.Slug), nil
}

// networkRoleQuery scopes the query to the network role identified by its
// ID, or by its slug within the tenant in the context.
func (u *UserRole) networkRoleQuery(ctx context.Context) (*gorm.DB, error) {
	tenantID := scope.ID(ctx)
	query := db.FromContext(ctx).Model(&UserRole{}).
		Where(fmt.Sprintf("%s.network_id <> '' AND %s.scope = ?", userRolesTable, userRolesTable), scope.TenantScope)
	if u.ID != "" {
		query = query.Where(fmt.Sprintf("%s.id = ?", userRolesTable), u.ID)
		if tenantID != "" {
			query = query.Where(fmt.Sprintf("%s.scope_id = ?", userRolesTable), tenantID)
		}
		return query, nil
	}
	if u.Slug == "" {
		return nil, ErrUserRoleIdentifiersNotProvided
	}
	return query.Where(fmt.Sprintf("%s.scope_id = ? AND %s.slug = ?", userRolesTable, userRolesTable), tenantID, u.Slug), nil
}

// Create creates the role, scoped to the tenant in the context if it's a
// network role. The ID defaults to a new uuid, and the slug to the ID.
func (u *UserRole) Create(ctx context.Context) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	if u.Slug == "" {
		u.Slug = UserRoleID(u.ID)
	}
	u.setScope(ctx)
	return db.FromContext(ctx).Model(&UserRole{}).Create(u).Error
}

// GetPlatformRole fetches the platform role by its ID, or by its slug.
func (u *UserRole) GetPlatformRole(ctx context.Context) error {
	query, err := u.platformRoleQuery(ctx)
	if err != nil {
		return err
	}

	var role UserRole
	if err := query.First(&role).Error; err != nil {
		return err
	}

	*u = role
	return nil
}

// GetNetworkRole fetches the network role by its ID, or by its slug within
// the tenant in the context.
func (u *UserRole) GetNetworkRole(ctx context.Context) error {
	query, err := u.networkRoleQuery(ctx)
	if err != nil {
		return err
	}

	var role UserRole
	if err := query.First(&role).Error; err != nil {
		return err
	}

	*u = role
	return nil
}

func (u *UserRole) ListPlatformRoles(ctx context.Context) ([]UserRole, error) {
	var userRoles []UserRole
	err := db.FromContext(ctx).Model(&UserRole{}).
		Where(fmt.Sprintf("%s.network_id = ''", userRolesTable)).
		Find(&userRoles).
		Error
	return userRoles, err
}

// ListNetworkRoles lists the network roles of the tenant in the context, or
// of all the tenants if there's none.
func (u *UserRole) ListNetworkRoles(ctx context.Context) ([]UserRole, error) {
	query := db.FromContext(ctx).Model(&UserRole{}).Where(fmt.Sprintf("%s.network_id <> ''", userRolesTable))
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = query.Where(fmt.Sprintf("%s.scope_id = ?", userRolesTable), tenantID)
	}
	var userRoles []UserRole
	err := query.Find(&userRoles).Error
	return userRoles, err
}

// Upsert overwrites the role identified by its ID, or by its slug (within
// the tenant in the context for network roles), creating it if it does not
// exist.
func (u *UserRole) Upsert(ctx context.Context) error {
	if u.ID == "" {
		existing := &UserRole{Slug: u.Slug, NetworkID: u.NetworkID}
		var err error
		if u.NetworkID == "" {
			err = existing.GetPlatformRole(ctx)
		} else {
			err = existing.GetNetworkRole(ctx)
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err != nil {
			return u.Create(ctx)
		}
		u.ID = existing.ID
	}
	if u.Slug == "" {
		u.Slug = UserRoleID(u.ID)
	}
	u.setScope(ctx)
	return db.FromContext(ctx).Save(u).Error
}

// DeleteNetworkRole deletes the network role identified by its ID, or by its
// slug within the tenant in the context.
func (u *UserRole) DeleteNetworkRole(ctx context.Context) error {
	query, err := u.networkRoleQuery(ctx)
	if err != nil {
		return err
	}
	return query.Delete(&UserRole{}).Error
}

// DeleteNetworkRoles deletes the roles of the network, within the tenant in
// the context.
func (u *UserRole) DeleteNetworkRoles(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&UserRole{}).
		Where(fmt.Sprintf("%s.network_id <> '' AND %s.network_id = ?", userRolesTable, userRolesTable), u.NetworkID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = query.Where(fmt.Sprintf("%s.scope_id = ?", userRolesTable), tenantID)
	}
	return query.Delete(&UserRole{}).Error
}

// DeleteAllForNetworks deletes the roles of the networks, within the tenant
// in the context.
func (u *UserRole) DeleteAllForNetworks(ctx context.Context, networkIDs []string) error {
	if len(networkIDs) == 0 {
		return nil
	}
	query := db.FromContext(ctx).Model(&UserRole{}).Where(fmt.Sprintf("%s.network_id IN ?", userRolesTable), networkIDs)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = query.Where(fmt.Sprintf("%s.scope_id = ?", userRolesTable), tenantID)
	}
	return query.Delete(&UserRole{}).Error
}
