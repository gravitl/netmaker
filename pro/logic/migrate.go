package logic

import (
	"fmt"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func MigrateUserRoleAndGroups(user models.User) {
	var err error
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
		for netID := range user.NetworkRoles {
			var g models.UserGroup
			if user.PlatformRoleID == models.ServiceUser {
				g, err = GetUserGroup(models.UserGroupID(fmt.Sprintf("%s-%s-grp", netID, models.NetworkUser)))
			} else {
				g, err = GetUserGroup(models.UserGroupID(fmt.Sprintf("%s-%s-grp",
					netID, models.NetworkAdmin)))
			}
			if err != nil {
				continue
			}
			user.UserGroups[g.ID] = struct{}{}
			if err != nil {
				continue
			}
		}

	}
	logic.UpsertUser(user)
}
