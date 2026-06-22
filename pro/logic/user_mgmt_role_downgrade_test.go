package logic

import (
	"testing"

	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestRoleChangeDoesNotModifyGroups(t *testing.T) {
	netID := schema.NetworkID("net-a")
	netAdminGrp := GetDefaultNetworkAdminGroupID(netID)
	userGrp := GetDefaultNetworkUserGroupID(netID)

	groups := map[schema.UserGroupID]struct{}{
		globalNetworksAdminGroupID: {},
		netAdminGrp:                {},
		userGrp:                    {},
		"custom-grp":               {},
	}

	StripGroupsOnRoleDowngrade(schema.AdminRole, schema.PlatformUser, groups)
	if len(groups) != 4 {
		t.Fatalf("expected groups unchanged on downgrade, got %d", len(groups))
	}

	StripGroupsOnRoleDowngrade(schema.SuperAdminRole, schema.ServiceUser, groups)
	if len(groups) != 4 {
		t.Fatalf("expected groups unchanged on service-user downgrade, got %d", len(groups))
	}

	StripGroupsOnRoleDowngrade(schema.PlatformUser, schema.Auditor, groups)
	if len(groups) != 4 {
		t.Fatalf("expected groups unchanged on auditor downgrade, got %d", len(groups))
	}

	groups2 := map[schema.UserGroupID]struct{}{}
	AddGlobalGroupOnRoleUpgrade(schema.PlatformUser, schema.AdminRole, groups2)
	if _, ok := groups2[globalNetworksAdminGroupID]; !ok {
		t.Fatal("expected global admin group on upgrade when user has no groups")
	}

	groups3 := map[schema.UserGroupID]struct{}{"custom-grp": {}}
	AddGlobalGroupOnRoleUpgrade(schema.PlatformUser, schema.AdminRole, groups3)
	if _, ok := groups3[globalNetworksAdminGroupID]; ok {
		t.Fatal("expected no global admin group when user already has groups")
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
