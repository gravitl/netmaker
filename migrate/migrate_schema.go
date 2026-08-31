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
// Greenfield installs (empty jobs table, no legacy data) skip v1.5.1/v1.6.0 and run
// v1.7.0 bootstrap only.
//
// Hard requirement: v1.7.0 refuses startup when migration-v1.6.0 is incomplete on an
// upgrade that already has migration job history. v1.6.0 migration must run on a
// v1.6.0 release image first. Legacy KV upgrade chains (no jobs yet but legacy data
// present) still run the full pre-MT chain.
// The legacy migration-multitenancy job is folded into v1.7.0 step 0.
// SyncOrgAndTenants defaults to CreateLocalDefaults (CE); EE overrides it with
// license.SyncOrgAndTenants so MSP installs create tenants from the account
// server instead of a local UUID default. Existing deployments that already
// completed migration-multitenancy keep that job row; bootstrap is idempotent
// when org/tenant already exist.
func ToSQLSchema() error {
	ctx := context.TODO()
	dbctx := db.WithContext(ctx)

	hasPriorMigrationJobs, err := anyMigrationJobPresent(dbctx)
	if err != nil {
		return err
	}

	greenfield, err := isGreenfieldInstall(dbctx, hasPriorMigrationJobs)
	if err != nil {
		return err
	}
	if greenfield {
		if err := markMigrationJobSkipped(dbctx, migrationJobV151); err != nil {
			return err
		}
		if err := markMigrationJobSkipped(dbctx, migrationJobV160); err != nil {
			return err
		}
		return ensureMigrationCompleted(dbctx, migrationJobV170, migrateV1_7_0)
	}

	v151AlreadyComplete, err := migrationJobCompleted(dbctx, migrationJobV151)
	if err != nil {
		return err
	}
	v160AlreadyComplete, err := migrationJobCompleted(dbctx, migrationJobV160)
	if err != nil {
		return err
	}

	if err := enforceMigrationV170Compatibility(dbctx, hasPriorMigrationJobs, v151AlreadyComplete, v160AlreadyComplete); err != nil {
		return err
	}
	if !v151AlreadyComplete {
		err = ensureMigrationCompleted(dbctx, migrationJobV151, migrateV1_5_1)
		if err != nil {
			return err
		}
	}

	if !v160AlreadyComplete {
		err = ensureMigrationCompleted(dbctx, migrationJobV160, migrateV1_6_0)
		if err != nil {
			return err
		}
	}

	return ensureMigrationCompleted(dbctx, migrationJobV170, migrateV1_7_0)
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
