package auth

import (
	"context"
	"fmt"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/pro/idp"
	"github.com/gravitl/netmaker/pro/idp/azure"
	"github.com/gravitl/netmaker/pro/idp/google"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"strings"
	"sync"
	"time"
)

var (
	cancelSyncHook context.CancelFunc
	hookStopWg     sync.WaitGroup
)

func ResetIDPSyncHook() {
	if cancelSyncHook != nil {
		cancelSyncHook()
		hookStopWg.Wait()
		cancelSyncHook = nil
	}

	if logic.IsSyncEnabled() {
		ctx, cancel := context.WithCancel(context.Background())
		cancelSyncHook = cancel
		hookStopWg.Add(1)
		go runIDPSyncHook(ctx)
	}
}

func runIDPSyncHook(ctx context.Context) {
	defer hookStopWg.Done()
	ticker := time.NewTicker(logic.GetIDPSyncInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Log(0, "idp sync hook stopped")
			return
		case <-ticker.C:
			if err := SyncFromIDP(); err != nil {
				logger.Log(0, "failed to sync from idp: ", err.Error())
			} else {
				logger.Log(0, "sync from idp complete")
			}
		}
	}
}

func SyncFromIDP() error {
	settings := logic.GetServerSettings()

	var idpClient idp.Client
	var idpUsers []idp.User
	var idpGroups []idp.Group
	var err error

	switch settings.AuthProvider {
	case "google":
		idpClient, err = google.NewGoogleWorkspaceClient()
		if err != nil {
			return err
		}
	case "azure-ad":
		idpClient = azure.NewAzureEntraIDClient()
	default:
		if settings.AuthProvider != "" {
			return fmt.Errorf("invalid auth provider: %s", settings.AuthProvider)
		}
	}

	if settings.AuthProvider != "" && idpClient != nil {
		idpUsers, err = idpClient.GetUsers()
		if err != nil {
			return err
		}

		idpGroups, err = idpClient.GetGroups()
		if err != nil {
			return err
		}
	}

	err = syncUsers(idpUsers)
	if err != nil {
		return err
	}

	return syncGroups(idpGroups)
}

func syncUsers(idpUsers []idp.User) error {
	dbUsers, err := logic.GetUsersDB()
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	password, err := logic.FetchPassValue("")
	if err != nil {
		return err
	}

	idpUsersMap := make(map[string]struct{})
	for _, user := range idpUsers {
		idpUsersMap[user.Username] = struct{}{}
	}

	dbUsersMap := make(map[string]models.User)
	for _, user := range dbUsers {
		dbUsersMap[user.UserName] = user
	}

	filters := logic.GetServerSettings().UserFilters

	for _, user := range idpUsers {
		if user.AccountArchived {
			// delete the user if it has been archived.
			_ = logic.DeleteUser(user.Username)
			continue
		}

		var found bool
		for _, filter := range filters {
			if strings.HasPrefix(user.Username, filter) {
				found = true
				break
			}
		}

		// if there are filters but none of them match, then skip this user.
		if len(filters) > 0 && !found {
			continue
		}

		dbUser, ok := dbUsersMap[user.Username]
		if !ok {
			// create the user only if it doesn't exist.
			err = logic.CreateUser(&models.User{
				UserName:                   user.Username,
				ExternalIdentityProviderID: user.ID,
				DisplayName:                user.DisplayName,
				AccountDisabled:            user.AccountDisabled,
				Password:                   password,
				AuthType:                   models.OAuth,
				PlatformRoleID:             models.ServiceUser,
			})
			if err != nil {
				return err
			}

			// It's possible that a user can attempt to log in to Netmaker
			// after the IDP is configured but before the users are synced.
			// Since the user doesn't exist, a pending user will be
			// created. Now, since the user is created, the pending user
			// can be deleted.
			_ = logic.DeletePendingUser(user.Username)
		} else if dbUser.AuthType == models.OAuth {
			if dbUser.AccountDisabled != user.AccountDisabled ||
				dbUser.DisplayName != user.DisplayName ||
				dbUser.ExternalIdentityProviderID != user.ID {

				dbUser.AccountDisabled = user.AccountDisabled
				dbUser.DisplayName = user.DisplayName
				dbUser.ExternalIdentityProviderID = user.ID

				err = logic.UpsertUser(dbUser)
				if err != nil {
					return err
				}
			}
		} else {
			logger.Log(0, "user with username "+user.Username+" already exists, skipping creation")
			continue
		}
	}

	for _, user := range dbUsersMap {
		if user.ExternalIdentityProviderID == "" {
			continue
		}
		if _, ok := idpUsersMap[user.UserName]; !ok {
			// delete the user if it has been deleted on idp.
			err = logic.DeleteUser(user.UserName)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func syncGroups(idpGroups []idp.Group) error {
	dbGroups, err := proLogic.ListUserGroups()
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	dbUsers, err := logic.GetUsersDB()
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	idpGroupsMap := make(map[string]struct{})
	for _, group := range idpGroups {
		idpGroupsMap[group.ID] = struct{}{}
	}

	dbGroupsMap := make(map[string]models.UserGroup)
	for _, group := range dbGroups {
		if group.ExternalIdentityProviderID != "" {
			dbGroupsMap[group.ExternalIdentityProviderID] = group
		}
	}

	dbUsersMap := make(map[string]models.User)
	for _, user := range dbUsers {
		if user.ExternalIdentityProviderID != "" {
			dbUsersMap[user.ExternalIdentityProviderID] = user
		}
	}

	modifiedUsers := make(map[string]struct{})

	filters := logic.GetServerSettings().GroupFilters

	for _, group := range idpGroups {
		var found bool
		for _, filter := range filters {
			if strings.HasPrefix(group.Name, filter) {
				found = true
				break
			}
		}

		// if there are filters but none of them match, then skip this group.
		if len(filters) > 0 && !found {
			continue
		}

		dbGroup, ok := dbGroupsMap[group.ID]
		if !ok {
			dbGroup.ExternalIdentityProviderID = group.ID
			dbGroup.Name = group.Name
			dbGroup.Default = false
			dbGroup.NetworkRoles = make(map[models.NetworkID]map[models.UserRoleID]struct{})
			err := proLogic.CreateUserGroup(&dbGroup)
			if err != nil {
				return err
			}
		} else {
			dbGroup.Name = group.Name
			err = proLogic.UpdateUserGroup(dbGroup)
			if err != nil {
				return err
			}
		}

		groupMembersMap := make(map[string]struct{})
		for _, member := range group.Members {
			groupMembersMap[member] = struct{}{}
		}

		for _, user := range dbUsers {
			// use dbGroup.Name because the group name may have been changed on idp.
			_, inNetmakerGroup := user.UserGroups[dbGroup.ID]
			_, inIDPGroup := groupMembersMap[user.ExternalIdentityProviderID]

			if inNetmakerGroup && !inIDPGroup {
				// use dbGroup.Name because the group name may have been changed on idp.
				delete(dbUsersMap[user.ExternalIdentityProviderID].UserGroups, dbGroup.ID)
				modifiedUsers[user.ExternalIdentityProviderID] = struct{}{}
			}

			if !inNetmakerGroup && inIDPGroup {
				// use dbGroup.Name because the group name may have been changed on idp.
				dbUsersMap[user.ExternalIdentityProviderID].UserGroups[dbGroup.ID] = struct{}{}
				modifiedUsers[user.ExternalIdentityProviderID] = struct{}{}
			}
		}
	}

	for userID := range modifiedUsers {
		err = logic.UpsertUser(dbUsersMap[userID])
		if err != nil {
			return err
		}
	}

	for _, group := range dbGroups {
		if group.ExternalIdentityProviderID != "" {
			if _, ok := idpGroupsMap[group.ExternalIdentityProviderID]; !ok {
				// delete the group if it has been deleted on idp.
				err = proLogic.DeleteUserGroup(group.ID)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
