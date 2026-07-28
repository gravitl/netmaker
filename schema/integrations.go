package schema

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Integration struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Provider  string         `gorm:"default:'';uniqueIndex:idx_integrations_provider_tenant" json:"provider"`
	TenantID  string         `gorm:"default:'';uniqueIndex:idx_integrations_provider_tenant" json:"tenant_id"`
	Type      string         `gorm:"not null" json:"type"`
	Config    datatypes.JSON `gorm:"not null" json:"config"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

const integrationsTable = "integrations_v1"

func (i *Integration) TableName() string {
	return integrationsTable
}

func (i *Integration) Upsert(ctx context.Context) error {
	if i.ID == "" {
		var existing Integration
		err := db.FromContext(ctx).
			Where("provider = ? AND tenant_id = ?", i.Provider, i.TenantID).
			First(&existing).Error
		switch {
		case err == nil:
			i.ID = existing.ID
		case errors.Is(err, gorm.ErrRecordNotFound):
			i.ID = uuid.NewString()
		default:
			return err
		}
	}
	return db.FromContext(ctx).Save(i).Error
}

func (i *Integration) Get(ctx context.Context) error {
	return db.FromContext(ctx).Where("provider = ? AND tenant_id = ?", i.Provider, scope.ID(ctx)).First(i).Error
}

func (i *Integration) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Where("provider = ? AND tenant_id = ?", i.Provider, scope.ID(ctx)).Delete(i).Error
}

func (i *Integration) ListByType(ctx context.Context) ([]Integration, error) {
	var integrations []Integration
	query := db.FromContext(ctx).Model(&Integration{}).
		Where("type = ?", i.Type)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", integrationsTable), tenantID)(query)
	}
	err := query.Find(&integrations).Error
	return integrations, err
}
