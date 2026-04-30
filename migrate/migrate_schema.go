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
	// begin a new transaction.
	dbctx := db.BeginTx(context.TODO())
	commit := false
	defer func() {
		if commit {
			db.FromContext(dbctx).Commit()
		} else {
			db.FromContext(dbctx).Rollback()
		}
	}()

	// v1.5.1 migration includes migrating the users, groups, roles, networks and hosts tables.
	// future table migrations should be made below this block,
	// with a different version number and a similar check for whether the
	// migration was already done.
	err := ensureMigrationCompleted(dbctx, "migration-v1.5.1", migrateV1_5_1)
	if err != nil {
		return err
	}

	// v1.5.2 migration includes migrating the pending users and user invites tables.
	err = ensureMigrationCompleted(dbctx, "migration-v1.5.2", migrateV1_5_2)
	if err != nil {
		return err
	}

	commit = true
	return nil
}

func ensureMigrationCompleted(ctx context.Context, version string, migrate migrationFunc) error {
	migrationJob := &schema.Job{
		ID: version,
	}
	err := migrationJob.Get(ctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		logger.Log(1, fmt.Sprintf("running migration job %s", migrationJob.ID))
		// migrate.
		err = migrate(ctx)
		if err != nil {
			return err
		}

		// mark migration job completed.
		err = migrationJob.Create(ctx)
		if err != nil {
			return err
		}

		logger.Log(1, fmt.Sprintf("migration job %s completed", migrationJob.ID))
	} else {
		logger.Log(1, fmt.Sprintf("migration job %s already completed, skipping", migrationJob.ID))
	}

	return nil
}
