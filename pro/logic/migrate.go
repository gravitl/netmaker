package logic

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func MigrateToUUIDs() {
	groups, err := ListUserGroups()
	if err != nil {
		return
	}

	groupsMapping := make(map[models.UserGroupID]models.UserGroupID)

	for _, group := range groups {
		if group.Default {
			continue
		}

		_, err := uuid.Parse(string(group.ID))
		if err == nil {
			// group id is already an uuid, so no need to update
			continue
		}

		oldGroupID := group.ID
		group.ID = models.UserGroupID(uuid.NewString())
		groupsMapping[oldGroupID] = group.ID

		groupBytes, err := json.Marshal(group)
		if err != nil {
			continue
		}

		err = database.Insert(group.ID.String(), string(groupBytes), database.USER_GROUPS_TABLE_NAME)
		if err != nil {
			continue
		}

		err = database.DeleteRecord(database.USER_GROUPS_TABLE_NAME, oldGroupID.String())
		if err != nil {
			continue
		}
	}

	users, err := logic.GetUsersDB()
	if err != nil {
		return
	}

	for _, user := range users {
		userGroups := make(map[models.UserGroupID]struct{})
		for groupID := range user.UserGroups {
			newGroupID, ok := groupsMapping[groupID]
			if !ok {
				userGroups[groupID] = struct{}{}
			} else {
				userGroups[newGroupID] = struct{}{}
			}
		}

		user.UserGroups = userGroups

		err = logic.UpsertUser(user)
		if err != nil {
			continue
		}
	}

	for _, acl := range logic.ListAcls() {
		srcList := make([]models.AclPolicyTag, len(acl.Src))
		for i, src := range acl.Src {
			if src.ID == models.UserGroupAclID {
				newGroupID, ok := groupsMapping[models.UserGroupID(src.Value)]
				if ok {
					src.Value = newGroupID.String()
				}
			}

			srcList[i] = src
		}

		dstList := make([]models.AclPolicyTag, len(acl.Dst))
		for i, dst := range acl.Dst {
			if dst.ID == models.UserGroupAclID {
				newGroupID, ok := groupsMapping[models.UserGroupID(dst.Value)]
				if ok {
					dst.Value = newGroupID.String()
				}
			}

			dstList[i] = dst
		}

		err = logic.UpsertAcl(acl)
		if err != nil {
			continue
		}
	}

	invites, err := logic.ListUserInvites()
	if err != nil {
		return
	}

	for _, invite := range invites {
		userGroups := make(map[models.UserGroupID]struct{})
		for groupID := range invite.UserGroups {
			newGroupID, ok := groupsMapping[groupID]
			if !ok {
				invite.UserGroups[groupID] = struct{}{}
			} else {
				invite.UserGroups[newGroupID] = struct{}{}
			}
		}

		invite.UserGroups = userGroups

		err = logic.InsertUserInvite(invite)
		if err != nil {
			continue
		}
	}
}

func MigrateUserRoleAndGroups(user models.User) {
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
			var groupID models.UserGroupID
			if user.PlatformRoleID == models.ServiceUser {
				groupID = GetDefaultNetworkUserGroupID(models.NetworkID(gwNode.Network))
			} else {
				groupID = GetDefaultNetworkAdminGroupID(models.NetworkID(gwNode.Network))
			}
			if err != nil {
				continue
			}
			user.UserGroups[groupID] = struct{}{}
		}
	}
	if len(user.NetworkRoles) > 0 {
		for netID, netRoles := range user.NetworkRoles {
			var groupID models.UserGroupID
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
				groupID = GetDefaultNetworkUserGroupID(netID)
			} else {
				if adminAccess {
					groupID = GetDefaultNetworkAdminGroupID(netID)
				} else {
					groupID = GetDefaultNetworkUserGroupID(netID)
				}
			}
			user.UserGroups[groupID] = struct{}{}
			user.NetworkRoles = make(map[models.NetworkID]map[models.UserRoleID]struct{})
		}

	}
	logic.UpsertUser(user)
}
