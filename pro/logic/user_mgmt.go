package logic

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/exp/slog"
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

func UserRolesInit() {
	d, _ := json.Marshal(ServiceUserPermissionTemplate)
	database.Insert(ServiceUserPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(PlatformUserUserPermissionTemplate)
	database.Insert(PlatformUserUserPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)

}

func CreateDefaultNetworkRoles(netID string) {
	var NetworkAdminPermissionTemplate = models.UserRolePermissionTemplate{
		ID:                 models.UserRole(fmt.Sprintf("%s_%s", netID, models.NetworkAdmin)),
		Default:            false,
		NetworkID:          netID,
		FullAccess:         true,
		NetworkLevelAccess: make(map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope),
	}

	var NetworkUserPermissionTemplate = models.UserRolePermissionTemplate{
		ID:                  models.UserRole(fmt.Sprintf("%s_%s", netID, models.NetworkUser)),
		Default:             false,
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
	userGs, _ := ListUserGroups()
	for _, userGI := range userGs {
		if _, ok := userGI.NetworkRoles[models.NetworkID(netID)]; ok {
			delete(userGI.NetworkRoles, models.NetworkID(netID))
			UpdateUserGroup(userGI)
		}
	}

	roles, _ := ListRoles()
	for _, role := range roles {
		if role.NetworkID == netID {
			DeleteRole(role.ID)
		}
	}
}

// ListRoles - lists user roles permission templates
func ListRoles() ([]models.UserRolePermissionTemplate, error) {
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
		userRoles = append(userRoles, userRole)
	}
	return userRoles, nil
}

func ValidateCreateRoleReq(userRole models.UserRolePermissionTemplate) error {
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
		}
	}
	if userRole.NetworkID == "" {
		return errors.New("only network roles are allowed to be created")
	}
	return nil
}

func ValidateUpdateRoleReq(userRole models.UserRolePermissionTemplate) error {
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
func DeleteRole(rid models.UserRole) error {
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
	if role.Default {
		return errors.New("cannot delete default role")
	}
	for _, user := range users {
		for userG := range user.UserGroups {
			ug, err := GetUserGroup(userG)
			if err == nil {
				if role.NetworkID != "" {
					for _, networkRoles := range ug.NetworkRoles {
						if _, ok := networkRoles[rid]; ok {
							err = errors.New("role cannot be deleted as active user groups are using this role")
							return err
						}
					}
				}

			}
		}

		if user.PlatformRoleID == rid {
			err = errors.New("active roles cannot be deleted.switch existing users to a new role before deleting")
			return err
		}
		for _, networkRoles := range user.NetworkRoles {
			if _, ok := networkRoles[rid]; ok {
				err = errors.New("active roles cannot be deleted.switch existing users to a new role before deleting")
				return err
			}

		}
	}
	return database.DeleteRecord(database.USER_PERMISSIONS_TABLE_NAME, rid.String())
}

func ValidateCreateGroupReq(g models.UserGroup) error {
	// check platform role is valid
	role, err := logic.GetRole(g.PlatformRole)
	if err != nil {
		err = fmt.Errorf("invalid platform role")
		return err
	}
	if role.NetworkID != "" {
		return errors.New("network role cannot be used as platform role")
	}
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
	// check platform role is valid
	role, err := logic.GetRole(g.PlatformRole)
	if err != nil {
		err = fmt.Errorf("invalid platform role")
		return err
	}
	if role.NetworkID != "" {
		return errors.New("network role cannot be used as platform role")
	}
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
	logger.Log(0, "------------> 8. getUserRemoteAccessGwsV1")
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
	logger.Log(0, "------------> 7.2 getUserRemoteAccessGwsV1")
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
	allNetworkRoles := []models.UserRole{}
	if len(user.NetworkRoles) > 0 {
		for _, netRoles := range user.NetworkRoles {
			for netRoleI := range netRoles {
				allNetworkRoles = append(allNetworkRoles, netRoleI)
			}
		}
	}
	if len(user.UserGroups) > 0 {
		for userGID := range user.UserGroups {
			userG, err := GetUserGroup(userGID)
			if err == nil {
				if len(userG.NetworkRoles) > 0 {
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
		networkNodes := logic.GetNetworkNodesMemory(nodes, userPermTemplate.NetworkID)
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
				allNetworkRoles[netID] = struct{}{}

			}
		}
		if len(user.UserGroups) > 0 {
			for userGID := range user.UserGroups {
				userG, err := GetUserGroup(userGID)
				if err == nil {
					if len(userG.NetworkRoles) > 0 {
						for netID := range userG.NetworkRoles {
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
	uniqueGroupsPlatformRole := make(map[models.UserRole]struct{})
	for groupID := range groups {
		userG, err := GetUserGroup(groupID)
		if err != nil {
			return err
		}
		uniqueGroupsPlatformRole[userG.PlatformRole] = struct{}{}
	}
	if len(uniqueGroupsPlatformRole) > 1 {

		return errors.New("only groups with same platform role can be assigned to an user")
	}
	return nil
}

func RemoveNetworkRoleFromUsers(host models.Host, node models.Node) {
	users, err := logic.GetUsersDB()
	if err == nil {
		for _, user := range users {
			// delete role from user
			if netRoles, ok := user.NetworkRoles[models.NetworkID(node.Network)]; ok {
				delete(netRoles, models.GetRAGRoleName(node.Network, host.Name))
				user.NetworkRoles[models.NetworkID(node.Network)] = netRoles
				err = logic.UpsertUser(user)
				if err != nil {
					slog.Error("failed to get user", "user", user.UserName, "error", err)
				}
			}
		}
	} else {
		slog.Error("failed to get users", "error", err)
	}
	err = DeleteRole(models.GetRAGRoleName(node.Network, host.Name))
	if err != nil {
		slog.Error("failed to delete role: ", models.GetRAGRoleName(node.Network, host.Name), err)
	}
}
