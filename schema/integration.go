package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

// Integration stores a single active integration config.
// The primary key is the provider ID (e.g. "splunk", "elastic").
// Deleting a row fully removes the integration — there is no soft-delete.
type Integration struct {
	IntegrationID string         `gorm:"primaryKey;column:integration_id" json:"integration_id"`
	Type          string         `gorm:"not null;column:type"              json:"type"`
	Config        datatypes.JSON `gorm:"not null;column:config"            json:"config"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// Upsert inserts or updates the integration record.
func (i *Integration) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(i).Error
}

// Get loads the integration by its primary key.
func (i *Integration) Get(ctx context.Context) error {
	return db.FromContext(ctx).First(i, "integration_id = ?", i.IntegrationID).Error
}

// Delete hard-deletes the integration record.
func (i *Integration) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Delete(i, "integration_id = ?", i.IntegrationID).Error
}
