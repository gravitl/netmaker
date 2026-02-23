package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// Pre-Define Permission Templates for default Roles
var SuperAdminPermissionTemplate = schema.UserRole{
	ID:         models.SuperAdminRole,
	Default:    true,
	FullAccess: true,
}

var AdminPermissionTemplate = schema.UserRole{
	ID:         models.AdminRole,
	Default:    true,
	FullAccess: true,
}

var GetFilteredNodesByUserAccess = func(user *schema.User, nodes []models.Node) (filteredNodes []models.Node) {
	return
}

var DeleteRole = func(r models.UserRoleID, force bool) error {
	return nil
}

var FilterNetworksByRole = func(allnetworks []schema.Network, user *schema.User) []schema.Network {
	return allnetworks
}

var IsGroupsValid = func(groups map[models.UserGroupID]struct{}) error {
	return nil
}

var UpdateUserGwAccess = func(currentUser, changeUser *schema.User) {}

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
var ListUserGroups = func() ([]schema.UserGroup, error) { return nil, nil }
var GetUserGroup = func(groupId models.UserGroupID) (userGrps schema.UserGroup, err error) { return }
var AddGlobalNetRolesToAdmins = func(u *schema.User) {}
var EmailInit = func() {}

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
	case models.PostureCheckRsrc:
		return models.AllPostureCheckRsrcID
	case models.NameserverRsrc:
		return models.AllNameserverRsrcID
	case models.JitAdminRsrc:
		return models.AllJitAdminRsrcID
	case models.JitUserRsrc:
		return models.AllJitUserRsrcID
	}
	return ""
}

func userRolesInit() {
	_ = SuperAdminPermissionTemplate.Upsert(db.WithContext(context.TODO()))
	_ = AdminPermissionTemplate.Upsert(db.WithContext(context.TODO()))
}
