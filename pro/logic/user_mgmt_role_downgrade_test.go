package logic

import (
	"testing"

	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestStripGroupsOnRoleDowngrade_platformUserKeepsNetworkAdminGroup(t *testing.T) {
	netID := schema.NetworkID("net-a")
	netAdminGrp := GetDefaultNetworkAdminGroupID(netID)
	userGrp := GetDefaultNetworkUserGroupID(netID)

	groups := map[schema.UserGroupID]struct{}{
		globalNetworksAdminGroupID: {},
		netAdminGrp:                {},
		userGrp:                    {},
	}

	StripGroupsOnRoleDowngrade(schema.AdminRole, schema.PlatformUser, groups)

	if _, ok := groups[globalNetworksAdminGroupID]; ok {
		t.Fatal("expected global admin group to be removed for platform-user")
	}
	if _, ok := groups[netAdminGrp]; !ok {
		t.Fatal("expected per-network admin group to remain for platform-user")
	}
	if _, ok := groups[userGrp]; !ok {
		t.Fatal("expected network user group to remain")
	}
}

func TestStripGroupsOnRoleDowngrade_serviceUserRemovesAdminGroups(t *testing.T) {
	netID := schema.NetworkID("net-b")
	netAdminGrp := GetDefaultNetworkAdminGroupID(netID)
	userGrp := GetDefaultNetworkUserGroupID(netID)

	groups := map[schema.UserGroupID]struct{}{
		globalNetworksAdminGroupID: {},
		netAdminGrp:                {},
		userGrp:                    {},
	}

	StripGroupsOnRoleDowngrade(schema.SuperAdminRole, schema.ServiceUser, groups)

	if _, ok := groups[globalNetworksAdminGroupID]; ok {
		t.Fatal("expected global admin group to be removed for service-user")
	}
	if _, ok := groups[netAdminGrp]; ok {
		t.Fatal("expected per-network admin group to be removed for service-user")
	}
	if _, ok := groups[userGrp]; !ok {
		t.Fatal("expected network user group to remain for service-user")
	}
}

func TestStripGroupsOnRoleDowngrade_auditorClearsAllGroups(t *testing.T) {
	groups := map[schema.UserGroupID]struct{}{
		globalNetworksAdminGroupID:                            {},
		GetDefaultNetworkAdminGroupID(schema.NetworkID("net")): {},
		GetDefaultNetworkUserGroupID(schema.NetworkID("net")):  {},
		"custom-grp": {},
	}

	StripGroupsOnRoleDowngrade(schema.PlatformUser, schema.Auditor, groups)

	if len(groups) != 0 {
		t.Fatalf("expected all groups removed for auditor, got %d", len(groups))
	}
}

func TestStripGroupsOnRoleDowngrade_noOpWithoutDowngrade(t *testing.T) {
	groups := map[schema.UserGroupID]struct{}{
		globalNetworksAdminGroupID: {},
	}

	StripGroupsOnRoleDowngrade(schema.AdminRole, schema.AdminRole, groups)
	if len(groups) != 1 {
		t.Fatalf("expected no change when role unchanged, got %d groups", len(groups))
	}

	StripGroupsOnRoleDowngrade(schema.PlatformUser, schema.ServiceUser, groups)
	if len(groups) != 1 {
		t.Fatalf("expected no change when not downgrading from admin, got %d groups", len(groups))
	}
}

func TestAddGlobalGroupOnRoleUpgrade(t *testing.T) {
	groups := map[schema.UserGroupID]struct{}{
		"custom-grp": {},
	}
	AddGlobalGroupOnRoleUpgrade(schema.PlatformUser, schema.AdminRole, groups)
	if _, ok := groups[globalNetworksAdminGroupID]; !ok {
		t.Fatal("expected global admin group on upgrade to admin")
	}

	groups2 := map[schema.UserGroupID]struct{}{"custom-grp": {}}
	AddGlobalGroupOnRoleUpgrade(schema.AdminRole, schema.SuperAdminRole, groups2)
	if _, ok := groups2[globalNetworksAdminGroupID]; ok {
		t.Fatal("expected no change when already elevated")
	}

	groups3 := map[schema.UserGroupID]struct{}{}
	AddGlobalGroupOnRoleUpgrade(schema.PlatformUser, schema.PlatformUser, groups3)
	if _, ok := groups3[globalNetworksAdminGroupID]; ok {
		t.Fatal("expected no global admin group without role upgrade")
	}
}

func TestAddGlobalNetRolesToAdmins_onlyWhenEmpty(t *testing.T) {
	u := &schema.User{PlatformRoleID: schema.AdminRole}
	u.UserGroups = datatypes.NewJSONType(map[schema.UserGroupID]struct{}{
		"custom-grp": {},
	})
	AddGlobalNetRolesToAdmins(u)
	if _, ok := u.UserGroups.Data()[globalNetworksAdminGroupID]; ok {
		t.Fatal("expected global admin group not to be added when user already has groups")
	}

	u2 := &schema.User{PlatformRoleID: schema.SuperAdminRole}
	AddGlobalNetRolesToAdmins(u2)
	if _, ok := u2.UserGroups.Data()[globalNetworksAdminGroupID]; !ok {
		t.Fatal("expected global admin group when elevated user has no groups")
	}
}

func TestIsNetworkAdmin_requiresGroupForElevatedPlatformRole(t *testing.T) {
	adminNoGroups := &schema.User{PlatformRoleID: schema.AdminRole, UserGroups: datatypes.NewJSONType(map[schema.UserGroupID]struct{}{})}
	if IsNetworkAdmin(adminNoGroups, "net-a") {
		t.Fatal("admin without groups should not be network admin")
	}

	adminWithGlobal := &schema.User{
		PlatformRoleID: schema.SuperAdminRole,
		UserGroups:     datatypes.NewJSONType(map[schema.UserGroupID]struct{}{globalNetworksAdminGroupID: {}}),
	}
	if !IsNetworkAdmin(adminWithGlobal, "net-a") {
		t.Fatal("expected network admin via global admin group")
	}
}

func TestUserGroupGrantsAdminAccess_customGroup(t *testing.T) {
	netID := schema.NetworkID("net-c")
	adminRole := GetDefaultNetworkAdminRoleID(netID)
	g := &schema.UserGroup{
		ID: "custom-admin-grp",
		NetworkRoles: datatypes.NewJSONType(schema.NetworkRoles{
			netID: {adminRole: {}},
		}),
	}
	if !userGroupGrantsAdminAccess(g) {
		t.Fatal("expected custom group with network-admin role to grant admin access")
	}
}
