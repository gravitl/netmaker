package logic

import (
	"encoding/json"
	"errors"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
)

var GetFilteredNodesByUserAccess = func(user models.User, nodes []models.Node) (filteredNodes []models.Node) {
	return
}

var CreateRole = func(r models.UserRolePermissionTemplate) error {
	return nil
}
var DeleteNetworkRoles = func(netID string) {}

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

func IsGroupsValid(groups map[models.UserGroupID]struct{}) error {
	uniqueGroupsPlatformRole := make(map[models.UserRole]struct{})
	for groupID := range groups {
		userG, err := logic.GetUserGroup(groupID)
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
