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

// ErrMigrationV170Required is returned when v1.8.0 migration is attempted before
// migration-v1.7.0 completed on a prior v1.7.0 deployment.
var ErrMigrationV170Required = errors.New(
	"migration-v1.7.0 must complete on Netmaker v1.7.0 before upgrading to v1.8.0; " +
		"deploy v1.7.0, restart the server successfully, then upgrade to v1.8.0",
)

func ToSQLSchema() error {
	// Bootstraps the default org/tenant independent of any version-gated
	// migration, so a fresh deployment gets one even if no migration job
	// below ever runs for it.
	if err := ensureMigrationCompleted(db.WithContext(context.TODO()), migrationJobInitializeTenants, initializeTenants); err != nil {
		return err
	}

	migratedToV180, err := migrationJobCompleted(db.WithContext(context.TODO()), migrationJobV180)
	if err != nil {
		return err
	}

	if migratedToV180 {
		return nil
	}

	newDeployment, err := isNewDeployment(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	if !newDeployment {
		migratedToV170, err := migrationJobCompleted(db.WithContext(context.TODO()), migrationJobV170)
		if err != nil {
			return err
		}

		if !migratedToV170 {
			return ErrMigrationV170Required
		}
	}

	return ensureMigrationCompleted(db.WithContext(context.TODO()), migrationJobV180, migrateV1_8_0)
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
