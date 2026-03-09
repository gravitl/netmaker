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

	// v1.5.1 migration includes migrating the users, groups, roles and networks tables.
	// future table migrations should be made below this block,
	// with a different version number and a similar check for whether the
	// migration was already done.
	migrationJob := &schema.Job{
		ID: "migration-v1.5.1",
	}
	err := migrationJob.Get(dbctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		logger.Log(1, fmt.Sprintf("running migration job %s", migrationJob.ID))
		// migrate.
		err = migrateV1_5_1(dbctx)
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

func migrateV1_5_1(ctx context.Context) error {
	err := migrateUsers(ctx)
	if err != nil {
		return err
	}

	err = migrateNetworks(ctx)
	if err != nil {
		return err
	}

	err = migrateUserRoles(ctx)
	if err != nil {
		return err
	}

	return migrateUserGroups(ctx)
}

func migrateUsers(ctx context.Context) error {
	records, err := database.FetchRecords(database.USERS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
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
			PlatformRoleID:             platformRoleID,
			ExternalIdentityProviderID: user.ExternalIdentityProviderID,
			AccountDisabled:            user.AccountDisabled,
			AuthType:                   user.AuthType,
			Password:                   user.Password,
			IsMFAEnabled:               user.IsMFAEnabled,
			TOTPSecret:                 user.TOTPSecret,
			LastLoginAt:                user.LastLoginTime,
			UserGroups:                 datatypes.NewJSONType(user.UserGroups),
			CreatedBy:                  user.CreatedBy,
			CreatedAt:                  user.CreatedAt,
			UpdatedAt:                  user.UpdatedAt,
		}

		logger.Log(4, fmt.Sprintf("migrating user %s", _user.Username))

		err = _user.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating user %s failed: %v", _user.Username, err))
			return err
		}
	}

	return nil
}

func migrateNetworks(ctx context.Context) error {
	records, err := database.FetchRecords(database.NETWORKS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var network models.Network
		err = json.Unmarshal([]byte(record), &network)
		if err != nil {
			return err
		}

		var autoJoin, autoRemove, jitEnabled bool

		if network.AutoJoin == "false" {
			autoJoin = false
		} else {
			autoJoin = true
		}

		if network.AutoRemove == "true" {
			autoRemove = true
		} else {
			autoRemove = false
		}

		if network.JITEnabled == "yes" {
			jitEnabled = true
		} else {
			jitEnabled = false
		}

		_network := &schema.Network{
			ID:                          "",
			Name:                        network.NetID,
			AddressRange:                network.AddressRange,
			AddressRange6:               network.AddressRange6,
			DefaultKeepAlive:            int(network.DefaultKeepalive),
			DefaultACL:                  network.DefaultACL,
			DefaultMTU:                  network.DefaultMTU,
			AutoJoin:                    autoJoin,
			AutoRemove:                  autoRemove,
			AutoRemoveTags:              network.AutoRemoveTags,
			AutoRemoveThreshold:         network.AutoRemoveThreshold,
			JITEnabled:                  jitEnabled,
			VirtualNATPoolIPv4:          network.VirtualNATPoolIPv4,
			VirtualNATSitePrefixLenIPv4: network.VirtualNATSitePrefixLenIPv4,
			NodesUpdatedAt:              time.Unix(network.NodesLastModified, 0),
			CreatedBy:                   network.CreatedBy,
			CreatedAt:                   network.CreatedAt,
			UpdatedAt:                   time.Unix(network.NetworkLastModified, 0),
		}

		logger.Log(4, fmt.Sprintf("migrating network %s", _network.Name))

		err = _network.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating network %s failed: %v", _network.Name, err))
			return err
		}
	}

	return nil
}

func migrateUserRoles(ctx context.Context) error {
	records, err := database.FetchRecords(database.USER_PERMISSIONS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var _userRole schema.UserRole
		err = json.Unmarshal([]byte(record), &_userRole)
		if err != nil {
			return err
		}

		logger.Log(4, fmt.Sprintf("migrating user role %s", _userRole.ID))

		err = _userRole.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating user role %s failed: %v", _userRole.ID, err))
			return err
		}
	}

	return nil
}

func migrateUserGroups(ctx context.Context) error {
	records, err := database.FetchRecords(database.USER_GROUPS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var _userGroup schema.UserGroup
		err = json.Unmarshal([]byte(record), &_userGroup)
		if err != nil {
			return err
		}

		logger.Log(4, fmt.Sprintf("migrating user group %s", _userGroup.ID))

		err = _userGroup.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating user group %s failed: %v", _userGroup.ID, err))
			return err
		}
	}

	return nil
}
