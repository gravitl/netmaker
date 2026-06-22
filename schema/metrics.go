package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
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

// MetricsEntry is the GORM model for the legacy "metrics" key-value table,
// extended with tenant_id and network_id columns for multi-tenancy.
type MetricsEntry struct {
	Key       string                      `gorm:"primaryKey;column:key"`
	TenantID  string                      `gorm:"column:tenant_id;default:''"`
	NetworkID string                      `gorm:"column:network_id"`
	Value     datatypes.JSONType[Metrics] `gorm:"column:value"`
}

func (*MetricsEntry) TableName() string { return "metrics" }

func (e *MetricsEntry) Create(ctx context.Context) error {
	return db.FromContext(ctx).Create(e).Error
}

// Save does an upsert — insert or replace on primary key conflict.
func (e *MetricsEntry) Save(ctx context.Context) error {
	return db.FromContext(ctx).Save(e).Error
}

func (e *MetricsEntry) Get(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", e.Key).First(e).Error
}

func (e *MetricsEntry) ListAll(ctx context.Context) ([]MetricsEntry, error) {
	var entries []MetricsEntry
	err := db.FromContext(ctx).Find(&entries).Error
	return entries, err
}

func (e *MetricsEntry) ListByNetwork(ctx context.Context) ([]MetricsEntry, error) {
	var entries []MetricsEntry
	err := db.FromContext(ctx).Where("network_id = ?", e.NetworkID).Find(&entries).Error
	return entries, err
}

func (e *MetricsEntry) Count(ctx context.Context) (int64, error) {
	var count int64
	err := db.FromContext(ctx).Model(&MetricsEntry{}).Count(&count).Error
	return count, err
}

func (e *MetricsEntry) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Where("key = ?", e.Key).Delete(&MetricsEntry{}).Error
}

func (e *MetricsEntry) DeleteByNetwork(ctx context.Context) error {
	return db.FromContext(ctx).Where("network_id = ?", e.NetworkID).Delete(&MetricsEntry{}).Error
}
