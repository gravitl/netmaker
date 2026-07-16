package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
)

const jitGrantTable = "jit_grants"

type JITGrant struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	TenantID  string    `gorm:"default:'';index" json:"tenant_id"`
	NetworkID string    `gorm:"network_id" json:"network_id"`
	UserID    string    `gorm:"user_id" json:"user_id"`
	RequestID string    `gorm:"request_id" json:"request_id"`
	GrantedAt time.Time `gorm:"granted_at" json:"granted_at"`
	ExpiresAt time.Time `gorm:"expires_at" json:"expires_at"`
}

func (g *JITGrant) Table() string {
	return jitGrantTable
}

func (g *JITGrant) Get(ctx context.Context) error {
	return db.FromContext(ctx).Table(g.Table()).Where("id = ?", g.ID).First(&g).Error
}

func (g *JITGrant) Create(ctx context.Context) error {
	return db.FromContext(ctx).Table(g.Table()).Create(&g).Error
}

func (g *JITGrant) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Table(g.Table()).Where("id = ?", g.ID).Delete(&g).Error
}

func (g *JITGrant) GetActiveByUserAndNetwork(ctx context.Context) (*JITGrant, error) {
	var grant JITGrant
	err := db.FromContext(ctx).Table(g.Table()).
		Where("network_id = ? AND user_id = ? AND expires_at > ?",
			g.NetworkID, g.UserID, time.Now()).
		First(&grant).Error
	if err != nil {
		return nil, err
	}
	return &grant, nil
}

func (g *JITGrant) ListActiveByNetwork(ctx context.Context) ([]JITGrant, error) {
	var grants []JITGrant
	query := db.FromContext(ctx).Table(g.Table()).
		Where("network_id = ? AND expires_at > ?", g.NetworkID, time.Now())
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", jitGrantTable), tenantID)(query)
	}
	err := query.Find(&grants).Error
	return grants, err
}

func (g *JITGrant) ListExpired(ctx context.Context) ([]JITGrant, error) {
	var grants []JITGrant
	query := db.FromContext(ctx).Table(g.Table()).
		Where("expires_at <= ?", time.Now())
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", jitGrantTable), tenantID)(query)
	}
	err := query.Find(&grants).Error
	return grants, err
}

func (g *JITGrant) ListByUserAndNetwork(ctx context.Context) ([]JITGrant, error) {
	var grants []JITGrant
	query := db.FromContext(ctx).Table(g.Table()).
		Where("network_id = ? AND user_id = ?", g.NetworkID, g.UserID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", jitGrantTable), tenantID)(query)
	}
	err := query.Find(&grants).Error
	return grants, err
}

func (g *JITGrant) GetByRequestID(ctx context.Context) (*JITGrant, error) {
	var grant JITGrant
	err := db.FromContext(ctx).Table(g.Table()).
		Where("request_id = ?", g.RequestID).
		First(&grant).Error
	if err != nil {
		return nil, err
	}
	return &grant, nil
}
