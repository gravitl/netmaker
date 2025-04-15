package auth

import (
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/pro/idp"
	"github.com/gravitl/netmaker/pro/idp/azure"
	"github.com/gravitl/netmaker/pro/idp/google"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"os"
)

var idpClient idp.Client

func InitializeIDP() error {
	if idpClient != nil {
		return nil
	}

	var err error

	switch os.Getenv("AUTH_PROVIDER") {
	case "google":
		idpClient, err = google.NewGoogleWorkspaceClient()
	case "azure-ad":
		idpClient, err = azure.NewAzureEntraIDClient()
	}

	return err
}

func SyncFromIDP() error {
	err := SyncUsers()
	if err != nil {
		return err
	}

	return SyncGroups()
}

func SyncUsers() error {
	idpUsers, err := idpClient.GetUsers()
	if err != nil {
		return err
	}

	dbUsers, err := logic.GetUsersDB()
	if err != nil {
		return err
	}

	password, err := logic.FetchPassValue("")
	if err != nil {
		return err
	}

	idpUsersMap := make(map[string]struct{})
	for _, user := range idpUsers {
		idpUsersMap[user.ID] = struct{}{}
	}

	dbUsersMap := make(map[string]struct{})
	for _, user := range dbUsers {
		dbUsersMap[user.UserName] = struct{}{}
	}

	for _, user := range idpUsers {
		if _, ok := dbUsersMap[user.Username]; !ok {
			// create the user only if it doesn't exist.
			err = logic.CreateUser(&models.User{
				UserName:                   user.Username,
				ExternalIdentityProviderID: user.ID,
				Password:                   password,
				AuthType:                   models.OAuth,
				PlatformRoleID:             models.PlatformUser,
			})
			if err != nil {
				return err
			}
		}
	}

	for _, user := range dbUsers {
		if user.ExternalIdentityProviderID != "" {
			if _, ok := idpUsersMap[user.ExternalIdentityProviderID]; !ok {
				// delete the user if it has been deleted on idp.
				_, err = logic.DeleteUser(user.UserName)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func SyncGroups() error {
	idpGroups, err := idpClient.GetGroups()
	if err != nil {
		return err
	}

	dbGroups, err := proLogic.ListUserGroups()
	if err != nil {
		return err
	}

	dbUsers, err := logic.GetUsersDB()
	if err != nil {
		return err
	}

	idpGroupsMap := make(map[string]struct{})
	for _, group := range idpGroups {
		idpGroupsMap[group.ID] = struct{}{}
	}

	dbGroupsMap := make(map[string]struct{})
	for _, group := range dbGroups {
		dbGroupsMap[group.ID.String()] = struct{}{}
	}

	dbUsersMap := make(map[string]models.User)
	for _, user := range dbUsers {
		if user.ExternalIdentityProviderID != "" {
			dbUsersMap[user.ExternalIdentityProviderID] = user
		}
	}

	modifiedUsers := make(map[string]struct{})

	for _, group := range idpGroups {
		if _, ok := dbGroupsMap[group.ID]; !ok {
			// create the group only if it doesn't exist.
			err := proLogic.CreateUserGroup(models.UserGroup{
				ID:                         models.UserGroupID(group.Name),
				ExternalIdentityProviderID: group.ID,
				Default:                    false,
				Name:                       group.Name,
			})
			if err != nil {
				return err
			}

			for _, member := range group.Members {
				dbUsersMap[member.ID].UserGroups[models.UserGroupID(group.Name)] = struct{}{}
				modifiedUsers[member.ID] = struct{}{}
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
