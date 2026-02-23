package logic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
	"gorm.io/gorm"

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

var ServiceUserPermissionTemplate = schema.UserRole{
	ID:                  models.ServiceUser,
	Default:             true,
	FullAccess:          false,
	DenyDashboardAccess: true,
}

var PlatformUserUserPermissionTemplate = schema.UserRole{
	ID:         models.PlatformUser,
	Default:    true,
	FullAccess: false,
}

var AuditorUserPermissionTemplate = schema.UserRole{
	ID:                  models.Auditor,
	Default:             true,
	DenyDashboardAccess: false,
	FullAccess:          false,
	NetworkLevelAccess: datatypes.NewJSONType(schema.ResourceAccess{
		models.NetworkRsrc: {
			models.AllNetworkRsrcID: models.RsrcPermissionScope{
				Read: true,
			},
		},
	}),
}

var NetworkAdminAllPermissionTemplate = schema.UserRole{
	ID:         globalNetworksAdminRoleID,
	Name:       "Network Admins",
	MetaData:   "can manage configuration of all networks",
	Default:    true,
	FullAccess: true,
	NetworkID:  models.AllNetworks,
}

var NetworkUserAllPermissionTemplate = schema.UserRole{
	ID:         globalNetworksUserRoleID,
	Name:       "Network Users",
	MetaData:   "Can connect to nodes in your networks via Netmaker Desktop App.",
	Default:    true,
	FullAccess: false,
	NetworkID:  models.AllNetworks,
	NetworkLevelAccess: datatypes.NewJSONType(schema.ResourceAccess{
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
		models.PostureCheckRsrc: {
			models.AllPostureCheckRsrcID: models.RsrcPermissionScope{
				Read: true,
			},
		},
		models.NameserverRsrc: {
			models.AllNameserverRsrcID: models.RsrcPermissionScope{
				Read: true,
			},
		},
		models.JitUserRsrc: {
			models.AllJitUserRsrcID: models.RsrcPermissionScope{
				Read:     true,
				Create:   true,
				Update:   true,
				Delete:   true,
				SelfOnly: true,
			},
		},
	}),
}

func UserRolesInit() {
	_ = logic.SuperAdminPermissionTemplate.Upsert(db.WithContext(context.TODO()))
	_ = logic.AdminPermissionTemplate.Upsert(db.WithContext(context.TODO()))
	_ = ServiceUserPermissionTemplate.Upsert(db.WithContext(context.TODO()))
	_ = PlatformUserUserPermissionTemplate.Upsert(db.WithContext(context.TODO()))
	_ = AuditorUserPermissionTemplate.Upsert(db.WithContext(context.TODO()))
	_ = NetworkAdminAllPermissionTemplate.Upsert(db.WithContext(context.TODO()))
	_ = NetworkUserAllPermissionTemplate.Upsert(db.WithContext(context.TODO()))
}

func UserGroupsInit() {
	// create default network groups
	var NetworkGlobalAdminGroup = schema.UserGroup{
		ID:       globalNetworksAdminGroupID,
		Default:  true,
		Name:     "All Networks Admin Group",
		MetaData: "can manage configuration of all networks",
		NetworkRoles: datatypes.NewJSONType(schema.NetworkRoles{
			models.AllNetworks: {
				globalNetworksAdminRoleID: {},
			},
		}),
	}
	var NetworkGlobalUserGroup = schema.UserGroup{
		ID:      globalNetworksUserGroupID,
		Name:    "All Networks User Group",
		Default: true,
		NetworkRoles: datatypes.NewJSONType(schema.NetworkRoles{
			models.AllNetworks: {
				globalNetworksUserRoleID: {},
			},
		}),
		MetaData: "Provides read-only dashboard access to platform users and allows connection to network nodes via the Netmaker Desktop App.",
	}

	_ = NetworkGlobalAdminGroup.Upsert(db.WithContext(context.TODO()))
	_ = NetworkGlobalUserGroup.Upsert(db.WithContext(context.TODO()))
}

func CreateDefaultNetworkRolesAndGroups(netID models.NetworkID) {
	if netID.String() == "" {
		return
	}
	var NetworkAdminPermissionTemplate = schema.UserRole{
		ID:                 GetDefaultNetworkAdminRoleID(netID),
		Name:               fmt.Sprintf("%s Admin", netID),
		MetaData:           fmt.Sprintf("can manage your network `%s` configuration.", netID),
		Default:            true,
		NetworkID:          netID,
		FullAccess:         true,
		NetworkLevelAccess: datatypes.NewJSONType(schema.ResourceAccess{}),
	}

	var NetworkUserPermissionTemplate = schema.UserRole{
		ID:                  GetDefaultNetworkUserRoleID(netID),
		Name:                fmt.Sprintf("%s User", netID),
		MetaData:            fmt.Sprintf("Can connect to nodes in your network `%s` via Netmaker Desktop App.", netID),
		Default:             true,
		FullAccess:          false,
		NetworkID:           netID,
		DenyDashboardAccess: false,
		NetworkLevelAccess: datatypes.NewJSONType(schema.ResourceAccess{
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
			models.PostureCheckRsrc: {
				models.AllPostureCheckRsrcID: models.RsrcPermissionScope{
					Read: true,
				},
			},
			models.NameserverRsrc: {
				models.AllNameserverRsrcID: models.RsrcPermissionScope{
					Read: true,
				},
			},
			models.JitUserRsrc: {
				models.AllJitUserRsrcID: models.RsrcPermissionScope{
					Read:     true,
					Create:   true,
					Update:   true,
					Delete:   true,
					SelfOnly: true,
				},
			},
		}),
	}

	_ = NetworkAdminPermissionTemplate.Upsert(db.WithContext(context.TODO()))
	_ = NetworkUserPermissionTemplate.Upsert(db.WithContext(context.TODO()))

	// create default network groups
	var NetworkAdminGroup = schema.UserGroup{
		ID:      GetDefaultNetworkAdminGroupID(netID),
		Name:    fmt.Sprintf("%s Admin Group", netID),
		Default: true,
		NetworkRoles: datatypes.NewJSONType(schema.NetworkRoles{
			netID: {
				GetDefaultNetworkAdminRoleID(netID): {},
			},
		}),
		MetaData: fmt.Sprintf("can manage your network `%s` configuration including adding and removing devices.", netID),
	}
	var NetworkUserGroup = schema.UserGroup{
		ID:      GetDefaultNetworkUserGroupID(netID),
		Name:    fmt.Sprintf("%s User Group", netID),
		Default: true,
		NetworkRoles: datatypes.NewJSONType(schema.NetworkRoles{
			netID: {
				GetDefaultNetworkUserRoleID(netID): {},
			},
		}),
		MetaData: fmt.Sprintf("Can connect to nodes in your network `%s` via Netmaker Desktop App. Platform users will have read-only access to the the dashboard.", netID),
	}
	_ = NetworkAdminGroup.Upsert(db.WithContext(context.TODO()))
	_ = NetworkUserGroup.Upsert(db.WithContext(context.TODO()))
}

func DeleteNetworkRoles(netID string) {
	users, err := (&schema.User{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return
	}

	defaultAdminGrpID := GetDefaultNetworkAdminGroupID(models.NetworkID(netID))
	defaultUserGrpID := GetDefaultNetworkUserGroupID(models.NetworkID(netID))
	for _, user := range users {
		var upsert bool
		if _, ok := user.UserGroups.Data()[defaultUserGrpID]; ok {
			delete(user.UserGroups.Data(), defaultUserGrpID)
			upsert = true
		}
		if _, ok := user.UserGroups.Data()[defaultAdminGrpID]; ok {
			delete(user.UserGroups.Data(), defaultAdminGrpID)
			upsert = true
		}
		if upsert {
			logic.UpsertUser(user)
		}
	}
	_ = (&schema.UserGroup{
		ID: defaultUserGrpID,
	}).Delete(db.WithContext(context.TODO()))

	_ = (&schema.UserGroup{
		ID: defaultAdminGrpID,
	}).Delete(db.WithContext(context.TODO()))

	userGs, _ := (&schema.UserGroup{}).ListAll(db.WithContext(context.TODO()))
	for _, userGI := range userGs {
		if _, ok := userGI.NetworkRoles.Data()[models.NetworkID(netID)]; ok {
			delete(userGI.NetworkRoles.Data(), models.NetworkID(netID))
			UpdateUserGroup(userGI)
		}
	}

	networkRoles := &schema.UserRole{
		NetworkID: models.NetworkID(netID),
	}
	_ = networkRoles.DeleteNetworkRoles(db.WithContext(context.TODO()))
}

func ValidateCreateRoleReq(userRole *schema.UserRole) error {
	// check if role exists with this id
	roleCheck := &schema.UserRole{ID: userRole.ID}
	err := roleCheck.Get(db.WithContext(context.TODO()))
	if err == nil {
		return fmt.Errorf("role with id `%s` exists already", userRole.ID.String())
	}
	if len(userRole.NetworkLevelAccess.Data()) > 0 {
		for rsrcType := range userRole.NetworkLevelAccess.Data() {
			if _, ok := models.RsrcTypeMap[rsrcType]; !ok {
				return errors.New("invalid rsrc type " + rsrcType.String())
			}
			if rsrcType == models.RemoteAccessGwRsrc {
				userRsrcPermissions := userRole.NetworkLevelAccess.Data()[models.RemoteAccessGwRsrc]
				var vpnAccess bool
				for _, scope := range userRsrcPermissions {
					if scope.VPNaccess {
						vpnAccess = true
						break
					}
				}
				if vpnAccess {
					userRole.NetworkLevelAccess.Data()[models.ExtClientsRsrc] = map[models.RsrcID]models.RsrcPermissionScope{
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

func ValidateUpdateRoleReq(userRole *schema.UserRole) error {
	roleInDB := &schema.UserRole{ID: userRole.ID}
	err := roleInDB.Get(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}
	if roleInDB.NetworkID != userRole.NetworkID {
		return errors.New("network id mismatch")
	}
	if roleInDB.Default {
		return errors.New("cannot update default role")
	}
	if len(userRole.NetworkLevelAccess.Data()) > 0 {
		for rsrcType := range userRole.NetworkLevelAccess.Data() {
			if _, ok := models.RsrcTypeMap[rsrcType]; !ok {
				return errors.New("invalid rsrc type " + rsrcType.String())
			}
			if rsrcType == models.RemoteAccessGwRsrc {
				userRsrcPermissions := userRole.NetworkLevelAccess.Data()[models.RemoteAccessGwRsrc]
				var vpnAccess bool
				for _, scope := range userRsrcPermissions {
					if scope.VPNaccess {
						vpnAccess = true
						break
					}
				}
				if vpnAccess {
					userRole.NetworkLevelAccess.Data()[models.ExtClientsRsrc] = map[models.RsrcID]models.RsrcPermissionScope{
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
func CreateRole(role *schema.UserRole) error {
	// default roles are currently created directly in the db.
	// this check is only to prevent future errors.
	if role.Default && role.ID == "" {
		return errors.New("role id cannot be empty for default role")
	}

	if !role.Default {
		role.ID = models.UserRoleID(uuid.NewString())
	}

	// check if the role already exists
	if role.Name == "" {
		return errors.New("role name cannot be empty")
	}

	exists, err := role.Exists(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	if exists {
		return errors.New("role already exists")
	}

	return role.Create(db.WithContext(context.TODO()))
}

// DeleteRole - deletes user role
func DeleteRole(rid models.UserRoleID, force bool) error {
	if rid.String() == "" {
		return errors.New("role id cannot be empty")
	}
	users, err := (&schema.User{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}
	role := &schema.UserRole{ID: rid}
	err = role.Get(db.WithContext(context.TODO()))
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
		for userG := range user.UserGroups.Data() {
			ug, err := GetUserGroup(userG)
			if err == nil {
				if role.NetworkID != "" {
					for netID, networkRoles := range ug.NetworkRoles.Data() {
						if _, ok := networkRoles[rid]; ok {
							delete(networkRoles, rid)
							ug.NetworkRoles.Data()[netID] = networkRoles
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
	}
	return (&schema.UserRole{
		ID: rid,
	}).Delete(db.WithContext(context.TODO()))
}

func ValidateCreateGroupReq(g schema.UserGroup) error {

	// check if network roles are valid
	for _, roleMap := range g.NetworkRoles.Data() {
		for roleID := range roleMap {
			role := &schema.UserRole{ID: roleID}
			err := role.Get(db.WithContext(context.TODO()))
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
func ValidateUpdateGroupReq(new schema.UserGroup) error {
	var newHasAllNetworkRole, newHasSpecNetworkRole bool
	for networkID := range new.NetworkRoles.Data() {
		if networkID == models.AllNetworks {
			newHasAllNetworkRole = true
		} else {
			newHasSpecNetworkRole = true
		}

		userRolesMap := new.NetworkRoles.Data()[networkID]
		for roleID := range userRolesMap {
			netRole := &schema.UserRole{ID: roleID}
			err := netRole.Get(db.WithContext(context.TODO()))
			if err != nil {
				err = fmt.Errorf("invalid network role")
				return err
			}
			if netRole.NetworkID == "" {
				return errors.New("platform role cannot be used as network role")
			}
		}
	}

	if newHasAllNetworkRole && newHasSpecNetworkRole {
		return errors.New("cannot have networks roles for all networks and a specific network")
	}

	return nil
}

// CreateUserGroup - creates new user group
func CreateUserGroup(g *schema.UserGroup) error {
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

	err := (&schema.UserGroup{
		Name: g.Name,
	}).GetByName(db.WithContext(context.TODO()))
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else {
		return errors.New("group already exists")
	}

	err = g.Create(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}
	// create default network gateway policies
	CreateDefaultUserGroupNetworkPolicies(*g)
	return nil
}

// GetUserGroup - fetches user group
func GetUserGroup(gid models.UserGroupID) (schema.UserGroup, error) {
	group := schema.UserGroup{
		ID: gid,
	}
	err := group.Get(db.WithContext(context.TODO()))
	return group, err
}

func GetDefaultGlobalAdminGroupID() models.UserGroupID {
	return globalNetworksAdminGroupID
}

func GetDefaultGlobalUserGroupID() models.UserGroupID {
	return globalNetworksUserGroupID
}

func GetDefaultGlobalAdminRoleID() models.UserRoleID {
	return globalNetworksAdminRoleID
}

func GetDefaultGlobalUserRoleID() models.UserRoleID {
	return globalNetworksUserRoleID
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

// UpdateUserGroup - updates new user group
func UpdateUserGroup(g schema.UserGroup) error {
	// check if the group exists
	if g.ID == "" {
		return errors.New("group id cannot be empty")
	}
	return g.Update(db.WithContext(context.TODO()))
}

func DeleteAndCleanUpGroup(group *schema.UserGroup) error {
	err := DeleteUserGroup(group.ID)
	if err != nil {
		return err
	}
	go func() {
		var replacePeers bool
		var networkIDs []models.NetworkID

		_, ok := group.NetworkRoles.Data()[models.AllNetworks]
		if ok {
			networks, _ := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
			for _, network := range networks {
				networkIDs = append(networkIDs, models.NetworkID(network.Name))
			}
		} else {
			for networkID := range group.NetworkRoles.Data() {
				networkIDs = append(networkIDs, networkID)
			}
		}

		for _, networkID := range networkIDs {
			go RemoveUserGroupFromPostureChecks(group.ID, networkID)
			acls, err := logic.ListAclsByNetwork(networkID)
			if err != nil {
				continue
			}

			for _, acl := range acls {
				var hasGroupSrc bool
				newAclSrc := make([]models.AclPolicyTag, 0)
				for _, src := range acl.Src {
					if src.ID == models.UserGroupAclID && src.Value == group.ID.String() {
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
					replacePeers = true
				}
			}
		}

		go UpdatesUserGwAccessOnGrpUpdates(group.ID, group.NetworkRoles.Data(), make(map[models.NetworkID]map[models.UserRoleID]struct{}))
		go mq.PublishPeerUpdate(replacePeers)
	}()

	return nil
}

// DeleteUserGroup - deletes user group
func DeleteUserGroup(gid models.UserGroupID) error {
	g, err := GetUserGroup(gid)
	if err != nil {
		return err
	}
	users, err := (&schema.User{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}
	for _, user := range users {
		delete(user.UserGroups.Data(), gid)
		logic.UpsertUser(user)
	}
	// create default network gateway policies
	DeleteDefaultUserGroupNetworkPolicies(g)
	return (&schema.UserGroup{
		ID: gid,
	}).Delete(db.WithContext(context.TODO()))
}

func GetUserRAGNodes(user *schema.User) (gws map[string]models.Node) {
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
			if ok, _ := IsUserAllowedToCommunicate(user.Username, node); ok {
				gws[node.ID.String()] = node
				continue
			}
		} else {
			for groupID := range user.UserGroups.Data() {
				userGrp, err := logic.GetUserGroup(groupID)
				if err == nil {
					if roles, ok := userGrp.NetworkRoles.Data()[models.NetworkID(node.Network)]; ok && len(roles) > 0 {
						if ok, _ := IsUserAllowedToCommunicate(user.Username, node); ok {
							gws[node.ID.String()] = node
							break
						}
					}
					if roles, ok := userGrp.NetworkRoles.Data()[models.AllNetworks]; ok && len(roles) > 0 {
						if ok, _ := IsUserAllowedToCommunicate(user.Username, node); ok {
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

func GetFilteredNodesByUserAccess(user *schema.User, nodes []models.Node) (filteredNodes []models.Node) {
	return nodes
}

func FilterNetworksByRole(allnetworks []schema.Network, user *schema.User) []schema.Network {
	platformRole := &schema.UserRole{ID: user.PlatformRoleID}
	err := platformRole.Get(db.WithContext(context.TODO()))
	if err != nil {
		return []schema.Network{}
	}
	if !platformRole.FullAccess {
		allNetworkRoles := make(map[models.NetworkID]struct{})
		_, ok := platformRole.NetworkLevelAccess.Data()[models.NetworkRsrc]
		if ok {
			perm, ok := platformRole.NetworkLevelAccess.Data()[models.NetworkRsrc][models.AllNetworkRsrcID]
			if ok && perm.Read {
				return allnetworks
			}
		}
		for userGID := range user.UserGroups.Data() {
			userG, err := GetUserGroup(userGID)
			if err == nil {
				if len(userG.NetworkRoles.Data()) > 0 {
					for netID := range userG.NetworkRoles.Data() {
						if netID == models.AllNetworks {
							return allnetworks
						}
						allNetworkRoles[netID] = struct{}{}
					}
				}
			}
		}
		var filteredNetworks []schema.Network
		for _, networkI := range allnetworks {
			if _, ok := allNetworkRoles[models.NetworkID(networkI.Name)]; ok {
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
			err := (&schema.Network{Name: netID.String()}).Get(db.WithContext(context.TODO()))
			if err != nil {
				return fmt.Errorf("failed to fetch network %s ", netID)
			}
		}
		for netRoleID := range netRoles {
			role := &schema.UserRole{ID: netRoleID}
			err := role.Get(db.WithContext(context.TODO()))
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
func PrepareOauthUserFromInvite(in models.UserInvite) (schema.User, error) {
	var newPass, fetchErr = logic.FetchPassValue("")
	if fetchErr != nil {
		return schema.User{}, fetchErr
	}
	user := schema.User{
		Username: in.Email,
		Password: newPass,
	}
	user.UserGroups = datatypes.NewJSONType(in.UserGroups)
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
						"id", extclient.ClientID, "owner", user.Username, "error", err)
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
						"id", extclient.ClientID, "owner", user.Username, "error", err)
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
				_, userInGroup := user.UserGroups.Data()[groupID]
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

		if shouldDelete {
			err = logic.DeleteExtClientAndCleanup(extclient)
			if err != nil {
				slog.Error("failed to delete extclient",
					"id", extclient.ClientID, "owner", user.Username, "error", err)
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

func UpdateUserGwAccess(currentUser, changeUser *schema.User) {
	if changeUser.PlatformRoleID != models.ServiceUser {
		return
	}

	networkChangeMap := make(map[models.NetworkID]map[models.UserRoleID]struct{})
	for gID := range currentUser.UserGroups.Data() {
		if _, ok := changeUser.UserGroups.Data()[gID]; ok {
			continue
		}
		userG, err := GetUserGroup(gID)
		if err == nil {
			for netID, networkUserRoles := range userG.NetworkRoles.Data() {
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
		if extclient.OwnerID == currentUser.Username {
			if _, ok := networkChangeMap[models.NetworkID(extclient.Network)]; ok {
				err = logic.DeleteExtClientAndCleanup(extclient)
				if err != nil {
					slog.Error("failed to delete extclient",
						"id", extclient.ClientID, "owner", changeUser.Username, "error", err)
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

func CreateDefaultUserGroupNetworkPolicies(g schema.UserGroup) {
	for networkID := range g.NetworkRoles.Data() {
		network := &schema.Network{Name: networkID.String()}
		err := network.Get(db.WithContext(context.TODO()))
		if err != nil {
			continue
		}

		acl := models.Acl{
			ID:          uuid.New().String(),
			Name:        fmt.Sprintf("%s group", g.Name),
			MetaData:    "This Policy allows user group to communicate with all gateways",
			Default:     true,
			ServiceType: models.Any,
			NetworkID:   models.NetworkID(network.Name),
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
					Value: fmt.Sprintf("%s.%s", models.NetworkID(network.Name), models.GwTagName),
				}},
			AllowedDirection: models.TrafficDirectionUni,
			Enabled:          true,
			CreatedBy:        "auto",
			CreatedAt:        time.Now().UTC(),
		}
		logic.InsertAcl(acl)

	}
}

func DeleteDefaultUserGroupNetworkPolicies(g schema.UserGroup) {
	for networkID := range g.NetworkRoles.Data() {
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

func GetUserGroupsInNetwork(netID models.NetworkID) (networkGrps map[models.UserGroupID]schema.UserGroup) {
	groups, _ := (&schema.UserGroup{}).ListAll(db.WithContext(context.TODO()))
	networkGrps = make(map[models.UserGroupID]schema.UserGroup)
	for _, grp := range groups {
		if _, ok := grp.NetworkRoles.Data()[models.AllNetworks]; ok {
			networkGrps[grp.ID] = grp
			continue
		}
		if _, ok := grp.NetworkRoles.Data()[netID]; ok {
			networkGrps[grp.ID] = grp
		}
	}
	return
}

func AddGlobalNetRolesToAdmins(u *schema.User) {
	if u.PlatformRoleID != models.SuperAdminRole && u.PlatformRoleID != models.AdminRole {
		return
	}

	if len(u.UserGroups.Data()) == 0 {
		u.UserGroups = datatypes.NewJSONType(make(map[models.UserGroupID]struct{}))
	}

	u.UserGroups.Data()[globalNetworksAdminGroupID] = struct{}{}
}

func GetUserGrpMap() map[models.UserGroupID]map[string]struct{} {
	grpUsersMap := make(map[models.UserGroupID]map[string]struct{})
	users, _ := (&schema.User{}).ListAll(db.WithContext(context.TODO()))
	for _, user := range users {
		for gID := range user.UserGroups.Data() {
			if grpUsers, ok := grpUsersMap[gID]; ok {
				grpUsers[user.Username] = struct{}{}
				grpUsersMap[gID] = grpUsers
			} else {
				grpUsersMap[gID] = make(map[string]struct{})
				grpUsersMap[gID][user.Username] = struct{}{}
			}
		}
	}

	return grpUsersMap
}
