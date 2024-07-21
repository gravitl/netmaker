package logic

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
)

// Pre-Define Permission Templates for default Roles
var SuperAdminPermissionTemplate = models.UserRolePermissionTemplate{
	ID:         models.SuperAdminRole,
	Default:    true,
	FullAccess: true,
}

var AdminPermissionTemplate = models.UserRolePermissionTemplate{
	ID:         models.AdminRole,
	Default:    true,
	FullAccess: true,
}

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

var NetworkAdminPermissionTemplate = models.UserRolePermissionTemplate{
	ID:                 models.NetworkAdmin,
	Default:            true,
	NetworkID:          "netmaker",
	FullAccess:         true,
	NetworkLevelAccess: make(map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope),
}

var NetworkUserPermissionTemplate = models.UserRolePermissionTemplate{
	ID:                  models.NetworkUser,
	Default:             true,
	FullAccess:          false,
	NetworkID:           "netmaker",
	DenyDashboardAccess: false,
	NetworkLevelAccess: map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope{
		models.RemoteAccessGwRsrc: {
			models.AllRemoteAccessGwRsrcID: models.RsrcPermissionScope{
				Read: true,
			},
		},
		models.ExtClientsRsrc: {
			models.AllExtClientsRsrcID: models.RsrcPermissionScope{
				Read:      true,
				Create:    true,
				Update:    true,
				Delete:    true,
				VPNaccess: true,
			},
		},
	},
}

func UserRolesInit() {
	d, _ := json.Marshal(SuperAdminPermissionTemplate)
	database.Insert(SuperAdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(AdminPermissionTemplate)
	database.Insert(AdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(ServiceUserPermissionTemplate)
	database.Insert(ServiceUserPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(NetworkAdminPermissionTemplate)
	database.Insert(NetworkAdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(NetworkUserPermissionTemplate)
	database.Insert(NetworkUserPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
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
	_, err := GetRole(userRole.ID)
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
	roleInDB, err := GetRole(userRole.ID)
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

// GetRole - fetches role template by id
func GetRole(roleID models.UserRole) (models.UserRolePermissionTemplate, error) {
	// check if role already exists
	data, err := database.FetchRecord(database.USER_PERMISSIONS_TABLE_NAME, roleID.String())
	if err != nil {
		return models.UserRolePermissionTemplate{}, err
	}
	ur := models.UserRolePermissionTemplate{}
	err = json.Unmarshal([]byte(data), &ur)
	if err != nil {
		return ur, err
	}
	return ur, nil
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
	users, err := GetUsersDB()
	if err != nil {
		return err
	}
	role, err := GetRole(rid)
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
	role, err := GetRole(g.PlatformRole)
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
			role, err := GetRole(roleID)
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
	role, err := GetRole(g.PlatformRole)
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
			_, err := GetRole(roleID)
			if err != nil {
				err = fmt.Errorf("invalid network role")
				return err
			}
			if role.NetworkID == "" {
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
	users, err := GetUsersDB()
	if err != nil {
		return err
	}
	for _, user := range users {
		delete(user.UserGroups, gid)
		UpsertUser(user)
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
	gws = make(map[string]models.Node)
	userGwAccessScope := GetUserNetworkRolesWithRemoteVPNAccess(user)
	_, allNetAccess := userGwAccessScope["*"]
	nodes, err := GetAllNodes()
	if err != nil {
		return
	}
	for _, node := range nodes {
		if node.IsIngressGateway && !node.PendingDelete {
			if allNetAccess {
				gws[node.ID.String()] = node
			} else {
				gwRsrcMap := userGwAccessScope[models.NetworkID(node.Network)]
				scope, ok := gwRsrcMap[models.AllRemoteAccessGwRsrcID]
				if !ok {
					if _, ok := gwRsrcMap[models.RsrcID(node.ID.String())]; !ok {
						continue
					}
				}
				if scope.VPNaccess {
					gws[node.ID.String()] = node
				}

			}
		}
	}
	return
}

// GetUserNetworkRoles - get user network roles
func GetUserNetworkRolesWithRemoteVPNAccess(user models.User) (gwAccess map[models.NetworkID]map[models.RsrcID]models.RsrcPermissionScope) {
	gwAccess = make(map[models.NetworkID]map[models.RsrcID]models.RsrcPermissionScope)
	platformRole, err := GetRole(user.PlatformRoleID)
	if err != nil {
		return
	}
	if platformRole.FullAccess {
		gwAccess[models.NetworkID("*")] = make(map[models.RsrcID]models.RsrcPermissionScope)
		return
	}
	for netID, roleMap := range user.NetworkRoles {
		for roleID := range roleMap {
			role, err := GetRole(roleID)
			if err == nil {
				if role.FullAccess {
					gwAccess[netID] = map[models.RsrcID]models.RsrcPermissionScope{
						models.AllRemoteAccessGwRsrcID: {
							Create:    true,
							Read:      true,
							VPNaccess: true,
							Delete:    true,
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
								gwAccess[netID][gwID] = scope
							}
						}
					}

				}

			}
		}
	}
	return
}
