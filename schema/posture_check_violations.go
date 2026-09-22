package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/scope"
)

const postureCheckViolationsTable = "posture_check_violations_v1"

type PostureCheckViolation struct {
	EvaluationCycleID string    `gorm:"primaryKey;column:evaluation_cycle_id" json:"evaluation_cycle_id"`
	TenantID          string    `gorm:"default:'';index" json:"tenant_id"`
	CheckID           string    `gorm:"primaryKey;column:check_id" json:"check_id"`
	NodeID            string    `gorm:"primaryKey;column:node_id" json:"node_id"`
	Name              string    `json:"name"`
	Attribute         string    `json:"attribute"`
	Message           string    `json:"message"`
	Severity          Severity  `json:"severity"`
	EvaluatedAt       time.Time `json:"evaluated_at"`
}

func (v *PostureCheckViolation) TableName() string {
	return postureCheckViolationsTable
}

func (v *PostureCheckViolation) DeleteAll(ctx context.Context) error {
	if tenantID := scope.ID(ctx); tenantID != "" {
		return db.FromContext(ctx).Where(fmt.Sprintf("%s.tenant_id = ?", postureCheckViolationsTable), tenantID).Delete(&PostureCheckViolation{}).Error
	}
	return db.FromContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s", postureCheckViolationsTable)).Error
}
