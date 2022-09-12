package pro

import (
	"testing"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models/promodels"
	"github.com/stretchr/testify/assert"
)

func TestUserGroupLogic(t *testing.T) {
	database.InitializeDatabase()

	t.Run("User Groups initialized successfully", func(t *testing.T) {
		err := InitializeGroups()
		assert.Nil(t, err)
	})

	t.Run("Check for default group", func(t *testing.T) {
		groups, err := GetUserGroups()
		assert.Nil(t, err)
		var hasdefault bool
		for k := range groups {
			if string(k) == DEFAULT_ALLOWED_GROUPS {
				hasdefault = true
			}
		}
		assert.True(t, hasdefault)
	})

	t.Run("User Groups created successfully", func(t *testing.T) {
		err := InsertUserGroup(promodels.UserGroupName("group1"))
		assert.Nil(t, err)
		err = InsertUserGroup(promodels.UserGroupName("group2"))
		assert.Nil(t, err)
	})

	t.Run("User Groups deleted successfully", func(t *testing.T) {
		err := DeleteUserGroup(promodels.UserGroupName("group1"))
		assert.Nil(t, err)
		assert.False(t, DoesUserGroupExist(promodels.UserGroupName("group1")))
	})
}
