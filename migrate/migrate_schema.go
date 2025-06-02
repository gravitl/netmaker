package migrate

import (
	"context"
	"errors"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"
)

// ToSQLSchema migrates the data from key-value
// db to sql db.
//
// This function archives the old data and does not
// delete it.
func ToSQLSchema() error {
	// initialize sql schema db.
	err := db.InitializeDB(schema.ListModels()...)
	if err != nil {
		return err
	}

	defer db.CloseDB()

	// migrate, if not done already.
	return migrate()
}

func migrate() error {
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

	// check if migrated already.
	migrationJob := &schema.Job{
		ID: "migration-v1.0.0",
	}
	err := migrationJob.Get(dbctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// initialize key-value schema db.
		err := database.InitializeDatabase()
		if err != nil {
			return err
		}
		defer database.CloseDB()

		// migrate.
		// TODO: add migration code.

		// mark migration job completed.
		err = migrationJob.Create(dbctx)
		if err != nil {
			return err
		}

		commit = true
	}

	return nil
}
