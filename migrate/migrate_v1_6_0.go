package migrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
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

	// v1.6.0 migration includes migrating the users, groups, roles and networks tables.
	// future table migrations should be made below this block,
	// with a different version number and a similar check for whether the
	// migration was already done.
	migrationJob := &schema.Job{
		ID: "migration-v1.6.0",
	}
	err := migrationJob.Get(dbctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		logger.Log(1, fmt.Sprintf("running migration job %s", migrationJob.ID))
		// migrate.
		err = migrateV1_6_0(dbctx)
		if err != nil {
			return err
		}

		// mark migration job completed.
		err = migrationJob.Create(dbctx)
		if err != nil {
			return err
		}

		logger.Log(1, fmt.Sprintf("migration job %s completed", migrationJob.ID))
		commit = true
	} else {
		logger.Log(1, fmt.Sprintf("migration job %s already completed, skipping", migrationJob.ID))
	}

	return nil
}

func migrateV1_6_0(ctx context.Context) error {
	err := migrateUsers(ctx)
	if err != nil {
		return err
	}

	return migrateNetworks(ctx)
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

		platformRoleID := user.PlatformRoleID
		if user.PlatformRoleID == "" {
			if user.IsSuperAdmin {
				platformRoleID = models.SuperAdminRole
			} else if user.IsAdmin {
				platformRoleID = models.AdminRole
			} else {
				platformRoleID = models.ServiceUser
			}
		}

		_user := schema.User{
			ID:                         "",
			Username:                   user.UserName,
			DisplayName:                user.DisplayName,
			PlatformRoleID:             string(platformRoleID),
			ExternalIdentityProviderID: user.ExternalIdentityProviderID,
			AccountDisabled:            user.AccountDisabled,
			AuthType:                   string(user.AuthType),
			Password:                   user.Password,
			IsMFAEnabled:               user.IsMFAEnabled,
			TOTPSecret:                 user.TOTPSecret,
			LastLoginAt:                user.LastLoginTime,
			UserGroups:                 make(datatypes.JSONMap),
			CreatedBy:                  user.CreatedBy,
			CreatedAt:                  user.CreatedAt,
			UpdatedAt:                  user.UpdatedAt,
		}

		for userGroupID := range user.UserGroups {
			_user.UserGroups[string(userGroupID)] = struct{}{}
		}

		err = _user.Create(ctx)
		if err != nil {
			return err
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

		var autoJoin, autoRemove bool

		if network.AutoJoin == "true" {
			autoJoin = true
		} else if network.AutoJoin == "false" {
			autoJoin = false
		} else {
			autoJoin = true
		}

		if network.AutoRemove == "true" {
			autoRemove = true
		} else if network.AutoRemove == "false" {
			autoRemove = false
		} else {
			autoRemove = false
		}

		_network := &schema.Network{
			ID:                  "",
			Name:                network.NetID,
			AddressRange:        network.AddressRange,
			AddressRange6:       network.AddressRange6,
			DefaultKeepAlive:    time.Duration(network.DefaultKeepalive) * time.Second,
			DefaultACL:          network.DefaultACL,
			DefaultMTU:          network.DefaultMTU,
			AutoJoin:            autoJoin,
			AutoRemove:          autoRemove,
			AutoRemoveTags:      network.AutoRemoveTags,
			AutoRemoveThreshold: time.Duration(network.AutoRemoveThreshold) * time.Minute,
			JITEnabled:          false,
			NodesUpdatedAt:      time.Unix(network.NodesLastModified, 0),
			CreatedBy:           network.CreatedBy,
			CreatedAt:           network.CreatedAt,
			UpdatedAt:           time.Unix(network.NetworkLastModified, 0),
		}
		err = _network.Create(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}
