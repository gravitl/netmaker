package logic

import (
	"fmt"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

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
				g, err = GetUserGroup(models.UserGroupID(fmt.Sprintf("%s-%s-grp", gwNode.Network, models.NetworkUser)))
			} else {
				g, err = GetUserGroup(models.UserGroupID(fmt.Sprintf("%s-%s-grp",
					gwNode.Network, models.NetworkAdmin)))
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
				g, err = GetUserGroup(models.UserGroupID(fmt.Sprintf("%s-%s-grp", netID, models.NetworkUser)))
			} else {
				role := models.NetworkUser
				if adminAccess {
					role = models.NetworkAdmin
				}
				g, err = GetUserGroup(models.UserGroupID(fmt.Sprintf("%s-%s-grp",
					netID, role)))
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
