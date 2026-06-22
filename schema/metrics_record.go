package schema

import (
	"time"

	"gorm.io/datatypes"
)

// Metrics - metrics struct
type Metrics struct {
	Network      string            `json:"network" bson:"network" yaml:"network"`
	NodeID       string            `json:"node_id" bson:"node_id" yaml:"node_id"`
	NodeName     string            `json:"node_name" bson:"node_name" yaml:"node_name"`
	Connectivity map[string]Metric `json:"connectivity" bson:"connectivity" yaml:"connectivity"`
	UpdatedAt    time.Time         `json:"updated_at" bson:"updated_at" yaml:"updated_at"`
}

// Metric - holds a metric for data between nodes
type Metric struct {
	NodeName          string        `json:"node_name" bson:"node_name" yaml:"node_name"`
	Uptime            int64         `json:"uptime" bson:"uptime" yaml:"uptime" swaggertype:"primitive,integer" format:"int64"`
	TotalTime         int64         `json:"totaltime" bson:"totaltime" yaml:"totaltime" swaggertype:"primitive,integer" format:"int64"`
	Latency           int64         `json:"latency" bson:"latency" yaml:"latency" swaggertype:"primitive,integer" format:"int64"`
	TotalReceived     int64         `json:"totalreceived" bson:"totalreceived" yaml:"totalreceived" swaggertype:"primitive,integer" format:"int64"`
	LastTotalReceived int64         `json:"lasttotalreceived" bson:"lasttotalreceived" yaml:"lasttotalreceived" swaggertype:"primitive,integer" format:"int64"`
	TotalSent         int64         `json:"totalsent" bson:"totalsent" yaml:"totalsent" swaggertype:"primitive,integer" format:"int64"`
	LastTotalSent     int64         `json:"lasttotalsent" bson:"lasttotalsent" yaml:"lasttotalsent" swaggertype:"primitive,integer" format:"int64"`
	ActualUptime      time.Duration `json:"actualuptime" swaggertype:"primitive,integer" format:"int64" bson:"actualuptime" yaml:"actualuptime"`
	PercentUp         float64       `json:"percentup" bson:"percentup" yaml:"percentup"`
	Connected         bool          `json:"connected" bson:"connected" yaml:"connected"`
}

type MetricsRecord struct {
	Key      string `gorm:"primaryKey"`
	TenantID string `gorm:"default:''"`
	Value    datatypes.JSONType[Metrics]
}

func (*MetricsRecord) TableName() string { return "metrics" }
