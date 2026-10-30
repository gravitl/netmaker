package logic

import (
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestDoesUserHaveAccessToEgress_AllUsersWildcard(t *testing.T) {
	user := &schema.User{Username: "alice"}
	e := &schema.Egress{ID: "eg-1", Status: true}
	acls := []models.Acl{{
		Enabled: true,
		Src:     []models.AclPolicyTag{{ID: models.UserAclID, Value: "*"}},
		Dst:     []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
	}}
	if !DoesUserHaveAccessToEgress(user, e, acls) {
		t.Fatal("All Users (user:*) → All Resources must grant egress access")
	}
}

func TestDoesUserHaveAccessToEgress_SpecificUserAndGroup(t *testing.T) {
	e := &schema.Egress{ID: "eg-1", Status: true}
	acls := []models.Acl{{
		Enabled: true,
		Src: []models.AclPolicyTag{
			{ID: models.UserAclID, Value: "alice"},
			{ID: models.UserGroupAclID, Value: "ops"},
		},
		Dst: []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-1"}},
	}}
	if !DoesUserHaveAccessToEgress(&schema.User{Username: "alice"}, e, acls) {
		t.Fatal("named user source must match")
	}
	if DoesUserHaveAccessToEgress(&schema.User{Username: "bob"}, e, acls) {
		t.Fatal("unrelated user must not match without group membership")
	}
	bob := &schema.User{
		Username:   "bob",
		UserGroups: datatypes.NewJSONType(map[schema.UserGroupID]struct{}{"ops": {}}),
	}
	if !DoesUserHaveAccessToEgress(bob, e, acls) {
		t.Fatal("user-group source must match group members")
	}
}

func TestDoesUserHaveAccessToEgress_DisabledOrInactive(t *testing.T) {
	user := &schema.User{Username: "alice"}
	e := &schema.Egress{ID: "eg-1", Status: false}
	acls := []models.Acl{{
		Enabled: true,
		Src:     []models.AclPolicyTag{{ID: models.UserAclID, Value: "*"}},
		Dst:     []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
	}}
	if DoesUserHaveAccessToEgress(user, e, acls) {
		t.Fatal("inactive egress must be denied")
	}
	e.Status = true
	acls[0].Enabled = false
	if DoesUserHaveAccessToEgress(user, e, acls) {
		t.Fatal("disabled policy must be denied")
	}
}
