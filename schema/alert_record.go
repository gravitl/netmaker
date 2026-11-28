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

const alertRecordsTable = "alert_records_v1"

type AlertStatus string

const (
	AlertStatusFiring       AlertStatus = "firing"
	AlertStatusAcknowledged AlertStatus = "acknowledged"
	AlertStatusResolved     AlertStatus = "resolved"
)

type AlertRecord struct {
	ID             string         `gorm:"primaryKey" json:"id"`
	TenantID       string         `gorm:"default:'';index" json:"tenant_id"`
	NetworkID      string         `gorm:"default:'';index" json:"network_id"`
	RuleID         string         `gorm:"default:'';index" json:"rule_id"`
	RuleName       string         `json:"rule_name"`
	EventType      AlertEventType `gorm:"not null;index" json:"event_type"`
	Severity       string         `gorm:"not null;default:'warning'" json:"severity"` // info, warning, critical
	SubjectID      string         `gorm:"not null;index" json:"subject_id"`
	SubjectName    string         `json:"subject_name"`
	SubjectType    SubjectType    `json:"subject_type"`
	Summary        string         `gorm:"not null" json:"summary"`
	Details        datatypes.JSON `json:"details"`
	Status         AlertStatus    `gorm:"not null;default:'firing';index" json:"status"`
	AcknowledgedBy string         `json:"acknowledged_by"`
	AcknowledgedAt *time.Time     `json:"acknowledged_at"`
	ResolvedAt     *time.Time     `json:"resolved_at"`
	CreatedAt      time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func (a *AlertRecord) TableName() string {
	return alertRecordsTable
}

func (a *AlertRecord) Create(ctx context.Context) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if tenantID := scope.ID(ctx); tenantID != "" && a.TenantID == "" {
		a.TenantID = tenantID
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if a.UpdatedAt.IsZero() {
		a.UpdatedAt = time.Now().UTC()
	}
	return db.FromContext(ctx).Create(a).Error
}

func (a *AlertRecord) Get(ctx context.Context) error {
	query := db.FromContext(ctx).Model(&AlertRecord{}).Where("id = ?", a.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", alertRecordsTable), tenantID)(query)
	}
	return query.First(a).Error
}

func (a *AlertRecord) List(ctx context.Context, status AlertStatus, networkID, severity string, from, to time.Time) ([]AlertRecord, error) {
	var records []AlertRecord
	query := db.FromContext(ctx).Model(&AlertRecord{})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", alertRecordsTable), tenantID)(query)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if networkID != "" {
		query = query.Where("network_id = ?", networkID)
	}
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if !from.IsZero() && !to.IsZero() {
		query = query.Where("created_at BETWEEN ? AND ?", from.UTC(), to.UTC())
	}
	err := query.Order("created_at DESC").Find(&records).Error
	return records, err
}

func (a *AlertRecord) Acknowledge(ctx context.Context, username string) error {
	now := time.Now().UTC()
	query := db.FromContext(ctx).Model(&AlertRecord{}).Where("id = ?", a.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", alertRecordsTable), tenantID)(query)
	}
	return query.Updates(map[string]interface{}{
		"status":          AlertStatusAcknowledged,
		"acknowledged_by": username,
		"acknowledged_at": &now,
		"updated_at":      now,
	}).Error
}

func (a *AlertRecord) Resolve(ctx context.Context) error {
	now := time.Now().UTC()
	query := db.FromContext(ctx).Model(&AlertRecord{}).Where("id = ?", a.ID)
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", alertRecordsTable), tenantID)(query)
	}
	return query.Updates(map[string]interface{}{
		"status":      AlertStatusResolved,
		"resolved_at": &now,
		"updated_at":  now,
	}).Error
}

func (a *AlertRecord) FindRecentFiring(ctx context.Context, ruleID, subjectID string, since time.Time) (*AlertRecord, error) {
	var record AlertRecord
	query := db.FromContext(ctx).Model(&AlertRecord{}).
		Where("rule_id = ? AND subject_id = ? AND status = ? AND created_at >= ?", ruleID, subjectID, AlertStatusFiring, since.UTC())
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", alertRecordsTable), tenantID)(query)
	}
	err := query.First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (a *AlertRecord) FindActiveFiringBySubject(ctx context.Context, eventType AlertEventType, subjectID string) ([]AlertRecord, error) {
	var records []AlertRecord
	query := db.FromContext(ctx).Model(&AlertRecord{}).
		Where("event_type = ? AND subject_id = ? AND status IN ?", eventType, subjectID, []AlertStatus{AlertStatusFiring, AlertStatusAcknowledged})
	if tenantID := scope.ID(ctx); tenantID != "" {
		query = dbtypes.WithFilter(fmt.Sprintf("%s.tenant_id", alertRecordsTable), tenantID)(query)
	}
	err := query.Find(&records).Error
	return records, err
}
