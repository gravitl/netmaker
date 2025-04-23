package logic

import (
	"encoding/json"

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

var GetFilteredNodesByUserAccess = func(user models.User, nodes []models.Node) (filteredNodes []models.Node) {
	return
}

var CreateRole = func(r models.UserRolePermissionTemplate) error {
	return nil
}

var DeleteRole = func(r models.UserRoleID, force bool) error {
	return nil
}

var FilterNetworksByRole = func(allnetworks []models.Network, user models.User) []models.Network {
	return allnetworks
}

var IsGroupsValid = func(groups map[models.UserGroupID]struct{}) error {
	return nil
}
var IsGroupValid = func(groupID models.UserGroupID) error {
	return nil
}
var IsNetworkRolesValid = func(networkRoles map[models.NetworkID]map[models.UserRoleID]struct{}) error {
	return nil
}

var MigrateUserRoleAndGroups = func(u models.User) {

}

var UpdateUserGwAccess = func(currentUser, changeUser models.User) {}

var UpdateRole = func(r models.UserRolePermissionTemplate) error { return nil }

var InitialiseRoles = userRolesInit
var IntialiseGroups = func() {}
var DeleteNetworkRoles = func(netID string) {}
var CreateDefaultNetworkRolesAndGroups = func(netID models.NetworkID) {}
var CreateDefaultUserPolicies = func(netID models.NetworkID) {}
var GetUserGroupsInNetwork = func(netID models.NetworkID) (networkGrps map[models.UserGroupID]models.UserGroup) { return }
var GetUserGroup = func(groupId models.UserGroupID) (userGrps models.UserGroup, err error) { return }
var AddGlobalNetRolesToAdmins = func(u *models.User) {}

// GetRole - fetches role template by id
func GetRole(roleID models.UserRoleID) (models.UserRolePermissionTemplate, error) {
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

func GetUserGrpMap() map[models.UserGroupID]map[string]struct{} {
	grpUsersMap := make(map[models.UserGroupID]map[string]struct{})
	users, _ := GetUsersDB()
	for _, user := range users {
		for gID := range user.UserGroups {
			if grpUsers, ok := grpUsersMap[gID]; ok {
				grpUsers[user.UserName] = struct{}{}
				grpUsersMap[gID] = grpUsers
			} else {
				grpUsersMap[gID] = make(map[string]struct{})
				grpUsersMap[gID][user.UserName] = struct{}{}
			}
		}

	}

	return grpUsersMap
}

func userRolesInit() {
	d, _ := json.Marshal(SuperAdminPermissionTemplate)
	database.Insert(SuperAdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(AdminPermissionTemplate)
	database.Insert(AdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)

}
