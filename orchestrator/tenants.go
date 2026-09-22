package orchestrator

import (
	"context"
	"errors"
	"fmt"

	chdriver "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gravitl/netmaker/clickhouse"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
	"gorm.io/gorm"
)

type TenantOrchestrator struct {
}

func (t *TenantOrchestrator) CreateDefaultTenant(ctx context.Context, orgID string) (*schema.Tenant, error) {
	tenant := &schema.Tenant{OrganizationID: orgID}
	if err := tenant.CreateDefault(ctx); err != nil {
		return nil, err
	}

	if err := t.seedTenantSettings(ctx, tenant); err != nil {
		return nil, err
	}

	t.initTenantRoles(ctx, tenant)
	t.initTenantGroups(ctx, tenant)

	if err := t.grantExistingOwnerAccess(ctx, tenant); err != nil {
		return nil, err
	}

	return tenant, nil
}

func (t *TenantOrchestrator) CreateTenant(ctx context.Context, tenant *schema.Tenant) error {
	if err := tenant.Create(ctx); err != nil {
		return err
	}

	if err := t.seedTenantSettings(ctx, tenant); err != nil {
		return err
	}

	t.initTenantRoles(ctx, tenant)
	t.initTenantGroups(ctx, tenant)

	return t.grantExistingOwnerAccess(ctx, tenant)
}

func (t *TenantOrchestrator) initTenantRoles(ctx context.Context, tenant *schema.Tenant) {
	tenantCtx := scope.WithContext(ctx, scope.TenantScope, tenant.ID)
	logic.InitialiseNetworkRoles(tenantCtx)
}

func (t *TenantOrchestrator) initTenantGroups(ctx context.Context, tenant *schema.Tenant) {
	tenantCtx := scope.WithContext(ctx, scope.TenantScope, tenant.ID)
	logic.IntialiseGroups(tenantCtx)
}

func (t *TenantOrchestrator) TeardownTenant(ctx context.Context, tenantID string) error {
	tenantCtx := scope.WithContext(ctx, scope.TenantScope, tenantID)

	nodeCount, _ := (&schema.Node{}).Count(tenantCtx)
	hostCount, _ := (&schema.Host{}).Count(tenantCtx)
	networkCount, _ := (&schema.Network{}).Count(tenantCtx)
	slog.Warn("license reports tenant deleted -- tearing down all local tenant data",
		"tenant_id", tenantID,
		"nodes", nodeCount,
		"hosts", hostCount,
		"networks", networkCount,
	)

	networks, err := (&schema.Network{}).ListAll(tenantCtx)
	if err != nil {
		return fmt.Errorf("tenant %s: failed to list networks: %w", tenantID, err)
	}
	networkIDs := make([]string, 0, len(networks))
	for _, n := range networks {
		networkIDs = append(networkIDs, n.ID)
	}

	hosts, err := (&schema.Host{}).ListAll(tenantCtx)
	if err != nil {
		return fmt.Errorf("tenant %s: failed to list hosts: %w", tenantID, err)
	}

	t.notifyHostsOfDeletion(tenantID, hosts)

	deletions := []struct {
		resource string
		delete   func(context.Context) error
	}{
		{"posture_check_violations", (&schema.PostureCheckViolation{}).DeleteAll},
		{"device_edr_state", (&schema.DeviceEDRState{}).DeleteAll},
		{"device_mdm_state", (&schema.DeviceMDMState{}).DeleteAll},
		{"nodes", (&schema.Node{}).DeleteAll},
		{"pending_hosts", (&schema.PendingHost{}).DeleteAll},
		{"hosts", (&schema.Host{}).DeleteAll},
		{"egresses", func(ctx context.Context) error { return (&schema.Egress{}).Delete(ctx) }},
		{"nameservers", func(ctx context.Context) error { return (&schema.Nameserver{}).Delete(ctx) }},
		{"dns_records", (&schema.DNSRecord{}).DeleteAll},
		{"extclient_records", (&schema.ExtClientRecord{}).DeleteAll},
		{"tag_records", (&schema.TagRecord{}).DeleteAll},
		{"acl_records", (&schema.AclRecord{}).DeleteAll},
		{"enrollment_keys", (&schema.EnrollmentKey{}).DeleteAll},
		{"posture_checks", func(ctx context.Context) error { return (&schema.PostureCheck{}).Delete(ctx) }},
		{"jit_requests", (&schema.JITRequest{}).DeleteAll},
		{"jit_grants", (&schema.JITGrant{}).DeleteAll},
		{"events", (&schema.Event{}).DeleteAllForTenant},
		{"metrics", (&schema.MetricsRecord{}).DeleteAll},
		{"integrations", (&schema.Integration{}).DeleteAll},
		{"user_access_tokens", (&schema.UserAccessToken{}).DeleteAll},
		{"user_groups", (&schema.UserGroup{}).DeleteAll},
		{"pending_users", (&schema.PendingUser{}).DeleteAll},
		{"user_invites", (&schema.UserInvite{}).DeleteAll},
		{"network_user_roles", func(ctx context.Context) error { return (&schema.UserRole{}).DeleteAllForNetworks(ctx, networkIDs) }},
		{"networks", (&schema.Network{}).DeleteAll},
	}

	for _, d := range deletions {
		if err := d.delete(tenantCtx); err != nil {
			return fmt.Errorf("tenant %s: failed to delete %s: %w", tenantID, d.resource, err)
		}
	}

	if err := t.deleteTenantFlowLogs(tenantCtx, tenantID); err != nil {
		slog.Warn("failed to delete clickhouse flow logs", "tenant_id", tenantID, "error", err)
	}

	memberships, err := (&schema.TenantMembership{TenantID: tenantID}).ListByTenantID(ctx)
	if err != nil {
		return fmt.Errorf("tenant %s: failed to list memberships: %w", tenantID, err)
	}
	if err := (&schema.TenantMembership{TenantID: tenantID}).DeleteAllByTenantID(ctx); err != nil {
		return fmt.Errorf("tenant %s: failed to delete memberships: %w", tenantID, err)
	}

	if err := (&schema.TenantSettingsRecord{Key: tenantID}).Delete(ctx); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("tenant %s: failed to delete tenant settings: %w", tenantID, err)
	}

	if err := (&schema.Tenant{ID: tenantID}).Delete(ctx); err != nil {
		return fmt.Errorf("tenant %s: failed to delete tenant row: %w", tenantID, err)
	}

	slog.Warn("tenant teardown complete", "tenant_id", tenantID, "memberships_removed", len(memberships))

	return nil
}

func (t *TenantOrchestrator) deleteTenantFlowLogs(ctx context.Context, tenantID string) error {
	if !clickhouse.IsConnected() {
		return nil
	}

	conn, err := clickhouse.FromContext(clickhouse.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("clickhouse connection not available: %w", err)
	}

	return conn.Exec(ctx,
		"ALTER TABLE {database:Identifier}.flows DELETE WHERE tenant_id = {tenantID:String}",
		chdriver.Named("database", servercfg.GetClickHouseDB()),
		chdriver.Named("tenantID", tenantID),
	)
}

func (t *TenantOrchestrator) notifyHostsOfDeletion(tenantID string, hosts []schema.Host) {
	for _, h := range hosts {
		if err := mq.HostUpdate(&models.HostUpdate{Action: models.DeleteHost, Host: h}); err != nil {
			slog.Warn("failed to send delete host update", "tenant_id", tenantID, "host_id", h.ID.String(), "error", err)
		}
		if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
			if err := mq.GetEmqxHandler().DeleteEmqxUser(h.ID.String()); err != nil {
				slog.Warn("failed to remove host credentials from EMQX", "tenant_id", tenantID, "host_id", h.ID.String(), "error", err)
			}
		}
	}
}

func (t *TenantOrchestrator) GrantTenantSuperAdmin(ctx context.Context, tenant *schema.Tenant, user *schema.User) error {
	tenantCtx := scope.WithContext(ctx, scope.TenantScope, tenant.ID)
	user.PlatformRoleID = schema.SuperAdminRole
	if t.isMSPTenant(ctx, tenant) {
		return GetRepository().UserOrchestrator().CreateUser(tenantCtx, user, WithInheritedAuth())
	}
	return GetRepository().UserOrchestrator().CreateUser(tenantCtx, user, WithReplicatedAuth())
}

func (t *TenantOrchestrator) seedTenantSettings(ctx context.Context, tenant *schema.Tenant) error {
	defaultSettings := logic.GetDefaultTenantSettings()

	if t.isMSPTenant(ctx, tenant) {
		defaultSettings.Telemetry = "off"
	}

	tenantCtx := scope.WithContext(ctx, scope.TenantScope, tenant.ID)
	return logic.UpsertServerSettings(tenantCtx, defaultSettings)
}

func (t *TenantOrchestrator) isMSPTenant(ctx context.Context, tenant *schema.Tenant) bool {
	defaultOrg := &schema.Organization{}
	err := defaultOrg.GetDefault(ctx)
	if err != nil {
		return true
	}

	return defaultOrg.ID != tenant.OrganizationID
}

func (t *TenantOrchestrator) grantExistingOwnerAccess(ctx context.Context, tenant *schema.Tenant) error {
	owner := &schema.OrgMembership{OrganizationID: tenant.OrganizationID}
	err := owner.GetOwner(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// No org owner yet. super-admin access is granted once one is
			// created, via GrantTenantSuperAdmin.
			return nil
		}
		return err
	}

	user := &schema.User{ID: owner.UserID}
	if err := user.Get(ctx); err != nil {
		return err
	}

	return t.GrantTenantSuperAdmin(ctx, tenant, user)
}
