package logic

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"

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
		groupMapping[oldGroupID] = group.ID

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
			newGroupID, ok := groupMapping[groupID]
			if !ok {
				userGroups[groupID] = struct{}{}
			} else {
				userGroups[newGroupID] = struct{}{}
			}
		}

		user.UserGroups = userGroups
		logic.UpsertUser(user)
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

func MigrateToGws() {
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return
	}
	for _, node := range nodes {
		if node.IsIngressGateway || node.IsRelay {
			node.IsGw = true
			node.IsIngressGateway = true
			node.IsRelay = true
			if node.Tags == nil {
				node.Tags = make(map[models.TagID]struct{})
			}
			node.Tags[models.TagID(fmt.Sprintf("%s.%s", node.Network, models.GwTagName))] = struct{}{}
			delete(node.Tags, models.TagID(fmt.Sprintf("%s.%s", node.Network, models.OldRemoteAccessTagName)))
			logic.UpsertNode(&node)
		}
	}
	acls := logic.ListAcls()
	for _, acl := range acls {
		upsert := false
		for i, srcI := range acl.Src {
			if srcI.ID == models.NodeTagID && srcI.Value == fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.OldRemoteAccessTagName) {
				srcI.Value = fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.GwTagName)
				acl.Src[i] = srcI
				upsert = true
			}
		}
		for i, dstI := range acl.Dst {
			if dstI.ID == models.NodeTagID && dstI.Value == fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.OldRemoteAccessTagName) {
				dstI.Value = fmt.Sprintf("%s.%s", acl.NetworkID.String(), models.GwTagName)
				acl.Dst[i] = dstI
				upsert = true
			}
		}
		if upsert {
			logic.UpsertAcl(acl)
		}
	}
	nets, _ := logic.GetNetworks()
	for _, netI := range nets {
		DeleteTag(models.TagID(fmt.Sprintf("%s.%s", netI.NetID, models.OldRemoteAccessTagName)), true)
	}
}
