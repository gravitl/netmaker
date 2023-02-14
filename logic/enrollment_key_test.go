package logic

import (
	"testing"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/stretchr/testify/assert"
)

func TestCreate_EnrollmentKey(t *testing.T) {
	database.InitializeDatabase()
	defer database.CloseDB()
	t.Run("Can_Not_Create_Key", func(t *testing.T) {
		newKey, err := CreateEnrollmentKey(0, time.Time{}, nil, nil, false)
		assert.Nil(t, newKey)
		assert.Equal(t, err.Error(), EnrollmentKeyErrors.InvalidCreate)
	})
	t.Run("Can_Create_Key_Uses", func(t *testing.T) {
		newKey, err := CreateEnrollmentKey(1, time.Time{}, nil, nil, false)
		assert.Nil(t, err)
		assert.Equal(t, 1, newKey.UsesRemaining)
		assert.True(t, newKey.IsValid())
	})
	t.Run("Can_Create_Key_Time", func(t *testing.T) {
		newKey, err := CreateEnrollmentKey(0, time.Now().Add(time.Minute), nil, nil, false)
		assert.Nil(t, err)
		assert.True(t, newKey.IsValid())
	})
	t.Run("Can_Create_Key_Unlimited", func(t *testing.T) {
		newKey, err := CreateEnrollmentKey(0, time.Time{}, nil, nil, true)
		assert.Nil(t, err)
		assert.True(t, newKey.IsValid())
	})
	t.Run("Can_Create_Key_WithNetworks", func(t *testing.T) {
		newKey, err := CreateEnrollmentKey(0, time.Time{}, []string{"mynet", "skynet"}, nil, true)
		assert.Nil(t, err)
		assert.True(t, newKey.IsValid())
		assert.True(t, len(newKey.Networks) == 2)
	})
	t.Run("Can_Create_Key_WithTags", func(t *testing.T) {
		newKey, err := CreateEnrollmentKey(0, time.Time{}, nil, []string{"tag1", "tag2"}, true)
		assert.Nil(t, err)
		assert.True(t, newKey.IsValid())
		assert.True(t, len(newKey.Tags) == 2)
	})
	removeAllEnrollments()
}

func TestDelete_EnrollmentKey(t *testing.T) {
	database.InitializeDatabase()
	defer database.CloseDB()

}

func TestDecrement_EnrollmentKey(t *testing.T) {
	database.InitializeDatabase()
	defer database.CloseDB()

}

func TestValidity_EnrollmentKey(t *testing.T) {
	database.InitializeDatabase()
	defer database.CloseDB()

}

func removeAllEnrollments() {
	database.DeleteAllRecords(database.ENROLLMENT_KEYS_TABLE_NAME)
}
