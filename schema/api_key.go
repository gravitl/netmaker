package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
)

const apiKeysTable = "api_keys"

// APIKeyPermission is the hierarchical permission level for an API key.
type APIKeyPermission string

const (
	APIKeyPermissionRead       APIKeyPermission = "read"
	APIKeyPermissionModify     APIKeyPermission = "modify"
	APIKeyPermissionFullAccess APIKeyPermission = "full_access"
)

// API key network scope types.
const (
	APIKeyNetworkScopeAll      = "all"
	APIKeyNetworkScopeSelected = "selected"
)

// APIKey stores metadata for a tenant-scoped programmatic credential.
// The plaintext secret is never persisted; only KeyPrefix + KeyHash are stored.
type APIKey struct {
	ID               string                      `gorm:"primaryKey" json:"id"`
	TenantID         string                      `gorm:"default:'';index" json:"tenant_id"`
	Name             string                      `json:"name"`
	KeyPrefix        string                      `gorm:"uniqueIndex" json:"key_prefix"`
	KeyHash          string                      `json:"-"`
	Permission       APIKeyPermission            `json:"permission"`
	NetworkScopeType string                      `json:"network_scope_type"`
	NetworkIDs       datatypes.JSONSlice[string] `json:"network_ids"`
	CreatedBy        string                      `json:"created_by"`
	CreatedAt        time.Time                   `json:"created_at"`
	ExpiresAt        *time.Time                  `json:"expires_at,omitempty"`
	LastUsedAt       *time.Time                  `json:"last_used_at,omitempty"`
	RevokedAt        *time.Time                  `json:"revoked_at,omitempty"`
}

func (a *APIKey) TableName() string {
	return apiKeysTable
}

func (a *APIKey) Get(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&APIKey{}).Where("id = ?", a.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", apiKeysTable), tenantID)(query)
	}
	return query.First(a).Error
}

func (a *APIKey) GetByPrefix(ctx context.Context) error {
	return db.FromContext(ctx).Model(&APIKey{}).Where("key_prefix = ?", a.KeyPrefix).First(a).Error
}

func (a *APIKey) Create(ctx context.Context) error {
	return db.FromContext(ctx).Model(&APIKey{}).Create(a).Error
}

func (a *APIKey) Update(ctx context.Context) error {
	return db.FromContext(ctx).Model(&APIKey{}).Where("id = ?", a.ID).Updates(a).Error
}

func (a *APIKey) List(ctx context.Context) (keys []APIKey, err error) {
	query := db.FromContext(ctx).Model(&APIKey{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", apiKeysTable), tenantID)(query)
	}
	err = query.Order("created_at DESC").Find(&keys).Error
	if keys == nil {
		keys = []APIKey{}
	}
	return
}

func (a *APIKey) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Where(fmt.Sprintf("%s.tenant_id = ?", apiKeysTable), tenantID).Delete(&APIKey{}).Error
	}
	return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s", apiKeysTable)).Error
}
