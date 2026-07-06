package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"
)

type migrationFunc func(ctx context.Context) error

// ToSQLSchema migrates the data from key-value
// db to sql db.
func ToSQLSchema() error {
	// multitenancy migration creates the default organization and tenant. this is
	// done separately from v1.7.0 migration because user role and group info has
	// been dropped from the users table in v1.7.0. if a tenant is migrated from v1.5.0
	// to v1.7.0, this info won't be available.
	err := ensureMigrationCompleted(context.TODO(), "migration-multitenancy", migrateMultiTenancy)
	if err != nil {
		return err
	}

	// v1.5.1 migration includes migrating the users, groups, roles, networks and hosts tables.
	err = ensureMigrationCompleted(context.TODO(), "migration-v1.5.1", migrateV1_5_1)
	if err != nil {
		return err
	}

	// v1.6.0 migration includes migrating the pending users and user invites tables.
	err = ensureMigrationCompleted(context.TODO(), "migration-v1.6.0", migrateV1_6_0)
	if err != nil {
		return err
	}

	// v1.7.0 migration includes migrating the server conf, generated, server uuid and
	// enrollment key tables.
	// this version also includes changes for multi-tenancy and so this job
	// assigns the tenant id to all the existing records.
	err = ensureMigrationCompleted(context.TODO(), "migration-v1.7.0", migrateV1_7_0)
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
