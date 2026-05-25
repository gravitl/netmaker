package schema

import (
	"context"
	"time"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type Integration struct {
	ID        string         `gorm:"primaryKey;column:id" json:"id"`
	Type      string         `gorm:"not null;column:type"              json:"type"`
	Config    datatypes.JSON `gorm:"not null;column:config"            json:"config"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (i *Integration) TableName() string {
	return "integrations_v1"
}

func (i *Integration) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(i).Error
}

func (i *Integration) Get(ctx context.Context) error {
	return db.FromContext(ctx).First(i, "id = ?", i.ID).Error
}

func (i *Integration) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Delete(i, "id = ?", i.ID).Error
}

func (i *Integration) ListByType(ctx context.Context) ([]Integration, error) {
	var integrations []Integration
	err := db.FromContext(ctx).Model(&Integration{}).
		Where("type = ?", i.Type).
		Find(&integrations).
		Error
	return integrations, err
}
