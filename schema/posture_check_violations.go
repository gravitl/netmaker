package schema

import (
	"time"
)

const postureCheckViolationsTable = "posture_check_violations_v1"

type PostureCheckViolation struct {
	EvaluationCycleID string `gorm:"primaryKey"`
	CheckID           string `gorm:"primaryKey"`
	NodeID            string `gorm:"primaryKey"`
	Name              string
	Attribute         string
	Message           string
	Severity          Severity
	EvaluatedAt       time.Time
}

func (v *PostureCheckViolation) TableName() string {
	return postureCheckViolationsTable
}
