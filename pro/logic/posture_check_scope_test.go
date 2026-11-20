package logic

import (
	"context"
	"testing"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestPostureCheckAppliesToSubject_allResourcesAndAllUsers(t *testing.T) {
	check := &schema.PostureCheck{
		Status:     true,
		Attribute:  schema.ClientLocation,
		Tags:       datatypes.JSONMap{"*": "*"},
		UserGroups: datatypes.JSONMap{"*": "*"},
		Values:     datatypes.JSONSlice[string]{"AX"},
	}

	infra := models.PostureCheckDeviceInfo{ClientLocation: "US"}
	assert.True(t, postureCheckAppliesToSubject(check, infra), "Tags=* covers infrastructure")

	userHost := models.PostureCheckDeviceInfo{
		ClientLocation: "US",
		Username:       "alice",
		UserGroups:     map[schema.UserGroupID]struct{}{"eng": {}},
	}
	assert.True(t, postureCheckAppliesToSubject(check, userHost), "UserGroups=* covers host-backed user devices")

	extClient := models.PostureCheckDeviceInfo{
		ClientLocation: "US",
		IsUser:         true,
		Username:       "bob",
		UserGroups:     map[schema.UserGroupID]struct{}{"eng": {}},
	}
	assert.True(t, postureCheckAppliesToSubject(check, extClient), "UserGroups=* covers ExtClient users")
}

func TestPostureCheckAppliesToSubject_tagsStarExcludesUserHosts(t *testing.T) {
	tagsOnly := &schema.PostureCheck{
		Status:    true,
		Attribute: schema.ClientLocation,
		Tags:      datatypes.JSONMap{"*": "*"},
		Values:    datatypes.JSONSlice[string]{"AX"},
	}

	infra := models.PostureCheckDeviceInfo{ClientLocation: "US"}
	assert.True(t, postureCheckAppliesToSubject(tagsOnly, infra))

	userHost := models.PostureCheckDeviceInfo{
		ClientLocation: "US",
		Username:       "alice",
	}
	assert.False(t, postureCheckAppliesToSubject(tagsOnly, userHost),
		"All Resources must not enforce posture on host-backed user devices")
}

func TestPostureCheckAppliesToSubject_userGroupsOnly(t *testing.T) {
	usersOnly := &schema.PostureCheck{
		Status:     true,
		Attribute:  schema.ClientLocation,
		UserGroups: datatypes.JSONMap{"*": "*"},
		Values:     datatypes.JSONSlice[string]{"AX"},
	}

	infra := models.PostureCheckDeviceInfo{ClientLocation: "US"}
	assert.False(t, postureCheckAppliesToSubject(usersOnly, infra))

	userHost := models.PostureCheckDeviceInfo{
		ClientLocation: "US",
		Username:       "alice",
	}
	assert.True(t, postureCheckAppliesToSubject(usersOnly, userHost))
}

func TestGetPostureCheckViolations_flagsHostBackedUserViaUserGroups(t *testing.T) {
	orig := logic.GetFeatureFlags
	t.Cleanup(func() { logic.GetFeatureFlags = orig })
	logic.GetFeatureFlags = func(context.Context) models.FeatureFlags {
		return models.FeatureFlags{EnablePostureChecks: true}
	}

	checks := []schema.PostureCheck{{
		ID:         "loc",
		Name:       "loc",
		Status:     true,
		Attribute:  schema.ClientLocation,
		Severity:   schema.SeverityMedium,
		Tags:       datatypes.JSONMap{"*": "*"},
		UserGroups: datatypes.JSONMap{"*": "*"},
		Values:     datatypes.JSONSlice[string]{"AX"},
	}}

	userHost := models.PostureCheckDeviceInfo{
		ClientLocation: "US",
		Username:       "alice",
	}
	violations, sev := GetPostureCheckViolations(context.Background(), checks, userHost)
	require.NotEmpty(t, violations)
	assert.Equal(t, schema.SeverityMedium, sev)

	infraClean := models.PostureCheckDeviceInfo{ClientLocation: "AX"}
	violations, sev = GetPostureCheckViolations(context.Background(), checks, infraClean)
	assert.Empty(t, violations)
	assert.Equal(t, schema.SeverityUnknown, sev)
}
