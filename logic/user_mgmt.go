package logic

import (
	"encoding/json"
	"errors"

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

func UserRolesInit() {
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

// CreateRole - inserts new role into DB
func CreateRole(r models.UserRolePermissionTemplate) error {
	// check if role already exists
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
func GetRole(roleID string) (models.UserRolePermissionTemplate, error) {
	// check if role already exists
	data, err := database.FetchRecord(database.USER_PERMISSIONS_TABLE_NAME, roleID)
	if err != nil {
		return models.UserRolePermissionTemplate{}, errors.New("role already exists")
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
	users, err := GetUsersDB()
	if err != nil {
		return err
	}
	for _, user := range users {
		if user.GroupID != "" {
			// TODO - get permission template  of the group
			continue
		}
		if user.PermissionTemplate.ID == rid {
			errors.New("active roles cannot be deleted.switch existing users to a new role before deleting")
		}
	}
	return database.DeleteRecord(database.USER_PERMISSIONS_TABLE_NAME, rid.String())
}
