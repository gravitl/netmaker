package logic

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
)

// Pre-Define Permission Templates for default Roles
var SuperAdminPermissionTemplate = models.UserRolePermissionTemplate{
	ID:      models.SuperAdminRole,
	Default: true,
	DashBoardAcls: models.DashboardAccessControls{
		FullAccess: true,
	},
}
var AdminPermissionTemplate = models.UserRolePermissionTemplate{
	ID:      models.AdminRole,
	Default: true,
	DashBoardAcls: models.DashboardAccessControls{
		FullAccess: true,
	},
}

var NetworkAdminPermissionTemplate = models.UserRolePermissionTemplate{
	ID:      models.NetworkAdmin,
	Default: true,
	DashBoardAcls: models.DashboardAccessControls{
		NetworkLevelAccess: make(map[models.NetworkID]models.NetworkAccessControls),
	},
}

var NetworkUserPermissionTemplate = models.UserRolePermissionTemplate{
	ID:      models.NetworkUser,
	Default: true,
	DashBoardAcls: models.DashboardAccessControls{
		DenyDashboardAccess: true,
		NetworkLevelAccess:  make(map[models.NetworkID]models.NetworkAccessControls),
	},
}

func init() {
	d, _ := json.Marshal(SuperAdminPermissionTemplate)
	database.Insert(SuperAdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(AdminPermissionTemplate)
	database.Insert(AdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(NetworkAdminPermissionTemplate)
	database.Insert(NetworkAdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(NetworkUserPermissionTemplate)
	database.Insert(NetworkUserPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
}

// ListRoles - lists user roles permission templates
func ListRoles() ([]models.UserRolePermissionTemplate, error) {
	data, err := database.FetchRecords(database.USER_PERMISSIONS_TABLE_NAME)
	if err != nil {
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
