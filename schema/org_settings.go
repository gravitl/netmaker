package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

const orgSettingsTable = "org_settings_v1"

type OrganizationSettings struct {
	ID       string `gorm:"primaryKey"`
	Settings datatypes.JSONType[OrganizationSettingsData]
}

type OrganizationSettingsData struct{}

func (o *OrganizationSettings) TableName() string {
	return orgSettingsTable
}

func (o *OrganizationSettings) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(&o).Error
}

func (o *OrganizationSettings) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&OrganizationSettings{}).
		First(&o).
		Error
}

func (o *OrganizationSettings) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Delete(&o).Error
}
