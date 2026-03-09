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
	ID:         schema.SuperAdminRole,
	Default:    true,
	FullAccess: true,
}

var AdminPermissionTemplate = schema.UserRole{
	ID:         schema.AdminRole,
	Default:    true,
	FullAccess: true,
}

var GetFilteredNodesByUserAccess = func(user *schema.User, nodes []models.Node) (filteredNodes []models.Node) {
	return
}

var DeleteRole = func(r schema.UserRoleID, force bool) error {
	return nil
}

var FilterNetworksByRole = func(allnetworks []schema.Network, user *schema.User) []schema.Network {
	return allnetworks
}

var IsGroupsValid = func(groups map[schema.UserGroupID]struct{}) error {
	return nil
}

var UpdateUserGwAccess = func(currentUser, changeUser *schema.User) {}

var InitialiseRoles = userRolesInit
var IntialiseGroups = func() {}
var DeleteNetworkRoles = func(netID string) {}
var CreateDefaultNetworkRolesAndGroups = func(netID schema.NetworkID) {}
var CreateDefaultUserPolicies = func(netID schema.NetworkID) {
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
var GetUserGroup = func(groupId schema.UserGroupID) (userGrps schema.UserGroup, err error) { return }
var AddGlobalNetRolesToAdmins = func(u *schema.User) {}
var EmailInit = func() {}

func GetAllRsrcIDForRsrc(rsrc schema.RsrcType) schema.RsrcID {
	switch rsrc {
	case schema.HostRsrc:
		return schema.AllHostRsrcID
	case schema.RelayRsrc:
		return schema.AllRelayRsrcID
	case schema.RemoteAccessGwRsrc:
		return schema.AllRemoteAccessGwRsrcID
	case schema.ExtClientsRsrc:
		return schema.AllExtClientsRsrcID
	case schema.InetGwRsrc:
		return schema.AllInetGwRsrcID
	case schema.EgressGwRsrc:
		return schema.AllEgressGwRsrcID
	case schema.NetworkRsrc:
		return schema.AllNetworkRsrcID
	case schema.EnrollmentKeysRsrc:
		return schema.AllEnrollmentKeysRsrcID
	case schema.UserRsrc:
		return schema.AllUserRsrcID
	case schema.DnsRsrc:
		return schema.AllDnsRsrcID
	case schema.FailOverRsrc:
		return schema.AllFailOverRsrcID
	case schema.AclRsrc:
		return schema.AllAclsRsrcID
	case schema.TagRsrc:
		return schema.AllTagsRsrcID
	case schema.PostureCheckRsrc:
		return schema.AllPostureCheckRsrcID
	case schema.NameserverRsrc:
		return schema.AllNameserverRsrcID
	case schema.JitAdminRsrc:
		return schema.AllJitAdminRsrcID
	case schema.JitUserRsrc:
		return schema.AllJitUserRsrcID
	case schema.UserActivityRsrc:
		return schema.AllUserActivityRsrcID
	case schema.TrafficFlow:
		return schema.AllTrafficFlowRsrcID
	}
	return ""
}

func userRolesInit() {
	_ = SuperAdminPermissionTemplate.Upsert(db.WithContext(context.TODO()))
	_ = AdminPermissionTemplate.Upsert(db.WithContext(context.TODO()))
}
