package logic

import (
	"encoding/json"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
)

// Pre-Define Permission Templates for default Roles
var SuperAdminPermissionTemplate = models.UserPermissionTemplate{
	ID:      models.SuperAdminRole,
	Default: true,
	DashBoardAcls: models.DashboardAccessControls{
		FullAccess: true,
	},
}
var AdminPermissionTemplate = models.UserPermissionTemplate{
	ID:      models.AdminRole,
	Default: true,
	DashBoardAcls: models.DashboardAccessControls{
		FullAccess: true,
	},
}

var NetworkAdminPermissionTemplate = models.UserPermissionTemplate{
	ID:      models.NetworkAdmin,
	Default: true,
	DashBoardAcls: models.DashboardAccessControls{
		NetworkLevelAccess: make(map[models.NetworkID]models.NetworkAccessControls),
	},
}

var NetworkUserPermissionTemplate = models.UserPermissionTemplate{
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
