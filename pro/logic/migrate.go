package logic

import (
	"github.com/google/uuid"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func MigrateGroups() {
	groups, err := ListUserGroups()
	if err != nil {
		return
	}

	groupMapping := make(map[models.UserGroupID]models.UserGroupID)

	for _, group := range groups {
		newGroupID := models.UserGroupID(uuid.NewString())
		groupMapping[group.ID] = newGroupID

		group.ID = newGroupID
		UpdateUserGroup(group)
	}

	users, err := logic.GetUsersDB()
	if err != nil {
		return
	}

	for _, user := range users {
		userGroups := make(map[models.UserGroupID]struct{})
		for groupID := range user.UserGroups {
			newGroupID := groupMapping[groupID]
			userGroups[newGroupID] = struct{}{}
		}

		user.UserGroups = userGroups
		logic.UpsertUser(user)
	}
}

func MigrateUserRoleAndGroups(user models.User) {
	var err error
	if user.PlatformRoleID == models.AdminRole || user.PlatformRoleID == models.SuperAdminRole {
		return
	}
	if len(user.RemoteGwIDs) > 0 {
		// define user roles for network
		// assign relevant network role to user
		for remoteGwID := range user.RemoteGwIDs {
			gwNode, err := logic.GetNodeByID(remoteGwID)
			if err != nil {
				continue
			}
			var g models.UserGroup
			if user.PlatformRoleID == models.ServiceUser {
				g, err = GetDefaultNetworkUserGroup(models.NetworkID(gwNode.Network))
			} else {
				g, err = GetDefaultNetworkAdminGroup(models.NetworkID(gwNode.Network))
			}
			if err != nil {
				continue
			}
			user.UserGroups[g.ID] = struct{}{}
		}
	}
	if len(user.NetworkRoles) > 0 {
		for netID, netRoles := range user.NetworkRoles {
			var g models.UserGroup
			adminAccess := false
			for netRoleID := range netRoles {
				permTemplate, err := logic.GetRole(netRoleID)
				if err == nil {
					if permTemplate.FullAccess {
						adminAccess = true
					}
				}
			}

			if user.PlatformRoleID == models.ServiceUser {
				g, err = GetDefaultNetworkUserGroup(netID)
			} else {
				if adminAccess {
					g, err = GetDefaultNetworkAdminGroup(netID)
				} else {
					g, err = GetDefaultNetworkUserGroup(netID)
				}
			}
			if err != nil {
				continue
			}
			user.UserGroups[g.ID] = struct{}{}
			user.NetworkRoles = make(map[models.NetworkID]map[models.UserRoleID]struct{})
		}

	}
	logic.UpsertUser(user)
}
