package migrate

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"
)

type migrationFunc func(ctx context.Context) error

// ErrMigrationV160Required is returned when v1.7.0 migration is attempted before
// migration-v1.6.0 completed on a prior v1.6.0 deployment.
var ErrMigrationV160Required = errors.New(
	"migration-v1.6.0 must complete on Netmaker v1.6.0 before upgrading to v1.7.0; " +
		"deploy v1.6.0, restart the server successfully, then upgrade to v1.7.0",
)

func ToSQLSchema() error {
	migratedToV170, err := migrationJobCompleted(db.WithContext(context.TODO()), migrationJobV170)
	if err != nil {
		return err
	}

	if migratedToV170 {
		return nil
	}

	newDeployment, err := isNewDeployment(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	if !newDeployment {
		migratedToV160, err := migrationJobCompleted(db.WithContext(context.TODO()), migrationJobV160)
		if err != nil {
			return err
		}

		if !migratedToV160 {
			logger.Log(0, ErrMigrationV160Required.Error())
			os.Exit(1)
			return ErrMigrationV160Required
		}
	}

	return ensureMigrationCompleted(db.WithContext(context.TODO()), migrationJobV170, migrateV1_7_0)
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
