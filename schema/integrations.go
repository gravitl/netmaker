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

type Integration struct {
	ID        string         `gorm:"primaryKey;column:id" json:"id"`
	TenantID  string         `gorm:"primaryKey;default:''" json:"tenant_id"`
	Type      string         `gorm:"not null;column:type"              json:"type"`
	Config    datatypes.JSON `gorm:"not null;column:config"            json:"config"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

const integrationsTable = "integrations_v1"

func (i *Integration) TableName() string {
	return integrationsTable
}

func (i *Integration) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(i).Error
}

func (i *Integration) Get(ctx context.Context) error {
	return db.FromContext(ctx).Where("id = ? AND tenant_id = ?", i.ID, scope.ID(ctx)).First(i).Error
}

func (i *Integration) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Where("id = ? AND tenant_id = ?", i.ID, scope.ID(ctx)).Delete(i).Error
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
