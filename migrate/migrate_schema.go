package migrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gravitl/netmaker/converters"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/gorm"
	"os"
	"path/filepath"
)

// ToSQLSchema migrates the data from key-value
// db to sql db.
//
// This function archives the old data and does not
// delete it.
//
// Based on the db server, the archival is done in the
// following way:
//
// 1. Sqlite: Moves the old data to a
// netmaker_archive.db file.
//
// 2. Postgres: Moves the data to a netmaker_archive
// schema within the same database.
func ToSQLSchema() error {
	// initialize sql schema db.
	err := db.InitializeDB(schema.ListModels()...)
	if err != nil {
		return err
	}

	// migrate, if not done already.
	err = migrate()
	if err != nil {
		return err
	}

	// archive key-value schema db, if not done already.
	// ignore errors.
	_ = archive()

	return nil
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
		err = migrateNetworksTable(dbctx)
		if err != nil {
			return err
		}

		// mark migration job completed.
		err = migrationJob.Create(dbctx)
		if err != nil {
			return err
		}

		commit = true
	}

	return nil
}

func archive() error {
	dbServer := servercfg.GetDB()
	if dbServer != "sqlite" && dbServer != "postgres" {
		return nil
	}

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

	// check if key-value schema db archived already.
	archivalJob := &schema.Job{
		ID: "archival-v1.0.0",
	}
	err := archivalJob.Get(dbctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// archive.
		switch dbServer {
		case "sqlite":
			err = sqliteArchiveOldData()
		default:
			err = pgArchiveOldData()
		}
		if err != nil {
			return err
		}

		// mark archival job completed.
		err = archivalJob.Create(dbctx)
		if err != nil {
			return err
		}

		commit = true
	} else {
		// remove the residual
		if dbServer == "sqlite" {
			_ = os.Remove(filepath.Join("data", "netmaker.db"))
		}
	}

	return nil
}

func sqliteArchiveOldData() error {
	oldDBFilePath := filepath.Join("data", "netmaker.db")
	archiveDBFilePath := filepath.Join("data", "netmaker_archive.db")

	// check if netmaker_archive.db exist.
	_, err := os.Stat(archiveDBFilePath)
	if err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	// rename old db file to netmaker_archive.db.
	return os.Rename(oldDBFilePath, archiveDBFilePath)
}

func pgArchiveOldData() error {
	_, err := database.PGDB.Exec("CREATE SCHEMA IF NOT EXISTS netmaker_archive")
	if err != nil {
		return err
	}

	for _, table := range database.Tables {
		_, err := database.PGDB.Exec(
			fmt.Sprintf(
				"ALTER TABLE public.%s SET SCHEMA netmaker_archive",
				table,
			),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func migrateNetworksTable(dbctx context.Context) error {
	networksMap, err := database.FetchRecords(database.NETWORKS_TABLE_NAME)
	if err != nil {
		return err
	}

	var networks []models.Network

	for _, networkStr := range networksMap {
		var network models.Network
		err = json.Unmarshal([]byte(networkStr), &network)
		if err != nil {
			return err
		}

		networks = append(networks, network)
	}

	_networks := converters.ToSchemaNetworks(networks)

	for _, _network := range _networks {
		err = _network.Create(dbctx)
		if err != nil {
			return err
		}
	}

	return nil
}
