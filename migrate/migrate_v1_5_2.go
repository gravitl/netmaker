package migrate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func migrateV1_5_2(ctx context.Context) error {
	err := migratePendingUsers(ctx)
	if err != nil {
		return err
	}

	err = migrateUserInvites(ctx)
	if err != nil {
		return err
	}

	return migrateNodes(ctx)
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
	records, err := database.FetchRecords(database.USER_INVITES_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var userInvite models.UserInvite
		err = json.Unmarshal([]byte(record), &userInvite)
		if err != nil {
			return err
		}

		_userInvite := &schema.UserInvite{
			InviteCode:     userInvite.InviteCode,
			InviteURL:      userInvite.InviteURL,
			Email:          userInvite.Email,
			PlatformRoleID: userInvite.PlatformRoleID,
			UserGroups:     datatypes.NewJSONType(userInvite.UserGroups),
		}

		logger.Log(4, fmt.Sprintf("migrating user invite %s", _userInvite.InviteCode))

		err = _userInvite.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating user invite (%s/%s) failed: %v", _userInvite.InviteCode, _userInvite.Email, err))
			return err
		}
	}

	return nil
}

func migrateNodes(ctx context.Context) error {
	records, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var node models.Node
		err = json.Unmarshal([]byte(record), &node)
		if err != nil {
			return err
		}

		// TODO: add nodes migration logic.
	}

	return nil
}
