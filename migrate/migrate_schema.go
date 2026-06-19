package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"
)

type migrationFunc func(ctx context.Context) error

// ToSQLSchema migrates the data from key-value
// db to sql db.
func ToSQLSchema() error {
	// load server settings in cache.
	_ = logic.GetServerSettings()
	// v1.5.1 migration includes migrating the users, groups, roles, networks and hosts tables.
	// future table migrations should be made below this block,
	// with a different version number and a similar check for whether the
	// migration was already done.
	err := ensureMigrationCompleted(context.TODO(), "migration-v1.5.1", migrateV1_5_1)
	if err != nil {
		return err
	}

	// v1.6.0 migration includes migrating the pending users and user invites tables.
	err = ensureMigrationCompleted(context.TODO(), "migration-v1.6.0", migrateV1_6_0)
	if err != nil {
		return err
	}

	// v1.7.0 migration includes migrating the server conf, generated, server uuid and server
	// settings tables.
	err = ensureMigrationCompleted(context.TODO(), "migration-v1.7.0", migrateV1_7_0)
	if err != nil {
		return err
	}

	// v1.7.0 multi-tenancy: adds org/tenant tables, tenant_id columns, and bootstraps defaults.
	err = ensureMigrationCompleted(context.TODO(), "migration-v1.7.0-multitenancy", migrateMultitenancy)
	if err != nil {
		return err
	}

	return nil
}

func ensureMigrationCompleted(ctx context.Context, version string, migrate migrationFunc) error {
	dbctx := db.BeginTx(ctx)
	commit := false
	defer func() {
		if commit {
			db.FromContext(dbctx).Commit()
		} else {
			db.FromContext(dbctx).Rollback()
		}
	}()

	migrationJob := &schema.Job{
		ID: version,
	}
	err := migrationJob.Get(dbctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		logger.Log(0, fmt.Sprintf("running migration job %s", migrationJob.ID))
		// migrate.
		err = migrate(dbctx)
		if err != nil {
			return err
		}

		// mark migration job completed.
		err = migrationJob.Create(dbctx)
		if err != nil {
			return err
		}

		logger.Log(0, fmt.Sprintf("migration job %s completed", migrationJob.ID))
	} else {
		logger.Log(0, fmt.Sprintf("migration job %s already completed, skipping", migrationJob.ID))
	}

	commit = true
	return nil
}
