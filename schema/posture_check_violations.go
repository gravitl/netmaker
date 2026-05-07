package schema

import (
	"time"
)

const postureCheckViolationsTable = "posture_check_violations_v1"

type PostureCheckViolation struct {
	EvaluationCycleID string    `gorm:"primaryKey;column:evaluation_cycle_id" json:"evaluation_cycle_id"`
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
