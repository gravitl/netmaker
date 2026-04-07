package migrate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func migrateV1_5_2(ctx context.Context) error {
	err := migratePendingUsers(ctx)
	if err != nil {
		return err
	}

	return migrateUserInvites(ctx)
}

func migratePendingUsers(ctx context.Context) error {
	records, err := database.FetchRecords(database.PENDING_USERS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var pendingUser models.User
		err = json.Unmarshal([]byte(record), &pendingUser)
		if err != nil {
			return err
		}

		_pendingUser := &schema.PendingUser{
			Username:                   pendingUser.UserName,
			ExternalIdentityProviderID: pendingUser.ExternalIdentityProviderID,
		}

		logger.Log(4, fmt.Sprintf("migrating pending user %s", _pendingUser.Username))

		err = _pendingUser.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating pending user %s failed: %v", _pendingUser.Username, err))
			return err
		}
	}

	return nil
}

func migrateUserInvites(ctx context.Context) error {
	return nil
}
