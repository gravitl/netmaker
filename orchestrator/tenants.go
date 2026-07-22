package orchestrator

import (
	"context"
	"errors"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
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

	return t.grantExistingOwnerAccess(ctx, tenant)
}

func (t *TenantOrchestrator) seedTenantSettings(ctx context.Context, tenant *schema.Tenant) error {
	tenantCtx := scope.WithContext(ctx, scope.TenantScope, tenant.ID)
	return logic.UpsertServerSettings(tenantCtx, logic.GetServerSettingsFromEnv())
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

	return t.GrantTenantSuperAdmin(ctx, tenant.ID, user)
}

func (t *TenantOrchestrator) GrantTenantSuperAdmin(ctx context.Context, tenantID string, user *schema.User) error {
	tenantCtx := scope.WithContext(ctx, scope.TenantScope, tenantID)
	user.PlatformRoleID = schema.SuperAdminRole
	return GetRepository().UserOrchestrator().CreateUser(tenantCtx, user, WithInheritedAuth())
}
