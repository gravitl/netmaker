package migrate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/migrate/types"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	migrationJobV151 = "migration-v1.5.1"
	migrationJobV160 = "migration-v1.6.0"
	migrationJobV170 = "migration-v1.7.0"
)

// legacyMigrationJobIDs are historical migration job records that indicate an
// in-place upgrade (as opposed to a greenfield install with an empty jobs table).
var legacyMigrationJobIDs = []string{
	"migration-multitenancy",
	migrationJobV151,
	migrationJobV160,
	migrationJobV170,
}

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

// canRunMigrationV170 reports whether a v1.7.0 image may apply v1.6.0 migration in-process
// (greenfield installs and full legacy KV upgrade chains). Existing SQL deployments must
// complete migration-v1.6.0 on a v1.6.0 release image first.
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

// isServerVersionAtLeast170 reports whether this binary is a v1.7.0 release (used for
// the v1.6.0-before-v1.7.0 upgrade gate). Dev builds are treated as v1.7.0 but skip gating.
func isServerVersionAtLeast170() bool {
	return parseServerVersionConstraint(">= 1.7.0")
}

// enforceV160ReleaseGate reports whether v1.7.0 images must refuse startup when
// migration-v1.6.0 is incomplete on an existing SQL deployment (dev builds exempt).
func enforceV160ReleaseGate() bool {
	raw := servercfg.GetVersion()
	if raw == "" || raw == "dev" {
		return false
	}
	return isServerVersionAtLeast170()
}

func parseServerVersionConstraint(constraintStr string) bool {
	raw := strings.TrimSpace(servercfg.GetVersion())
	if raw == "" || raw == "dev" {
		return raw == "dev"
	}
	ver, err := semver.NewVersion(strings.TrimPrefix(raw, "v"))
	if err != nil {
		return false
	}
	c, err := semver.NewConstraint(constraintStr)
	if err != nil {
		return false
	}
	return c.Check(ver)
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

// anyMigrationJobPresent reports whether the jobs table records a prior schema migration
// (any known migration job id). An empty jobs table means greenfield install.
func anyMigrationJobPresent(ctx context.Context) (bool, error) {
	var count int64
	err := db.FromContext(ctx).Model(&schema.Job{}).
		Where("id IN ?", legacyMigrationJobIDs).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// isGreenfieldInstall is true when no migration jobs exist yet and there is no legacy
// KV/SQL data to upgrade (fresh database).
func isGreenfieldInstall(ctx context.Context, hasPriorMigrationJobs bool) (bool, error) {
	if hasPriorMigrationJobs {
		return false, nil
	}
	return isNewDeployment(ctx)
}

// markMigrationJobSkipped records a migration job complete without running its logic.
func markMigrationJobSkipped(ctx context.Context, jobID string) error {
	completed, err := migrationJobCompleted(ctx, jobID)
	if err != nil || completed {
		return err
	}
	return (&schema.Job{ID: jobID}).Create(ctx)
}

// shouldEnforceV160ReleaseGate applies the v1.6.0-before-v1.7.0 release gate only on
// v1.7.0 images upgrading an deployment that already has migration job history.
func shouldEnforceV160ReleaseGate(hasPriorMigrationJobs bool) bool {
	return enforceV160ReleaseGate() && hasPriorMigrationJobs
}

// enforceMigrationV170Compatibility returns ErrMigrationV160Required when a v1.7.0
// image cannot proceed on an existing SQL deployment until migration-v1.6.0
// completed on a v1.6.0 release. Call before running any migration steps.
func enforceMigrationV170Compatibility(
	ctx context.Context,
	hasPriorMigrationJobs, v151AlreadyComplete, v160AlreadyComplete bool,
) error {
	if v160AlreadyComplete {
		return nil
	}
	if !shouldEnforceV160ReleaseGate(hasPriorMigrationJobs) {
		return nil
	}
	ok, err := canRunMigrationV170(ctx, v160AlreadyComplete, v151AlreadyComplete)
	if err != nil {
		return err
	}
	if !ok {
		return ErrMigrationV160Required
	}
	return nil
}
