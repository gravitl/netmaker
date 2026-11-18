package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
)

const alertRulesTable = "alert_rules_v1"

type AlertEventType string

const (
	EventNodeOffline         AlertEventType = "node_offline"
	EventNodeRecovered       AlertEventType = "node_recovered"
	EventGatewayDegraded     AlertEventType = "gateway_degraded"
	EventConnectivityFailure AlertEventType = "connectivity_failure"
	EventSecurityViolation   AlertEventType = "security_violation"
	EventAclViolation        AlertEventType = "acl_violation"
)

type AlertRule struct {
	ID              string                      `gorm:"primaryKey" json:"id"`
	TenantID        string                      `gorm:"default:'';index" json:"tenant_id"`
	NetworkID       string                      `gorm:"default:'';index" json:"network_id"` // optional: empty means all networks in tenant
	Name            string                      `gorm:"not null" json:"name"`
	EventType       AlertEventType              `gorm:"not null;index" json:"event_type"`
	Severity        string                      `gorm:"not null;default:'warning'" json:"severity"` // info, warning, critical
	Threshold       int                         `gorm:"default:0" json:"threshold"`                 // e.g. seconds or retry count
	Enabled         bool                        `gorm:"default:true" json:"enabled"`
	CooldownMinutes int                         `gorm:"default:15" json:"cooldown_minutes"` // time before duplicate alert is sent
	ChannelIDs      datatypes.JSONSlice[string] `gorm:"type:text" json:"channel_ids"`
	CreatedAt       time.Time                   `json:"created_at"`
	UpdatedAt       time.Time                   `json:"updated_at"`
}

func (r *AlertRule) TableName() string {
	return alertRulesTable
}

func (r *AlertRule) Create(ctx context.Context) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if tenantID := scope.ID(ctx); tenantID != "" && r.TenantID == "" {
		r.TenantID = tenantID
	}
	return db.FromContext(ctx).Create(r).Error
}

func (r *AlertRule) Get(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&AlertRule{}).Where("id = ?", r.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", alertRulesTable), tenantID)(query)
	}
	return query.First(r).Error
}

func (r *AlertRule) Update(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&AlertRule{}).Where("id = ?", r.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", alertRulesTable), tenantID)(query)
	}
	return query.Updates(r).Error
}

func (r *AlertRule) Delete(ctx context.Context) error {
	query := db.FromContext(ctx).Where("id = ?", r.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", alertRulesTable), tenantID)(query)
	}
	return query.Delete(&AlertRule{}).Error
}

func (r *AlertRule) ListByTenant(ctx context.Context, networkID string) ([]AlertRule, error) {
	var rules []AlertRule
	query := db.FromContext(ctx).Model(&AlertRule{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", alertRulesTable), tenantID)(query)
	}
	if networkID != "" {
		query = query.Where("network_id = ? OR network_id = ''", networkID)
	}
	err := query.Order("created_at DESC").Find(&rules).Error
	return rules, err
}

func (r *AlertRule) ListActiveByEventAndNetwork(ctx context.Context, eventType AlertEventType, networkID string) ([]AlertRule, error) {
	var rules []AlertRule
	query := db.FromContext(ctx).Model(&AlertRule{}).
		Where("enabled = ?", true).
		Where("event_type = ?", eventType)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", alertRulesTable), tenantID)(query)
	}
	if networkID != "" {
		query = query.Where("network_id = ? OR network_id = ''", networkID)
	}
	err := query.Find(&rules).Error
	return rules, err
}
