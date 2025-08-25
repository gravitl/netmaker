package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

var (
	globalNetworksAdminGroupID = models.UserGroupID(fmt.Sprintf("global-%s-grp", models.NetworkAdmin))
	globalNetworksUserGroupID  = models.UserGroupID(fmt.Sprintf("global-%s-grp", models.NetworkUser))
	globalNetworksAdminRoleID  = models.UserRoleID(fmt.Sprintf("global-%s", models.NetworkAdmin))
	globalNetworksUserRoleID   = models.UserRoleID(fmt.Sprintf("global-%s", models.NetworkUser))
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
	GlobalLevelAccess: map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope{
		models.UserRsrc: {
			models.AllUserRsrcID: models.RsrcPermissionScope{
				Read: true,
			},
		},
	},
}

var NetworkAdminAllPermissionTemplate = models.UserRolePermissionTemplate{
	ID:         globalNetworksAdminRoleID,
	Name:       "Network Admins",
	MetaData:   "can manage configuration of all networks",
	Default:    true,
	FullAccess: true,
	NetworkID:  models.AllNetworks,
}

var NetworkUserAllPermissionTemplate = models.UserRolePermissionTemplate{
	ID:         globalNetworksUserRoleID,
	Name:       "Network Users",
	MetaData:   "Can connect to nodes in your networks via Netmaker Desktop App.",
	Default:    true,
	FullAccess: false,
	NetworkID:  models.AllNetworks,
	NetworkLevelAccess: map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope{
		models.HostRsrc: {
			models.AllHostRsrcID: models.RsrcPermissionScope{
				Read: true,
			},
		},
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
		models.DnsRsrc: {
			models.AllDnsRsrcID: models.RsrcPermissionScope{
				Read: true,
			},
		},
		models.AclRsrc: {
			models.AllAclsRsrcID: models.RsrcPermissionScope{
				Read: true,
			},
		},
		models.EgressGwRsrc: {
			models.AllEgressGwRsrcID: models.RsrcPermissionScope{
				Read: true,
			},
		},
		models.InetGwRsrc: {
			models.AllInetGwRsrcID: models.RsrcPermissionScope{
				Read: true,
			},
		},
		models.RelayRsrc: {
			models.AllRelayRsrcID: models.RsrcPermissionScope{
				Read: true,
			},
		},
		models.TagRsrc: {
			models.AllTagsRsrcID: models.RsrcPermissionScope{
				Read: true,
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

func UserGroupsInit() {
	// create default network groups
	var NetworkGlobalAdminGroup = models.UserGroup{
		ID:       globalNetworksAdminGroupID,
		Default:  true,
		Name:     "All Networks Admin Group",
		MetaData: "can manage configuration of all networks",
		NetworkRoles: map[models.NetworkID]map[models.UserRoleID]struct{}{
			models.AllNetworks: {
				globalNetworksAdminRoleID: {},
			},
		},
	}
	var NetworkGlobalUserGroup = models.UserGroup{
		ID:      globalNetworksUserGroupID,
		Name:    "All Networks User Group",
		Default: true,
		NetworkRoles: map[models.NetworkID]map[models.UserRoleID]struct{}{
			models.AllNetworks: {
				globalNetworksUserRoleID: {},
			},
		},
		MetaData: "Provides read-only dashboard access to platform users and allows connection to network nodes via the Netmaker Desktop App.",
	}
	d, _ := json.Marshal(NetworkGlobalAdminGroup)
	database.Insert(NetworkGlobalAdminGroup.ID.String(), string(d), database.USER_GROUPS_TABLE_NAME)
	d, _ = json.Marshal(NetworkGlobalUserGroup)
	database.Insert(NetworkGlobalUserGroup.ID.String(), string(d), database.USER_GROUPS_TABLE_NAME)
}

func CreateDefaultNetworkRolesAndGroups(netID models.NetworkID) {
	if netID.String() == "" {
		return
	}
	var NetworkAdminPermissionTemplate = models.UserRolePermissionTemplate{
		ID:                 GetDefaultNetworkAdminRoleID(netID),
		Name:               fmt.Sprintf("%s Admin", netID),
		MetaData:           fmt.Sprintf("can manage your network `%s` configuration.", netID),
		Default:            true,
		NetworkID:          netID,
		FullAccess:         true,
		NetworkLevelAccess: make(map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope),
	}

	var NetworkUserPermissionTemplate = models.UserRolePermissionTemplate{
		ID:                  GetDefaultNetworkUserRoleID(netID),
		Name:                fmt.Sprintf("%s User", netID),
		MetaData:            fmt.Sprintf("Can connect to nodes in your network `%s` via Netmaker Desktop App.", netID),
		Default:             true,
		FullAccess:          false,
		NetworkID:           netID,
		DenyDashboardAccess: false,
		NetworkLevelAccess: map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope{
			models.HostRsrc: {
				models.AllHostRsrcID: models.RsrcPermissionScope{
					Read: true,
				},
			},
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
			models.DnsRsrc: {
				models.AllDnsRsrcID: models.RsrcPermissionScope{
					Read: true,
				},
			},
			models.AclRsrc: {
				models.AllAclsRsrcID: models.RsrcPermissionScope{
					Read: true,
				},
			},
			models.EgressGwRsrc: {
				models.AllEgressGwRsrcID: models.RsrcPermissionScope{
					Read: true,
				},
			},
			models.InetGwRsrc: {
				models.AllInetGwRsrcID: models.RsrcPermissionScope{
					Read: true,
				},
			},
			models.RelayRsrc: {
				models.AllRelayRsrcID: models.RsrcPermissionScope{
					Read: true,
				},
			},
			models.TagRsrc: {
				models.AllTagsRsrcID: models.RsrcPermissionScope{
					Read: true,
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
		ID:      GetDefaultNetworkAdminGroupID(netID),
		Name:    fmt.Sprintf("%s Admin Group", netID),
		Default: true,
		NetworkRoles: map[models.NetworkID]map[models.UserRoleID]struct{}{
			netID: {
				GetDefaultNetworkAdminRoleID(netID): {},
			},
		},
		MetaData: fmt.Sprintf("can manage your network `%s` configuration including adding and removing devices.", netID),
	}
	var NetworkUserGroup = models.UserGroup{
		ID:      GetDefaultNetworkUserGroupID(netID),
		Name:    fmt.Sprintf("%s User Group", netID),
		Default: true,
		NetworkRoles: map[models.NetworkID]map[models.UserRoleID]struct{}{
			netID: {
				GetDefaultNetworkUserRoleID(netID): {},
			},
		},
		MetaData: fmt.Sprintf("Can connect to nodes in your network `%s` via Netmaker Desktop App. Platform users will have read-only access to the the dashboard.", netID),
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

	defaultAdminGrpID := GetDefaultNetworkAdminGroupID(models.NetworkID(netID))
	defaultUserGrpID := GetDefaultNetworkUserGroupID(models.NetworkID(netID))
	for _, user := range users {
		var upsert bool
		if _, ok := user.NetworkRoles[models.NetworkID(netID)]; ok {
			delete(user.NetworkRoles, models.NetworkID(netID))
			upsert = true
		}
		if _, ok := user.UserGroups[defaultUserGrpID]; ok {
			delete(user.UserGroups, defaultUserGrpID)
			upsert = true
		}
		if _, ok := user.UserGroups[defaultAdminGrpID]; ok {
			delete(user.UserGroups, defaultAdminGrpID)
			upsert = true
		}
		if upsert {
			logic.UpsertUser(user)
		}
	}
	database.DeleteRecord(database.USER_GROUPS_TABLE_NAME, defaultUserGrpID.String())
	database.DeleteRecord(database.USER_GROUPS_TABLE_NAME, defaultAdminGrpID.String())
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
	// default roles are currently created directly in the db.
	// this check is only to prevent future errors.
	if r.Default && r.ID == "" {
		return errors.New("role id cannot be empty for default role")
	}

	if !r.Default {
		r.ID = models.UserRoleID(uuid.NewString())
	}

	// check if the role already exists
	if r.Name == "" {
		return errors.New("role name cannot be empty")
	}

	roles, err := ListNetworkRoles()
	if err != nil {
		return err
	}

	for _, role := range roles {
		if role.Name == r.Name {
			return errors.New("role already exists")
		}
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
	if role.NetworkID == "" {
		return errors.New("cannot delete platform role")
	}
	// allow deletion of default network roles if network doesn't exist
	if role.NetworkID == models.AllNetworks {
		return errors.New("cannot delete default network role")
	}
	// check if network exists
	exists, _ := logic.NetworkExists(role.NetworkID.String())
	if role.Default {
		if exists && !force {
			return errors.New("cannot delete default role")
		}
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
func CreateUserGroup(g *models.UserGroup) error {
	// default groups are currently created directly in the db.
	// this check is only to prevent future errors.
	if g.Default && g.ID == "" {
		return errors.New("group id cannot be empty for default group")
	}

	if !g.Default {
		g.ID = models.UserGroupID(uuid.NewString())
	}

	// check if the group already exists
	if g.Name == "" {
		return errors.New("group name cannot be empty")
	}
	groups, err := ListUserGroups()
	if err != nil {
		return err
	}

	for _, group := range groups {
		if group.Name == g.Name {
			return errors.New("group already exists")
		}
	}

	d, err := json.Marshal(g)
	if err != nil {
		return err
	}
	err = database.Insert(g.ID.String(), string(d), database.USER_GROUPS_TABLE_NAME)
	if err != nil {
		return err
	}
	// create default network gateway policies
	CreateDefaultUserGroupNetworkPolicies(*g)
	return nil
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

func GetDefaultNetworkAdminGroupID(networkID models.NetworkID) models.UserGroupID {
	return models.UserGroupID(fmt.Sprintf("%s-%s-grp", networkID, models.NetworkAdmin))
}

func GetDefaultNetworkUserGroupID(networkID models.NetworkID) models.UserGroupID {
	return models.UserGroupID(fmt.Sprintf("%s-%s-grp", networkID, models.NetworkUser))
}

func GetDefaultNetworkAdminRoleID(networkID models.NetworkID) models.UserRoleID {
	return models.UserRoleID(fmt.Sprintf("%s-%s", networkID, models.NetworkAdmin))
}

func GetDefaultNetworkUserRoleID(networkID models.NetworkID) models.UserRoleID {
	return models.UserRoleID(fmt.Sprintf("%s-%s", networkID, models.NetworkUser))
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
	// check if the group exists
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
	g, err := GetUserGroup(gid)
	if err != nil {
		return err
	}
	users, err := logic.GetUsersDB()
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}
	for _, user := range users {
		delete(user.UserGroups, gid)
		logic.UpsertUser(user)
	}
	// create default network gateway policies
	DeleteDefaultUserGroupNetworkPolicies(g)
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

func GetUserRAGNodesV1(user models.User) (gws map[string]models.Node) {
	gws = make(map[string]models.Node)
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return
	}
	if user.PlatformRoleID == models.AdminRole || user.PlatformRoleID == models.SuperAdminRole {
		for _, node := range nodes {
			if node.IsIngressGateway {
				gws[node.ID.String()] = node
			}

		}
	}
	tagNodesMap := GetTagMapWithNodes()
	accessPolices := ListUserPolicies(user)
	for _, policyI := range accessPolices {
		if !policyI.Enabled {
			continue
		}
		for _, dstI := range policyI.Dst {
			if dstI.Value == "*" {
				networkNodes := logic.GetNetworkNodesMemory(nodes, policyI.NetworkID.String())
				for _, node := range networkNodes {
					if node.IsIngressGateway {
						gws[node.ID.String()] = node
					}
				}
			}
			if nodes, ok := tagNodesMap[models.TagID(dstI.Value)]; ok {
				for _, node := range nodes {
					if node.IsIngressGateway {
						gws[node.ID.String()] = node
					}

				}
			}
		}
	}
	return
}

func GetUserRAGNodes(user models.User) (gws map[string]models.Node) {
	gws = make(map[string]models.Node)
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return
	}

	for _, node := range nodes {
		if !node.IsGw {
			continue
		}
		if user.PlatformRoleID == models.AdminRole || user.PlatformRoleID == models.SuperAdminRole {
			if ok, _ := IsUserAllowedToCommunicate(user.UserName, node); ok {
				gws[node.ID.String()] = node
				continue
			}
		} else {
			// check if user has network role assigned
			if roles, ok := user.NetworkRoles[models.NetworkID(node.Network)]; ok && len(roles) > 0 {
				if ok, _ := IsUserAllowedToCommunicate(user.UserName, node); ok {
					gws[node.ID.String()] = node
					continue
				}
			}
			if roles, ok := user.NetworkRoles[models.AllNetworks]; ok && len(roles) > 0 {
				if ok, _ := IsUserAllowedToCommunicate(user.UserName, node); ok {
					gws[node.ID.String()] = node
					continue
				}
			}
			for groupID := range user.UserGroups {
				userGrp, err := logic.GetUserGroup(groupID)
				if err == nil {
					if roles, ok := userGrp.NetworkRoles[models.NetworkID(node.Network)]; ok && len(roles) > 0 {
						if ok, _ := IsUserAllowedToCommunicate(user.UserName, node); ok {
							gws[node.ID.String()] = node
							break
						}
					}
					if roles, ok := userGrp.NetworkRoles[models.AllNetworks]; ok && len(roles) > 0 {
						if ok, _ := IsUserAllowedToCommunicate(user.UserName, node); ok {
							gws[node.ID.String()] = node
							break
						}
					}
				}
			}
		}
	}
	return
}

// GetUserNetworkRoles - get user network roles
func GetUserNetworkRolesWithRemoteVPNAccess(user models.User) (gwAccess map[models.NetworkID]map[models.RsrcID]models.RsrcPermissionScope) {
	gwAccess = make(map[models.NetworkID]map[models.RsrcID]models.RsrcPermissionScope)
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
		return
	}
	if len(user.UserGroups) > 0 {
		for gID := range user.UserGroups {
			userG, err := GetUserGroup(gID)
			if err != nil {
				continue
			}
			if _, ok := userG.NetworkRoles[models.AllNetworks]; ok {
				gwAccess[models.NetworkID("*")] = make(map[models.RsrcID]models.RsrcPermissionScope)
				return
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

	return
}

func GetFilteredNodesByUserAccess(user models.User, nodes []models.Node) (filteredNodes []models.Node) {
	return filteredNodes
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

func IsGroupValid(groupID models.UserGroupID) error {

	_, err := GetUserGroup(groupID)
	if err != nil {
		return fmt.Errorf("user group `%s` not found", groupID)
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

func UpdatesUserGwAccessOnRoleUpdates(currNetworkAccess,
	changeNetworkAccess map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope, netID string) {
	networkChangeMap := make(map[models.RsrcID]models.RsrcPermissionScope)
	for rsrcType, RsrcPermsMap := range currNetworkAccess {
		if rsrcType != models.RemoteAccessGwRsrc {
			continue
		}
		if _, ok := changeNetworkAccess[rsrcType]; !ok {
			for rsrcID, scope := range RsrcPermsMap {
				networkChangeMap[rsrcID] = scope
			}
		} else {
			for rsrcID, scope := range RsrcPermsMap {
				if _, ok := changeNetworkAccess[rsrcType][rsrcID]; !ok {
					networkChangeMap[rsrcID] = scope
				}
			}
		}
	}

	extclients, err := logic.GetAllExtClients()
	if err != nil {
		slog.Error("failed to fetch extclients", "error", err)
		return
	}
	userMap, err := logic.GetUserMap()
	if err != nil {
		return
	}
	for _, extclient := range extclients {
		if extclient.Network != netID {
			continue
		}
		if _, ok := networkChangeMap[models.AllRemoteAccessGwRsrcID]; ok {
			if user, ok := userMap[extclient.OwnerID]; ok {
				if user.PlatformRoleID != models.ServiceUser {
					continue
				}
				err = logic.DeleteExtClientAndCleanup(extclient)
				if err != nil {
					slog.Error("failed to delete extclient",
						"id", extclient.ClientID, "owner", user.UserName, "error", err)
				} else {
					if err := mq.PublishDeletedClientPeerUpdate(&extclient); err != nil {
						slog.Error("error setting ext peers: " + err.Error())
					}
				}
			}
			continue
		}
		if _, ok := networkChangeMap[models.RsrcID(extclient.IngressGatewayID)]; ok {
			if user, ok := userMap[extclient.OwnerID]; ok {
				if user.PlatformRoleID != models.ServiceUser {
					continue
				}
				err = logic.DeleteExtClientAndCleanup(extclient)
				if err != nil {
					slog.Error("failed to delete extclient",
						"id", extclient.ClientID, "owner", user.UserName, "error", err)
				} else {
					if err := mq.PublishDeletedClientPeerUpdate(&extclient); err != nil {
						slog.Error("error setting ext peers: " + err.Error())
					}
				}
			}

		}

	}
	if servercfg.IsDNSMode() {
		logic.SetDNS()
	}
}

func UpdatesUserGwAccessOnGrpUpdates(groupID models.UserGroupID, oldNetworkRoles, newNetworkRoles map[models.NetworkID]map[models.UserRoleID]struct{}) {
	networkRemovedMap := make(map[models.NetworkID]struct{})
	for netID := range oldNetworkRoles {
		if _, ok := newNetworkRoles[netID]; !ok {
			networkRemovedMap[netID] = struct{}{}
		}
	}

	extclients, err := logic.GetAllExtClients()
	if err != nil {
		slog.Error("failed to fetch extclients", "error", err)
		return
	}
	userMap, err := logic.GetUserMap()
	if err != nil {
		return
	}

	for _, extclient := range extclients {
		var shouldDelete bool
		user, ok := userMap[extclient.OwnerID]
		if !ok {
			// user does not exist, delete extclient.
			shouldDelete = true
		} else {
			if user.PlatformRoleID == models.SuperAdminRole || user.PlatformRoleID == models.AdminRole {
				// Super-admin and Admin's access is not determined by group membership
				// or network roles. Even if a network is removed from the group, they
				// continue to have access to the network.
				// So, no need to delete the extclient.
				shouldDelete = false
			} else {
				_, hasAccess := user.NetworkRoles[models.NetworkID(extclient.Network)]
				if hasAccess {
					// The user has access to the network by themselves and not by
					// virtue of being a member of the group.
					// So, no need to delete the extclient.
					shouldDelete = false
				} else {
					_, userInGroup := user.UserGroups[groupID]
					_, networkRemoved := networkRemovedMap[models.NetworkID(extclient.Network)]
					if userInGroup && networkRemoved {
						// This group no longer provides it's members access to the
						// network.
						// This user is a member of the group and has no direct
						// access to the network (either by its platform role or by
						// network roles).
						// So, delete the extclient.
						shouldDelete = true
					}
				}
			}
		}

		if shouldDelete {
			err = logic.DeleteExtClientAndCleanup(extclient)
			if err != nil {
				slog.Error("failed to delete extclient",
					"id", extclient.ClientID, "owner", user.UserName, "error", err)
			} else {
				if err := mq.PublishDeletedClientPeerUpdate(&extclient); err != nil {
					slog.Error("error setting ext peers: " + err.Error())
				}
			}
		}
	}

	if servercfg.IsDNSMode() {
		logic.SetDNS()
	}

}

func UpdateUserGwAccess(currentUser, changeUser models.User) {
	if changeUser.PlatformRoleID != models.ServiceUser {
		return
	}

	networkChangeMap := make(map[models.NetworkID]map[models.UserRoleID]struct{})
	for netID, networkUserRoles := range currentUser.NetworkRoles {
		if _, ok := changeUser.NetworkRoles[netID]; !ok {
			for netRoleID := range networkUserRoles {
				if _, ok := networkChangeMap[netID]; !ok {
					networkChangeMap[netID] = make(map[models.UserRoleID]struct{})
				}
				networkChangeMap[netID][netRoleID] = struct{}{}
			}
		} else {
			for netRoleID := range networkUserRoles {
				if _, ok := changeUser.NetworkRoles[netID][netRoleID]; !ok {
					if _, ok := networkChangeMap[netID]; !ok {
						networkChangeMap[netID] = make(map[models.UserRoleID]struct{})
					}
					networkChangeMap[netID][netRoleID] = struct{}{}
				}
			}
		}
	}
	for gID := range currentUser.UserGroups {
		if _, ok := changeUser.UserGroups[gID]; ok {
			continue
		}
		userG, err := GetUserGroup(gID)
		if err == nil {
			for netID, networkUserRoles := range userG.NetworkRoles {
				for netRoleID := range networkUserRoles {
					if _, ok := networkChangeMap[netID]; !ok {
						networkChangeMap[netID] = make(map[models.UserRoleID]struct{})
					}
					networkChangeMap[netID][netRoleID] = struct{}{}
				}
			}
		}
	}
	if len(networkChangeMap) == 0 {
		return
	}
	// TODO - cleanup gw access when role and groups are updated
	//removedGwAccess
	extclients, err := logic.GetAllExtClients()
	if err != nil {
		slog.Error("failed to fetch extclients", "error", err)
		return
	}
	for _, extclient := range extclients {
		if extclient.OwnerID == currentUser.UserName {
			if _, ok := networkChangeMap[models.NetworkID(extclient.Network)]; ok {
				err = logic.DeleteExtClientAndCleanup(extclient)
				if err != nil {
					slog.Error("failed to delete extclient",
						"id", extclient.ClientID, "owner", changeUser.UserName, "error", err)
				} else {
					if err := mq.PublishDeletedClientPeerUpdate(&extclient); err != nil {
						slog.Error("error setting ext peers: " + err.Error())
					}
				}
			}

		}
	}
	if servercfg.IsDNSMode() {
		logic.SetDNS()
	}

}

func CreateDefaultUserGroupNetworkPolicies(g models.UserGroup) {
	for networkID := range g.NetworkRoles {
		network, err := logic.GetNetwork(networkID.String())
		if err != nil {
			continue
		}

		acl := models.Acl{
			ID:          uuid.New().String(),
			Name:        fmt.Sprintf("%s group", g.Name),
			MetaData:    "This Policy allows user group to communicate with all gateways",
			Default:     true,
			ServiceType: models.Any,
			NetworkID:   models.NetworkID(network.NetID),
			Proto:       models.ALL,
			RuleType:    models.UserPolicy,
			Src: []models.AclPolicyTag{
				{
					ID:    models.UserGroupAclID,
					Value: g.ID.String(),
				},
			},
			Dst: []models.AclPolicyTag{
				{
					ID:    models.NodeTagID,
					Value: fmt.Sprintf("%s.%s", models.NetworkID(network.NetID), models.GwTagName),
				}},
			AllowedDirection: models.TrafficDirectionUni,
			Enabled:          true,
			CreatedBy:        "auto",
			CreatedAt:        time.Now().UTC(),
		}
		logic.InsertAcl(acl)

	}
}

func DeleteDefaultUserGroupNetworkPolicies(g models.UserGroup) {
	for networkID := range g.NetworkRoles {
		acls, err := logic.ListAclsByNetwork(networkID)
		if err != nil {
			continue
		}

		for _, acl := range acls {
			var hasGroupSrc bool
			newAclSrc := make([]models.AclPolicyTag, 0)
			for _, src := range acl.Src {
				if src.ID == models.UserGroupAclID && src.Value == g.ID.String() {
					hasGroupSrc = true
				} else {
					newAclSrc = append(newAclSrc, src)
				}
			}

			if hasGroupSrc {
				if len(newAclSrc) == 0 {
					// no other src exists, delete acl.
					_ = logic.DeleteAcl(acl)
				} else {
					// other sources exist, update acl.
					acl.Src = newAclSrc
					_ = logic.UpsertAcl(acl)
				}
			}
		}
	}
}

func CreateDefaultUserPolicies(netID models.NetworkID) {
	if netID.String() == "" {
		return
	}

	if !logic.IsAclExists(fmt.Sprintf("%s.%s", netID, "all-users")) {
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
		logic.InsertAcl(defaultUserAcl)
	}

	if !logic.IsAclExists(fmt.Sprintf("%s.%s-grp", netID, models.NetworkAdmin)) {
		networkAdminGroupID := GetDefaultNetworkAdminGroupID(netID)

		defaultUserAcl := models.Acl{
			ID:          fmt.Sprintf("%s.%s-grp", netID, models.NetworkAdmin),
			Name:        "Network Admin",
			MetaData:    "This Policy allows all network admins to communicate with all gateways",
			Default:     true,
			ServiceType: models.Any,
			NetworkID:   netID,
			Proto:       models.ALL,
			RuleType:    models.UserPolicy,
			Src: []models.AclPolicyTag{
				{
					ID:    models.UserGroupAclID,
					Value: globalNetworksAdminGroupID.String(),
				},
				{
					ID:    models.UserGroupAclID,
					Value: networkAdminGroupID.String(),
				},
			},
			Dst: []models.AclPolicyTag{
				{
					ID:    models.NodeTagID,
					Value: fmt.Sprintf("%s.%s", netID, models.GwTagName),
				}},
			AllowedDirection: models.TrafficDirectionUni,
			Enabled:          true,
			CreatedBy:        "auto",
			CreatedAt:        time.Now().UTC(),
		}
		logic.InsertAcl(defaultUserAcl)
	}

	if !logic.IsAclExists(fmt.Sprintf("%s.%s-grp", netID, models.NetworkUser)) {
		networkUserGroupID := GetDefaultNetworkUserGroupID(netID)

		defaultUserAcl := models.Acl{
			ID:          fmt.Sprintf("%s.%s-grp", netID, models.NetworkUser),
			Name:        "Network User",
			MetaData:    "This Policy allows all network users to communicate with all gateways",
			Default:     true,
			ServiceType: models.Any,
			NetworkID:   netID,
			Proto:       models.ALL,
			RuleType:    models.UserPolicy,
			Src: []models.AclPolicyTag{
				{
					ID:    models.UserGroupAclID,
					Value: globalNetworksUserGroupID.String(),
				},
				{
					ID:    models.UserGroupAclID,
					Value: networkUserGroupID.String(),
				},
			},

			Dst: []models.AclPolicyTag{
				{
					ID:    models.NodeTagID,
					Value: fmt.Sprintf("%s.%s", netID, models.GwTagName),
				}},
			AllowedDirection: models.TrafficDirectionUni,
			Enabled:          true,
			CreatedBy:        "auto",
			CreatedAt:        time.Now().UTC(),
		}
		logic.InsertAcl(defaultUserAcl)
	}

}

func GetUserGroupsInNetwork(netID models.NetworkID) (networkGrps map[models.UserGroupID]models.UserGroup) {
	groups, _ := ListUserGroups()
	networkGrps = make(map[models.UserGroupID]models.UserGroup)
	for _, grp := range groups {
		if _, ok := grp.NetworkRoles[models.AllNetworks]; ok {
			networkGrps[grp.ID] = grp
			continue
		}
		if _, ok := grp.NetworkRoles[netID]; ok {
			networkGrps[grp.ID] = grp
		}
	}
	return
}

func AddGlobalNetRolesToAdmins(u *models.User) {
	if u.PlatformRoleID != models.SuperAdminRole && u.PlatformRoleID != models.AdminRole {
		return
	}

	if len(u.UserGroups) == 0 {
		u.UserGroups = make(map[models.UserGroupID]struct{})
	}

	u.UserGroups[globalNetworksAdminGroupID] = struct{}{}
}

func GetUserGrpMap() map[models.UserGroupID]map[string]struct{} {
	grpUsersMap := make(map[models.UserGroupID]map[string]struct{})
	users, _ := logic.GetUsersDB()
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
