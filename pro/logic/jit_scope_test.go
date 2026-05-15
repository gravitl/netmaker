package logic

import (
	"testing"

	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestUserMustSatisfyJIT(t *testing.T) {
	g1 := schema.UserGroupID("group-1")
	g2 := schema.UserGroupID("group-2")

	netUnscoped := &schema.Network{}
	netScoped := &schema.Network{JITUserGroupIDs: []schema.UserGroupID{g1}}

	userInG1 := &schema.User{UserGroups: datatypes.NewJSONType(map[schema.UserGroupID]struct{}{g1: {}})}
	userInG2 := &schema.User{UserGroups: datatypes.NewJSONType(map[schema.UserGroupID]struct{}{g2: {}})}
	userNoGroups := &schema.User{UserGroups: datatypes.NewJSONType(map[schema.UserGroupID]struct{}{})}

	t.Run("unscoped_requires_all", func(t *testing.T) {
		if !userMustSatisfyJIT(netUnscoped, nil) {
			t.Fatal("expected subject when jit_user_group_ids empty and user unknown")
		}
		if !userMustSatisfyJIT(netUnscoped, userInG2) {
			t.Fatal("expected subject when jit_user_group_ids empty")
		}
	})

	t.Run("scoped_nil_user_not_subject", func(t *testing.T) {
		if userMustSatisfyJIT(netScoped, nil) {
			t.Fatal("unknown user should not be subject when allowlist is set")
		}
	})

	t.Run("scoped_membership", func(t *testing.T) {
		if !userMustSatisfyJIT(netScoped, userInG1) {
			t.Fatal("user in allowlisted group should be subject")
		}
		if userMustSatisfyJIT(netScoped, userInG2) {
			t.Fatal("user not in allowlisted group should not be subject")
		}
		if userMustSatisfyJIT(netScoped, userNoGroups) {
			t.Fatal("user with no groups should not be subject when allowlist is set")
		}
	})

	t.Run("empty_slice_same_as_unscoped", func(t *testing.T) {
		net := &schema.Network{JITUserGroupIDs: []schema.UserGroupID{}}
		if !userMustSatisfyJIT(net, nil) {
			t.Fatal("empty allowlist should mean full JIT scope")
		}
	})
}
