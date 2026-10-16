package migrate

import (
	"context"
	"errors"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/migrate/types"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"
)

const (
	migrationJobV170 = "migration-v1.7.0"
	migrationJobV180 = "migration-v1.8.0"

	migrationJobInitializeTenants = "initialize-tenants"
)

func getNetworkByNameForMigration(ctx context.Context, name string) (*schema.Network, error) {
	var network schema.Network
	err := db.FromContext(ctx).Model(&schema.Network{}).
		Where("name = ?", name).
		First(&network).Error
	if err != nil {
		return nil, err
	}
	return &network, nil
}

func ensureLegacyUserColumns(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable((&types.LegacyUser{}).TableName()) {
		return nil
	}
	return db.FromContext(ctx).AutoMigrate(&types.LegacyUser{})
}

func migrationJobCompleted(ctx context.Context, jobID string) (bool, error) {
	job := &schema.Job{ID: jobID}
	err := job.Get(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
