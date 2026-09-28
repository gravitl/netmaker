package migrate

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/middleware"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
)

var MigrateOrgAndTenants = migrateOrgAndTenants

func initializeTenants(ctx context.Context) error {
	return MigrateOrgAndTenants(ctx)
}

func migrateOrgAndTenants(ctx context.Context) error {
	org, err := EnsureLocalOrganization(ctx)
	if err != nil {
		return err
	}

	_, err = EnsureLocalTenant(ctx, org.ID)
	return err
}

func EnsureLocalOrganization(ctx context.Context) (*schema.Organization, error) {
	orgs, err := (&schema.Organization{}).ListAll(ctx)
	if err != nil {
		return nil, err
	}
	if len(orgs) > 0 {
		return &orgs[0], nil
	}

	org := &schema.Organization{}
	if err := org.CreateDefault(ctx); err != nil {
		return nil, err
	}
	return org, nil
}

func EnsureLocalTenant(ctx context.Context, orgID string) (*schema.Tenant, error) {
	tenants, err := (&schema.Tenant{}).List(ctx)
	if err != nil {
		return nil, err
	}
	if len(tenants) > 0 {
		return &tenants[0], nil
	}

	return orchestrator.GetRepository().TenantOrchestrator().CreateDefaultTenant(ctx, orgID)
}

func tenantScopedModels() []any {
	return []any{
		&schema.AclRecord{}, &schema.DNSEntry{}, &schema.Nameserver{}, &schema.Egress{},
		&schema.EnrollmentKey{}, &schema.Event{}, &schema.Extclient{},
		&schema.Host{}, &schema.Integration{}, &schema.JITGrant{}, &schema.JITRequest{},
		&schema.MetricsRecord{}, &schema.Network{}, &schema.Node{}, &schema.PendingHost{},
		&schema.PostureCheck{}, &schema.PostureCheckViolation{},
		&schema.Tag{}, &schema.UserAccessToken{}, &schema.UserGroup{},
	}
}

func scopedModels() []any {
	return []any{&schema.PendingUser{}, &schema.UserInvite{}}
}

func rekeyTenantScopedKeys(ctx context.Context, oldID, newID string) error {
	if err := db.FromContext(ctx).Model(&schema.TenantSettingsRecord{}).
		Where("key = ?", oldID).
		Update("key", newID).Error; err != nil {
		return err
	}

	// only network roles are scoped to a tenant, org and platform roles are
	// shared by all the orgs and tenants.
	return db.FromContext(ctx).Model(&schema.UserRole{}).
		Where("network_id <> '' AND scope = ? AND scope_id = ?", scope.TenantScope, oldID).
		Update("scope_id", newID).Error
}

func RekeyTenant(ctx context.Context, oldID, newID string) error {
	if oldID == newID {
		return nil
	}

	if err := rekeyTenantScopedKeys(ctx, oldID, newID); err != nil {
		return err
	}

	models := append(tenantScopedModels(), &schema.TenantMembership{})
	for _, model := range models {
		if err := db.FromContext(ctx).Model(model).
			Where("tenant_id = ?", oldID).
			Update("tenant_id", newID).Error; err != nil {
			return err
		}
	}

	for _, model := range scopedModels() {
		if err := db.FromContext(ctx).Model(model).
			Where("scope = ? AND scope_id = ?", scope.TenantScope, oldID).
			Update("scope_id", newID).Error; err != nil {
			return err
		}
	}

	if err := db.FromContext(ctx).Model(&schema.Tenant{}).
		Where("id = ?", oldID).
		Update("id", newID).Error; err != nil {
		return err
	}

	middleware.ResetDefaultTenantID(oldID, newID)
	return nil
}

func RekeyOrganization(ctx context.Context, oldID, newID string) error {
	if oldID == newID {
		return nil
	}

	if err := db.FromContext(ctx).Model(&schema.Tenant{}).
		Where("organization_id = ?", oldID).
		Update("organization_id", newID).Error; err != nil {
		return err
	}

	if err := db.FromContext(ctx).Model(&schema.OrgMembership{}).
		Where("organization_id = ?", oldID).
		Update("organization_id", newID).Error; err != nil {
		return err
	}

	for _, model := range scopedModels() {
		if err := db.FromContext(ctx).Model(model).
			Where("scope = ? AND scope_id = ?", scope.OrgScope, oldID).
			Update("scope_id", newID).Error; err != nil {
			return err
		}
	}

	return db.FromContext(ctx).Model(&schema.Organization{}).
		Where("id = ?", oldID).
		Update("id", newID).Error
}

func isNewDeployment(ctx context.Context) (bool, error) {
	numUsers, err := (&schema.User{}).Count(ctx)
	if err != nil {
		return false, err
	}

	if numUsers == 0 {
		return true, nil
	}

	return false, nil
}
