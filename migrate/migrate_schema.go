package migrate

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gravitl/netmaker/converters"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"
)

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

	// check if migrated already.
	migrationJob := &schema.Job{
		ID: "migration-v1.5.0",
	}
	err := migrationJob.Get(dbctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// migrate.
		err = migrateUsers(dbctx)
		if err != nil {
			return err
		}

		err = migrateNetworks(dbctx)
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

func migrateUsers(ctx context.Context) error {
	records, err := database.FetchRecords(database.USERS_TABLE_NAME)
	if err != nil {
		return err
	}

	for _, record := range records {
		var user models.User
		err = json.Unmarshal([]byte(record), &user)
		if err != nil {
			return err
		}

		_user := converters.ToSchemaUser(user)
		err = _user.Create(ctx)
		if err != nil {
			return err
		}

		for groupID := range user.UserGroups {
			_groupMember := schema.GroupMember{
				GroupID: string(groupID),
				UserID:  _user.ID,
			}
			err = _groupMember.Create(ctx)
			if err != nil {
				return err
			}
		}

		for networkID, role := range user.NetworkRoles {
			_networkRole := schema.UserNetworkRole{
				UserID:    _user.ID,
				NetworkID: string(networkID),
			}
			for roleID := range role {
				_networkRole.RoleID = string(roleID)
			}
			err = _networkRole.Create(ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func migrateNetworks(ctx context.Context) error {
	records, err := database.FetchRecords(database.NETWORKS_TABLE_NAME)
	if err != nil {
		return err
	}

	for _, record := range records {
		var network models.Network
		err = json.Unmarshal([]byte(record), &network)
		if err != nil {
			return err
		}

		_network := converters.ToSchemaNetwork(network)
		err = _network.Create(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}
