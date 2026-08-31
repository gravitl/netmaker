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
//   - v1.7.0: MT bootstrap via SyncOrgAndTenants, server conf, memberships, tenant IDs, ...
//
// Hard requirement: existing SQL deployments must complete v1.6.0 on a v1.6.0 release
// (migration-v1.6.0 job recorded from a prior successful startup) before v1.7.0 runs.
// If v1.6.0 is applied during the same startup as v1.7.0 on an existing deployment,
// ToSQLSchema returns ErrMigrationV160Required so operators upgrade in two steps.
// Greenfield installs and full legacy KV upgrade chains are exempt.
//
// The legacy migration-multitenancy job is folded into v1.7.0 step 0.
// SyncOrgAndTenants defaults to CreateLocalDefaults (CE); EE overrides it with
// license.SyncOrgAndTenants so MSP installs create tenants from the account
// server instead of a local UUID default. Existing deployments that already
// completed migration-multitenancy keep that job row; bootstrap is idempotent
// when org/tenant already exist.
func ToSQLSchema() error {
	ctx := context.TODO()
	dbctx := db.WithContext(ctx)

	v151AlreadyComplete, err := migrationJobCompleted(dbctx, migrationJobV151)
	if err != nil {
		return err
	}
	v160AlreadyComplete, err := migrationJobCompleted(dbctx, migrationJobV160)
	if err != nil {
		return err
	}

	// v1.5.1 migration includes migrating the users, groups, roles, networks and hosts tables.
	err = ensureMigrationCompleted(ctx, migrationJobV151, migrateV1_5_1)
	if err != nil {
		return err
	}

	// v1.6.0 migration includes migrating the pending users, user invites and nodes tables.
	err = ensureMigrationCompleted(ctx, migrationJobV160, migrateV1_6_0)
	if err != nil {
		return err
	}

	ok, err := canRunMigrationV170(dbctx, v160AlreadyComplete, v151AlreadyComplete)
	if err != nil {
		return err
	}
	if !ok {
		return ErrMigrationV160Required
	}

	// v1.7.0 bootstraps multi-tenancy and migrates server conf, generated, server uuid,
	// enrollment keys, memberships, and assigns tenant IDs to existing records.
	return ensureMigrationCompleted(ctx, migrationJobV170, migrateV1_7_0)
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
