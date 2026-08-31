package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/migrate/types"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	migrationJobV151 = "migration-v1.5.1"
	migrationJobV160 = "migration-v1.6.0"
	migrationJobV170 = "migration-v1.7.0"
)

// ErrMigrationV160Required is returned when v1.7.0 migration is attempted before
// migration-v1.6.0 completed on a prior v1.6.0 deployment.
var ErrMigrationV160Required = errors.New(
	"migration-v1.6.0 must complete on Netmaker v1.6.0 before upgrading to v1.7.0; " +
	"deploy v1.6.0, restart the server successfully, then upgrade to v1.7.0",
)

func requireMigrationJobCompleted(ctx context.Context, jobID string) error {
	completed, err := migrationJobCompleted(ctx, jobID)
	if err != nil {
		return err
	}
	if !completed {
		return fmt.Errorf("%w (missing job %q)", ErrMigrationV160Required, jobID)
	}
	return nil
}

// canRunMigrationV170 reports whether v1.7.0 may run in this startup.
// Existing SQL deployments must have completed v1.6.0 on a prior startup.
// Greenfield installs and full legacy KV upgrade chains (v1.5.1 also ran here) are exempt.
func canRunMigrationV170(ctx context.Context, v160AlreadyComplete, v151AlreadyComplete bool) (bool, error) {
	if v160AlreadyComplete {
		return true, nil
	}
	newDeploy, err := isNewDeployment(ctx)
	if err != nil {
		return false, err
	}
	if newDeploy {
		return true, nil
	}
	if !v151AlreadyComplete {
		return true, nil
	}
	return false, nil
}

func getNetworkByNameForMigration(ctx context.Context, name string) (*schema.Network, error) {
	var network schema.Network
	err := db.FromContext(ctx).Model(&schema.Network{}).
		Where("name = ?", name).
		First(&network).Error
	if err != nil {
		return nil, err
	}
	return &network, nil
}

func ensureLegacyUserColumns(ctx context.Context) error {
	if !db.FromContext(ctx).Migrator().HasTable((&types.LegacyUser{}).TableName()) {
		return nil
	}
	return db.FromContext(ctx).AutoMigrate(&types.LegacyUser{})
}

func upsertLegacyUserAuth(
	ctx context.Context,
	userID string,
	user models.User,
	platformRoleID schema.UserRoleID,
	groups datatypes.JSONType[map[schema.UserGroupID]struct{}],
) error {
	if err := ensureLegacyUserColumns(ctx); err != nil {
		return err
	}

	legacy := types.LegacyUser{
		ID:                         userID,
		PlatformRoleID:             platformRoleID,
		UserGroups:                 groups,
		AuthType:                   user.AuthType,
		ExternalIdentityProviderID: user.ExternalIdentityProviderID,
		Password:                   user.Password,
		AccountDisabled:            user.AccountDisabled,
		IsMFAEnabled:               user.IsMFAEnabled,
		TOTPSecret:                 user.TOTPSecret,
	}
	return db.FromContext(ctx).Model(&types.LegacyUser{}).
		Where("id = ?", userID).
		Updates(&legacy).Error
}

func migrationJobCompleted(ctx context.Context, jobID string) (bool, error) {
	job := &schema.Job{ID: jobID}
	err := job.Get(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
