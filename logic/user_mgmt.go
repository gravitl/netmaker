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
var IsNetworkRolesValid = func(networkRoles map[models.NetworkID]map[models.UserRoleID]struct{}) error {
	return nil
}

var UpdateRole = func(r models.UserRolePermissionTemplate) error { return nil }

var InitialiseRoles = userRolesInit
var DeleteNetworkRoles = func(netID string) {}
var CreateDefaultNetworkRolesAndGroups = func(netID models.NetworkID) {}

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

func userRolesInit() {
	d, _ := json.Marshal(SuperAdminPermissionTemplate)
	database.Insert(SuperAdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(AdminPermissionTemplate)
	database.Insert(AdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)

}
