package logic

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

var ServiceUserPermissionTemplate = models.UserRolePermissionTemplate{
	ID:                  models.ServiceUser,
	Default:             true,
	FullAccess:          false,
	DenyDashboardAccess: true,
}

var PlatformUserUserPermissionTemplate = models.UserRolePermissionTemplate{
	ID:         models.PlatformUser,
	Default:    true,
	FullAccess: false,
}

var NetworkAdminAllPermissionTemplate = models.UserRolePermissionTemplate{
	ID:         models.UserRoleID(fmt.Sprintf("global-%s", models.NetworkAdmin)),
	Default:    true,
	FullAccess: true,
	NetworkID:  models.AllNetworks,
}

var NetworkUserAllPermissionTemplate = models.UserRolePermissionTemplate{
	ID:         models.UserRoleID(fmt.Sprintf("global-%s", models.NetworkUser)),
	Default:    true,
	FullAccess: false,
	NetworkID:  models.AllNetworks,
	NetworkLevelAccess: map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope{
		models.RemoteAccessGwRsrc: {
			models.AllRemoteAccessGwRsrcID: models.RsrcPermissionScope{
				Read:      true,
				VPNaccess: true,
			},
		},
		models.ExtClientsRsrc: {
			models.AllExtClientsRsrcID: models.RsrcPermissionScope{
				Read:     true,
				Create:   true,
				Update:   true,
				Delete:   true,
				SelfOnly: true,
			},
		},
	},
}

func UserRolesInit() {
	d, _ := json.Marshal(logic.SuperAdminPermissionTemplate)
	database.Insert(logic.SuperAdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(logic.AdminPermissionTemplate)
	database.Insert(logic.AdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(ServiceUserPermissionTemplate)
	database.Insert(ServiceUserPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(PlatformUserUserPermissionTemplate)
	database.Insert(PlatformUserUserPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(NetworkAdminAllPermissionTemplate)
	database.Insert(NetworkAdminAllPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(NetworkUserAllPermissionTemplate)
	database.Insert(NetworkUserAllPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)

}

func CreateDefaultNetworkRolesAndGroups(netID models.NetworkID) {
	var NetworkAdminPermissionTemplate = models.UserRolePermissionTemplate{
		ID:                 models.UserRoleID(fmt.Sprintf("%s-%s", netID, models.NetworkAdmin)),
		Default:            true,
		NetworkID:          netID,
		FullAccess:         true,
		NetworkLevelAccess: make(map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope),
	}

	var NetworkUserPermissionTemplate = models.UserRolePermissionTemplate{
		ID:                  models.UserRoleID(fmt.Sprintf("%s-%s", netID, models.NetworkUser)),
		Default:             true,
		FullAccess:          false,
		NetworkID:           netID,
		DenyDashboardAccess: false,
		NetworkLevelAccess: map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope{
			models.RemoteAccessGwRsrc: {
				models.AllRemoteAccessGwRsrcID: models.RsrcPermissionScope{
					Read:      true,
					VPNaccess: true,
				},
			},
			models.ExtClientsRsrc: {
				models.AllExtClientsRsrcID: models.RsrcPermissionScope{
					Read:     true,
					Create:   true,
					Update:   true,
					Delete:   true,
					SelfOnly: true,
				},
			},
		},
	}
	d, _ := json.Marshal(NetworkAdminPermissionTemplate)
	database.Insert(NetworkAdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(NetworkUserPermissionTemplate)
	database.Insert(NetworkUserPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)

	// create default network groups
	var NetworkAdminGroup = models.UserGroup{
		ID: models.UserGroupID(fmt.Sprintf("%s-%s-grp", netID, models.NetworkAdmin)),
		NetworkRoles: map[models.NetworkID]map[models.UserRoleID]struct{}{
			netID: {
				models.UserRoleID(fmt.Sprintf("%s-%s", netID, models.NetworkAdmin)): {},
			},
		},
		MetaData: "The network role was automatically created by Netmaker.",
	}
	var NetworkUserGroup = models.UserGroup{
		ID: models.UserGroupID(fmt.Sprintf("%s-%s-grp", netID, models.NetworkUser)),
		NetworkRoles: map[models.NetworkID]map[models.UserRoleID]struct{}{
			netID: {
				models.UserRoleID(fmt.Sprintf("%s-%s", netID, models.NetworkUser)): {},
			},
		},
		MetaData: "The network role was automatically created by Netmaker.",
	}
	d, _ = json.Marshal(NetworkAdminGroup)
	database.Insert(NetworkAdminGroup.ID.String(), string(d), database.USER_GROUPS_TABLE_NAME)
	d, _ = json.Marshal(NetworkUserGroup)
	database.Insert(NetworkUserGroup.ID.String(), string(d), database.USER_GROUPS_TABLE_NAME)
}

func DeleteNetworkRoles(netID string) {
	users, err := logic.GetUsersDB()
	if err != nil {
		return
	}
	for _, user := range users {
		if _, ok := user.NetworkRoles[models.NetworkID(netID)]; ok {
			delete(user.NetworkRoles, models.NetworkID(netID))
			logic.UpsertUser(user)
		}

	}
	database.DeleteRecord(database.USER_GROUPS_TABLE_NAME, fmt.Sprintf("%s-%s-grp", netID, models.NetworkUser))
	database.DeleteRecord(database.USER_GROUPS_TABLE_NAME, fmt.Sprintf("%s-%s-grp", netID, models.NetworkAdmin))
	userGs, _ := ListUserGroups()
	for _, userGI := range userGs {
		if _, ok := userGI.NetworkRoles[models.NetworkID(netID)]; ok {
			delete(userGI.NetworkRoles, models.NetworkID(netID))
			UpdateUserGroup(userGI)
		}
	}

	roles, _ := ListNetworkRoles()
	for _, role := range roles {
		if role.NetworkID.String() == netID {
			database.DeleteRecord(database.USER_PERMISSIONS_TABLE_NAME, role.ID.String())
		}
	}
}

// ListNetworkRoles - lists user network roles permission templates
func ListNetworkRoles() ([]models.UserRolePermissionTemplate, error) {
	data, err := database.FetchRecords(database.USER_PERMISSIONS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return []models.UserRolePermissionTemplate{}, err
	}
	userRoles := []models.UserRolePermissionTemplate{}
	for _, dataI := range data {
		userRole := models.UserRolePermissionTemplate{}
		err := json.Unmarshal([]byte(dataI), &userRole)
		if err != nil {
			continue
		}
		if userRole.NetworkID == "" {
			continue
		}
		userRoles = append(userRoles, userRole)
	}
	return userRoles, nil
}

// ListPlatformRoles - lists user platform roles permission templates
func ListPlatformRoles() ([]models.UserRolePermissionTemplate, error) {
	data, err := database.FetchRecords(database.USER_PERMISSIONS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return []models.UserRolePermissionTemplate{}, err
	}
	userRoles := []models.UserRolePermissionTemplate{}
	for _, dataI := range data {
		userRole := models.UserRolePermissionTemplate{}
		err := json.Unmarshal([]byte(dataI), &userRole)
		if err != nil {
			continue
		}
		if userRole.NetworkID != "" {
			continue
		}
		userRoles = append(userRoles, userRole)
	}
	return userRoles, nil
}

func ValidateCreateRoleReq(userRole *models.UserRolePermissionTemplate) error {
	// check if role exists with this id
	_, err := logic.GetRole(userRole.ID)
	if err == nil {
		return fmt.Errorf("role with id `%s` exists already", userRole.ID.String())
	}
	if len(userRole.NetworkLevelAccess) > 0 {
		for rsrcType := range userRole.NetworkLevelAccess {
			if _, ok := models.RsrcTypeMap[rsrcType]; !ok {
				return errors.New("invalid rsrc type " + rsrcType.String())
			}
			if rsrcType == models.RemoteAccessGwRsrc {
				userRsrcPermissions := userRole.NetworkLevelAccess[models.RemoteAccessGwRsrc]
				var vpnAccess bool
				for _, scope := range userRsrcPermissions {
					if scope.VPNaccess {
						vpnAccess = true
						break
					}
				}
				if vpnAccess {
					userRole.NetworkLevelAccess[models.ExtClientsRsrc] = map[models.RsrcID]models.RsrcPermissionScope{
						models.AllExtClientsRsrcID: {
							Read:     true,
							Create:   true,
							Update:   true,
							Delete:   true,
							SelfOnly: true,
						},
					}

				}

			}
		}
	}
	if userRole.NetworkID == "" {
		return errors.New("only network roles are allowed to be created")
	}
	return nil
}

func ValidateUpdateRoleReq(userRole *models.UserRolePermissionTemplate) error {
	roleInDB, err := logic.GetRole(userRole.ID)
	if err != nil {
		return err
	}
	if roleInDB.NetworkID != userRole.NetworkID {
		return errors.New("network id mismatch")
	}
	if roleInDB.Default {
		return errors.New("cannot update default role")
	}
	if len(userRole.NetworkLevelAccess) > 0 {
		for rsrcType := range userRole.NetworkLevelAccess {
			if _, ok := models.RsrcTypeMap[rsrcType]; !ok {
				return errors.New("invalid rsrc type " + rsrcType.String())
			}
			if rsrcType == models.RemoteAccessGwRsrc {
				userRsrcPermissions := userRole.NetworkLevelAccess[models.RemoteAccessGwRsrc]
				var vpnAccess bool
				for _, scope := range userRsrcPermissions {
					if scope.VPNaccess {
						vpnAccess = true
						break
					}
				}
				if vpnAccess {
					userRole.NetworkLevelAccess[models.ExtClientsRsrc] = map[models.RsrcID]models.RsrcPermissionScope{
						models.AllExtClientsRsrcID: {
							Read:     true,
							Create:   true,
							Update:   true,
							Delete:   true,
							SelfOnly: true,
						},
					}

				}

			}
		}
	}
	return nil
}

// CreateRole - inserts new role into DB
func CreateRole(r models.UserRolePermissionTemplate) error {
	// check if role already exists
	if r.ID.String() == "" {
		return errors.New("role id cannot be empty")
	}
	_, err := database.FetchRecord(database.USER_PERMISSIONS_TABLE_NAME, r.ID.String())
	if err == nil {
		return errors.New("role already exists")
	}
	d, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return database.Insert(r.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
}

// UpdateRole - updates role template
func UpdateRole(r models.UserRolePermissionTemplate) error {
	if r.ID.String() == "" {
		return errors.New("role id cannot be empty")
	}
	_, err := database.FetchRecord(database.USER_PERMISSIONS_TABLE_NAME, r.ID.String())
	if err != nil {
		return err
	}
	d, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return database.Insert(r.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
}

// DeleteRole - deletes user role
func DeleteRole(rid models.UserRoleID, force bool) error {
	if rid.String() == "" {
		return errors.New("role id cannot be empty")
	}
	users, err := logic.GetUsersDB()
	if err != nil {
		return err
	}
	role, err := logic.GetRole(rid)
	if err != nil {
		return err
	}
	if !force && role.Default {
		return errors.New("cannot delete default role")
	}
	for _, user := range users {
		for userG := range user.UserGroups {
			ug, err := GetUserGroup(userG)
			if err == nil {
				if role.NetworkID != "" {
					for netID, networkRoles := range ug.NetworkRoles {
						if _, ok := networkRoles[rid]; ok {
							delete(networkRoles, rid)
							ug.NetworkRoles[netID] = networkRoles
							UpdateUserGroup(ug)
						}

					}
				}

			}
		}

		if user.PlatformRoleID == rid {
			err = errors.New("active roles cannot be deleted.switch existing users to a new role before deleting")
			return err
		}
		if role.NetworkID != "" {
			for netID, networkRoles := range user.NetworkRoles {
				if _, ok := networkRoles[rid]; ok {
					delete(networkRoles, rid)
					user.NetworkRoles[netID] = networkRoles
					logic.UpsertUser(user)
				}

			}
		}
	}
	return database.DeleteRecord(database.USER_PERMISSIONS_TABLE_NAME, rid.String())
}

func ValidateCreateGroupReq(g models.UserGroup) error {

	// check if network roles are valid
	for _, roleMap := range g.NetworkRoles {
		for roleID := range roleMap {
			role, err := logic.GetRole(roleID)
			if err != nil {
				return fmt.Errorf("invalid network role %s", roleID)
			}
			if role.NetworkID == "" {
				return errors.New("platform role cannot be used as network role")
			}
		}
	}
	return nil
}
func ValidateUpdateGroupReq(g models.UserGroup) error {

	for networkID := range g.NetworkRoles {
		userRolesMap := g.NetworkRoles[networkID]
		for roleID := range userRolesMap {
			netRole, err := logic.GetRole(roleID)
			if err != nil {
				err = fmt.Errorf("invalid network role")
				return err
			}
			if netRole.NetworkID == "" {
				return errors.New("platform role cannot be used as network role")
			}
		}
	}
	return nil
}

// CreateUserGroup - creates new user group
func CreateUserGroup(g models.UserGroup) error {
	// check if role already exists
	if g.ID == "" {
		return errors.New("group id cannot be empty")
	}
	_, err := database.FetchRecord(database.USER_GROUPS_TABLE_NAME, g.ID.String())
	if err == nil {
		return errors.New("group already exists")
	}
	d, err := json.Marshal(g)
	if err != nil {
		return err
	}
	return database.Insert(g.ID.String(), string(d), database.USER_GROUPS_TABLE_NAME)
}

// GetUserGroup - fetches user group
func GetUserGroup(gid models.UserGroupID) (models.UserGroup, error) {
	d, err := database.FetchRecord(database.USER_GROUPS_TABLE_NAME, gid.String())
	if err != nil {
		return models.UserGroup{}, err
	}
	var ug models.UserGroup
	err = json.Unmarshal([]byte(d), &ug)
	if err != nil {
		return ug, err
	}
	return ug, nil
}

// ListUserGroups - lists user groups
func ListUserGroups() ([]models.UserGroup, error) {
	data, err := database.FetchRecords(database.USER_GROUPS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return []models.UserGroup{}, err
	}
	userGroups := []models.UserGroup{}
	for _, dataI := range data {
		userGroup := models.UserGroup{}
		err := json.Unmarshal([]byte(dataI), &userGroup)
		if err != nil {
			continue
		}
		userGroups = append(userGroups, userGroup)
	}
	return userGroups, nil
}

// UpdateUserGroup - updates new user group
func UpdateUserGroup(g models.UserGroup) error {
	// check if group exists
	if g.ID == "" {
		return errors.New("group id cannot be empty")
	}
	_, err := database.FetchRecord(database.USER_GROUPS_TABLE_NAME, g.ID.String())
	if err != nil {
		return err
	}
	d, err := json.Marshal(g)
	if err != nil {
		return err
	}
	return database.Insert(g.ID.String(), string(d), database.USER_GROUPS_TABLE_NAME)
}

// DeleteUserGroup - deletes user group
func DeleteUserGroup(gid models.UserGroupID) error {
	users, err := logic.GetUsersDB()
	if err != nil {
		return err
	}
	for _, user := range users {
		delete(user.UserGroups, gid)
		logic.UpsertUser(user)
	}
	return database.DeleteRecord(database.USER_GROUPS_TABLE_NAME, gid.String())
}

func HasNetworkRsrcScope(permissionTemplate models.UserRolePermissionTemplate, netid string, rsrcType models.RsrcType, rsrcID models.RsrcID, op string) bool {
	if permissionTemplate.FullAccess {
		return true
	}

	rsrcScope, ok := permissionTemplate.NetworkLevelAccess[rsrcType]
	if !ok {
		return false
	}
	_, ok = rsrcScope[rsrcID]
	return ok
}
func GetUserRAGNodes(user models.User) (gws map[string]models.Node) {
	logger.Log(0, "------------> 7. getUserRemoteAccessGwsV1")
	gws = make(map[string]models.Node)
	userGwAccessScope := GetUserNetworkRolesWithRemoteVPNAccess(user)
	logger.Log(0, fmt.Sprintf("User Gw Access Scope: %+v", userGwAccessScope))
	_, allNetAccess := userGwAccessScope["*"]
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return
	}
	logger.Log(0, fmt.Sprintf("------------> 8. getUserRemoteAccessGwsV1 %+v", allNetAccess))
	for _, node := range nodes {
		if node.IsIngressGateway && !node.PendingDelete {
			if allNetAccess {
				gws[node.ID.String()] = node
			} else {
				gwRsrcMap := userGwAccessScope[models.NetworkID(node.Network)]
				scope, ok := gwRsrcMap[models.AllRemoteAccessGwRsrcID]
				if !ok {
					if scope, ok = gwRsrcMap[models.RsrcID(node.ID.String())]; !ok {
						continue
					}
				}
				if scope.VPNaccess {
					gws[node.ID.String()] = node
				}

			}
		}
	}
	logger.Log(0, "------------> 9. getUserRemoteAccessGwsV1")
	return
}

// GetUserNetworkRoles - get user network roles
func GetUserNetworkRolesWithRemoteVPNAccess(user models.User) (gwAccess map[models.NetworkID]map[models.RsrcID]models.RsrcPermissionScope) {
	gwAccess = make(map[models.NetworkID]map[models.RsrcID]models.RsrcPermissionScope)
	logger.Log(0, "------------> 7.1 getUserRemoteAccessGwsV1")
	platformRole, err := logic.GetRole(user.PlatformRoleID)
	if err != nil {
		return
	}
	if platformRole.FullAccess {
		gwAccess[models.NetworkID("*")] = make(map[models.RsrcID]models.RsrcPermissionScope)
		return
	}
	if _, ok := user.NetworkRoles[models.AllNetworks]; ok {
		gwAccess[models.NetworkID("*")] = make(map[models.RsrcID]models.RsrcPermissionScope)
	}
	logger.Log(0, "------------> 7.2 getUserRemoteAccessGwsV1")
	if len(user.UserGroups) > 0 {
		for gID := range user.UserGroups {
			userG, err := GetUserGroup(gID)
			if err != nil {
				continue
			}
			for netID, roleMap := range userG.NetworkRoles {
				for roleID := range roleMap {
					role, err := logic.GetRole(roleID)
					if err == nil {
						if role.FullAccess {
							gwAccess[netID] = map[models.RsrcID]models.RsrcPermissionScope{
								models.AllRemoteAccessGwRsrcID: {
									Create:    true,
									Read:      true,
									Update:    true,
									VPNaccess: true,
									Delete:    true,
								},
								models.AllExtClientsRsrcID: {
									Create: true,
									Read:   true,
									Update: true,
									Delete: true,
								},
							}
							break
						}
						if rsrcsMap, ok := role.NetworkLevelAccess[models.RemoteAccessGwRsrc]; ok {
							if permissions, ok := rsrcsMap[models.AllRemoteAccessGwRsrcID]; ok && permissions.VPNaccess {
								if len(gwAccess[netID]) == 0 {
									gwAccess[netID] = make(map[models.RsrcID]models.RsrcPermissionScope)
								}
								gwAccess[netID][models.AllRemoteAccessGwRsrcID] = permissions
								break
							} else {
								for gwID, scope := range rsrcsMap {
									if scope.VPNaccess {
										if len(gwAccess[netID]) == 0 {
											gwAccess[netID] = make(map[models.RsrcID]models.RsrcPermissionScope)
										}
										gwAccess[netID][gwID] = scope
									}
								}
							}

						}

					}
				}
			}
		}
	}
	for netID, roleMap := range user.NetworkRoles {
		for roleID := range roleMap {
			role, err := logic.GetRole(roleID)
			if err == nil {
				if role.FullAccess {
					gwAccess[netID] = map[models.RsrcID]models.RsrcPermissionScope{
						models.AllRemoteAccessGwRsrcID: {
							Create:    true,
							Read:      true,
							Update:    true,
							VPNaccess: true,
							Delete:    true,
						},
						models.AllExtClientsRsrcID: {
							Create: true,
							Read:   true,
							Update: true,
							Delete: true,
						},
					}
					break
				}
				if rsrcsMap, ok := role.NetworkLevelAccess[models.RemoteAccessGwRsrc]; ok {
					if permissions, ok := rsrcsMap[models.AllRemoteAccessGwRsrcID]; ok && permissions.VPNaccess {
						if len(gwAccess[netID]) == 0 {
							gwAccess[netID] = make(map[models.RsrcID]models.RsrcPermissionScope)
						}
						gwAccess[netID][models.AllRemoteAccessGwRsrcID] = permissions
						break
					} else {
						for gwID, scope := range rsrcsMap {
							if scope.VPNaccess {
								if len(gwAccess[netID]) == 0 {
									gwAccess[netID] = make(map[models.RsrcID]models.RsrcPermissionScope)
								}
								gwAccess[netID][gwID] = scope
							}
						}
					}

				}

			}
		}
	}

	logger.Log(0, "------------> 7.3 getUserRemoteAccessGwsV1")
	return
}

func GetFilteredNodesByUserAccess(user models.User, nodes []models.Node) (filteredNodes []models.Node) {

	nodesMap := make(map[string]struct{})
	allNetworkRoles := []models.UserRoleID{}
	if len(user.NetworkRoles) > 0 {
		for _, netRoles := range user.NetworkRoles {
			for netRoleI := range netRoles {
				allNetworkRoles = append(allNetworkRoles, netRoleI)
			}
		}
	}
	if _, ok := user.NetworkRoles[models.AllNetworks]; ok {
		return nodes
	}
	if len(user.UserGroups) > 0 {
		for userGID := range user.UserGroups {
			userG, err := GetUserGroup(userGID)
			if err == nil {
				if len(userG.NetworkRoles) > 0 {
					if _, ok := userG.NetworkRoles[models.AllNetworks]; ok {
						return nodes
					}
					for _, netRoles := range userG.NetworkRoles {
						for netRoleI := range netRoles {
							allNetworkRoles = append(allNetworkRoles, netRoleI)
						}
					}
				}
			}
		}
	}
	for _, networkRoleID := range allNetworkRoles {
		userPermTemplate, err := logic.GetRole(networkRoleID)
		if err != nil {
			continue
		}
		networkNodes := logic.GetNetworkNodesMemory(nodes, userPermTemplate.NetworkID.String())
		if userPermTemplate.FullAccess {
			for _, node := range networkNodes {
				nodesMap[node.ID.String()] = struct{}{}
			}
			filteredNodes = append(filteredNodes, networkNodes...)
			continue
		}
		if rsrcPerms, ok := userPermTemplate.NetworkLevelAccess[models.RemoteAccessGwRsrc]; ok {
			if _, ok := rsrcPerms[models.AllRemoteAccessGwRsrcID]; ok {
				for _, node := range networkNodes {
					if _, ok := nodesMap[node.ID.String()]; ok {
						continue
					}
					if node.IsIngressGateway {
						nodesMap[node.ID.String()] = struct{}{}
						filteredNodes = append(filteredNodes, node)
					}
				}
			} else {
				for gwID, scope := range rsrcPerms {
					if _, ok := nodesMap[gwID.String()]; ok {
						continue
					}
					if scope.Read {
						gwNode, err := logic.GetNodeByID(gwID.String())
						if err == nil && gwNode.IsIngressGateway {
							filteredNodes = append(filteredNodes, gwNode)
						}
					}
				}
			}
		}

	}
	return
}

func FilterNetworksByRole(allnetworks []models.Network, user models.User) []models.Network {
	platformRole, err := logic.GetRole(user.PlatformRoleID)
	if err != nil {
		return []models.Network{}
	}
	if !platformRole.FullAccess {
		allNetworkRoles := make(map[models.NetworkID]struct{})
		if len(user.NetworkRoles) > 0 {
			for netID := range user.NetworkRoles {
				if netID == models.AllNetworks {
					return allnetworks
				}
				allNetworkRoles[netID] = struct{}{}

			}
		}
		if len(user.UserGroups) > 0 {
			for userGID := range user.UserGroups {
				userG, err := GetUserGroup(userGID)
				if err == nil {
					if len(userG.NetworkRoles) > 0 {
						for netID := range userG.NetworkRoles {
							if netID == models.AllNetworks {
								return allnetworks
							}
							allNetworkRoles[netID] = struct{}{}

						}
					}
				}
			}
		}
		filteredNetworks := []models.Network{}
		for _, networkI := range allnetworks {
			if _, ok := allNetworkRoles[models.NetworkID(networkI.NetID)]; ok {
				filteredNetworks = append(filteredNetworks, networkI)
			}
		}
		allnetworks = filteredNetworks
	}
	return allnetworks
}

func IsGroupsValid(groups map[models.UserGroupID]struct{}) error {

	for groupID := range groups {
		_, err := GetUserGroup(groupID)
		if err != nil {
			return fmt.Errorf("user group `%s` not found", groupID)
		}
	}
	return nil
}

func IsNetworkRolesValid(networkRoles map[models.NetworkID]map[models.UserRoleID]struct{}) error {
	for netID, netRoles := range networkRoles {

		if netID != models.AllNetworks {
			_, err := logic.GetNetwork(netID.String())
			if err != nil {
				return fmt.Errorf("failed to fetch network %s ", netID)
			}
		}
		for netRoleID := range netRoles {
			role, err := logic.GetRole(netRoleID)
			if err != nil {
				return fmt.Errorf("failed to fetch role %s ", netRoleID)
			}
			if role.NetworkID == "" {
				return fmt.Errorf("cannot use platform as network role %s", netRoleID)
			}
		}
	}
	return nil
}

// PrepareOauthUserFromInvite - init oauth user before create
func PrepareOauthUserFromInvite(in models.UserInvite) (models.User, error) {
	var newPass, fetchErr = logic.FetchPassValue("")
	if fetchErr != nil {
		return models.User{}, fetchErr
	}
	user := models.User{
		UserName: in.Email,
		Password: newPass,
	}
	user.UserGroups = in.UserGroups
	user.NetworkRoles = in.NetworkRoles
	user.PlatformRoleID = models.UserRoleID(in.PlatformRoleID)
	if user.PlatformRoleID == "" {
		user.PlatformRoleID = models.ServiceUser
	}
	return user, nil
}
