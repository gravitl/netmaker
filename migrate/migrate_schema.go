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

// ToSQLSchema migrates the data from key-value db to sql db.
//
// Migration order:
//   - v1.5.1: users, networks, roles, groups, hosts (pre-MT)
//   - v1.6.0: pending users, invites, nodes (pre-MT)
//   - v1.7.0: MT bootstrap (default org/tenant), server conf, memberships, tenant IDs, ...
//
// The legacy migration-multitenancy job is folded into v1.7.0 step 0. Existing
// deployments that already completed migration-multitenancy keep that job row;
// CreateLocalDefaults is a no-op when org/tenant already exist.
func ToSQLSchema() error {
	// v1.5.1 migration includes migrating the users, groups, roles, networks and hosts tables.
	err := ensureMigrationCompleted(context.TODO(), "migration-v1.5.1", migrateV1_5_1)
	if err != nil {
		return err
	}

	// v1.6.0 migration includes migrating the pending users, user invites and nodes tables.
	err = ensureMigrationCompleted(context.TODO(), "migration-v1.6.0", migrateV1_6_0)
	if err != nil {
		return err
	}

	// v1.7.0 bootstraps multi-tenancy and migrates server conf, generated, server uuid,
	// enrollment keys, memberships, and assigns tenant IDs to existing records.
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
