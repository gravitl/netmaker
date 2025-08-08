package logic

import (
	"encoding/json"
	"fmt"
	"time"

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

var MigrateToUUIDs = func() {}

var UpdateUserGwAccess = func(currentUser, changeUser models.User) {}

var UpdateRole = func(r models.UserRolePermissionTemplate) error { return nil }

var InitialiseRoles = userRolesInit
var IntialiseGroups = func() {}
var DeleteNetworkRoles = func(netID string) {}
var CreateDefaultNetworkRolesAndGroups = func(netID models.NetworkID) {}
var CreateDefaultUserPolicies = func(netID models.NetworkID) {
	if netID.String() == "" {
		return
	}
	if !IsAclExists(fmt.Sprintf("%s.%s", netID, "all-users")) {
		defaultUserAcl := models.Acl{
			ID:          fmt.Sprintf("%s.%s", netID, "all-users"),
			Default:     true,
			Name:        "All Users",
			MetaData:    "This policy gives access to everything in the network for an user",
			NetworkID:   netID,
			Proto:       models.ALL,
			ServiceType: models.Any,
			Port:        []string{},
			RuleType:    models.UserPolicy,
			Src: []models.AclPolicyTag{
				{
					ID:    models.UserAclID,
					Value: "*",
				},
			},
			Dst: []models.AclPolicyTag{{
				ID:    models.NodeTagID,
				Value: "*",
			}},
			AllowedDirection: models.TrafficDirectionUni,
			Enabled:          true,
			CreatedBy:        "auto",
			CreatedAt:        time.Now().UTC(),
		}
		InsertAcl(defaultUserAcl)
	}
}
var ListUserGroups = func() ([]models.UserGroup, error) { return nil, nil }
var GetUserGroupsInNetwork = func(netID models.NetworkID) (networkGrps map[models.UserGroupID]models.UserGroup) { return }
var GetUserGroup = func(groupId models.UserGroupID) (userGrps models.UserGroup, err error) { return }
var AddGlobalNetRolesToAdmins = func(u *models.User) {}
var EmailInit = func() {}

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

func GetAllRsrcIDForRsrc(rsrc models.RsrcType) models.RsrcID {
	switch rsrc {
	case models.HostRsrc:
		return models.AllHostRsrcID
	case models.RelayRsrc:
		return models.AllRelayRsrcID
	case models.RemoteAccessGwRsrc:
		return models.AllRemoteAccessGwRsrcID
	case models.ExtClientsRsrc:
		return models.AllExtClientsRsrcID
	case models.InetGwRsrc:
		return models.AllInetGwRsrcID
	case models.EgressGwRsrc:
		return models.AllEgressGwRsrcID
	case models.NetworkRsrc:
		return models.AllNetworkRsrcID
	case models.EnrollmentKeysRsrc:
		return models.AllEnrollmentKeysRsrcID
	case models.UserRsrc:
		return models.AllUserRsrcID
	case models.DnsRsrc:
		return models.AllDnsRsrcID
	case models.FailOverRsrc:
		return models.AllFailOverRsrcID
	case models.AclRsrc:
		return models.AllAclsRsrcID
	case models.TagRsrc:
		return models.AllTagsRsrcID
	}
	return ""
}

func userRolesInit() {
	d, _ := json.Marshal(SuperAdminPermissionTemplate)
	database.Insert(SuperAdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)
	d, _ = json.Marshal(AdminPermissionTemplate)
	database.Insert(AdminPermissionTemplate.ID.String(), string(d), database.USER_PERMISSIONS_TABLE_NAME)

}
