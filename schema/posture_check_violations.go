package schema

import (
	"time"
)

const postureCheckViolationsTable = "posture_check_violations_v1"

type PostureCheckViolation struct {
	EvaluationCycleID string `gorm:"primaryKey;evaluation_cycle_id"`
	CheckID           string `gorm:"primaryKey;check_id"`
	NodeID            string `gorm:"primaryKey;node_id"`
	Name              string
	Attribute         string
	Message           string
	Severity          Severity
	EvaluatedAt       time.Time
}

func (v *PostureCheckViolation) TableName() string {
	return postureCheckViolationsTable
}
