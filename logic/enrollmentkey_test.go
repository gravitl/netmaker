package logic

import (
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

func TestCreateEnrollmentKey(t *testing.T) {
	db.InitializeDB(schema.ListModels()...)
	defer db.CloseDB()

	database.InitializeDatabase()
	defer database.CloseDB()
	t.Run("Can_Not_Create_Key", func(t *testing.T) {
		newKey, err := CreateEnrollmentKey(0, time.Time{}, nil, nil, nil, false, uuid.Nil, false, false)
		assert.Nil(t, newKey)
		assert.NotNil(t, err)
		assert.ErrorIs(t, err, models.ErrInvalidEnrollmentKey)
	})
	t.Run("Can_Create_Key_Uses", func(t *testing.T) {
		newKey, err := CreateEnrollmentKey(1, time.Time{}, nil, nil, nil, false, uuid.Nil, false, false)
		assert.Nil(t, err)
		assert.Equal(t, 1, newKey.UsesRemaining)
		assert.True(t, newKey.IsValid())
	})
	t.Run("Can_Create_Key_Time", func(t *testing.T) {
		newKey, err := CreateEnrollmentKey(0, time.Now().Add(time.Minute), nil, nil, nil, false, uuid.Nil, false, false)
		assert.Nil(t, err)
		assert.True(t, newKey.IsValid())
	})
	t.Run("Can_Create_Key_Unlimited", func(t *testing.T) {
		newKey, err := CreateEnrollmentKey(0, time.Time{}, nil, nil, nil, true, uuid.Nil, false, false)
		assert.Nil(t, err)
		assert.True(t, newKey.IsValid())
	})
	t.Run("Can_Create_Key_WithNetworks", func(t *testing.T) {
		newKey, err := CreateEnrollmentKey(0, time.Time{}, []string{"mynet", "skynet"}, nil, nil, true, uuid.Nil, false, false)
		assert.Nil(t, err)
		assert.True(t, newKey.IsValid())
		assert.True(t, len(newKey.Networks) == 2)
	})
	t.Run("Can_Create_Key_WithTags", func(t *testing.T) {
		newKey, err := CreateEnrollmentKey(0, time.Time{}, nil, []string{"tag1", "tag2"}, nil, true, uuid.Nil, false, false)
		assert.Nil(t, err)
		assert.True(t, newKey.IsValid())
		assert.True(t, len(newKey.Tags) == 2)
	})

	t.Run("Can_Get_List_of_Keys", func(t *testing.T) {
		keys, err := GetAllEnrollmentKeys()
		assert.Nil(t, err)
		assert.True(t, len(keys) > 0)
		for i := range keys {
			assert.Equal(t, len(keys[i].Value), models.EnrollmentKeyLength)
		}
	})
	removeAllEnrollments()
}

func TestDelete_EnrollmentKey(t *testing.T) {
	db.InitializeDB(schema.ListModels()...)
	defer db.CloseDB()

	database.InitializeDatabase()
	defer database.CloseDB()
	newKey, _ := CreateEnrollmentKey(0, time.Time{}, []string{"mynet", "skynet"}, nil, nil, true, uuid.Nil, false, false)
	t.Run("Can_Delete_Key", func(t *testing.T) {
		assert.True(t, newKey.IsValid())
		err := DeleteEnrollmentKey(newKey.Value, false)
		assert.Nil(t, err)
		oldKey, err := GetEnrollmentKey(newKey.Value)
		assert.Equal(t, oldKey, models.EnrollmentKey{})
		assert.NotNil(t, err)
		assert.Equal(t, err, EnrollmentErrors.NoKeyFound)
	})
	t.Run("Can_Not_Delete_Invalid_Key", func(t *testing.T) {
		err := DeleteEnrollmentKey("notakey", false)
		assert.NotNil(t, err)
		assert.Equal(t, err, EnrollmentErrors.NoKeyFound)
	})
	removeAllEnrollments()
}

func TestDecrement_EnrollmentKey(t *testing.T) {
	db.InitializeDB(schema.ListModels()...)
	defer db.CloseDB()

	database.InitializeDatabase()
	defer database.CloseDB()
	newKey, _ := CreateEnrollmentKey(1, time.Time{}, nil, nil, nil, false, uuid.Nil, false, false)
	t.Run("Check_initial_uses", func(t *testing.T) {
		assert.True(t, newKey.IsValid())
		assert.Equal(t, newKey.UsesRemaining, 1)
	})
	t.Run("Check can decrement", func(t *testing.T) {
		assert.Equal(t, newKey.UsesRemaining, 1)
		k, err := decrementEnrollmentKey(newKey.Value)
		assert.Nil(t, err)
		newKey = k
	})
	t.Run("Check can not decrement", func(t *testing.T) {
		assert.Equal(t, newKey.UsesRemaining, 0)
		_, err := decrementEnrollmentKey(newKey.Value)
		assert.NotNil(t, err)
		assert.Equal(t, err, EnrollmentErrors.NoUsesRemaining)
	})

	removeAllEnrollments()
}

func TestUsability_EnrollmentKey(t *testing.T) {
	db.InitializeDB(schema.ListModels()...)
	defer db.CloseDB()

	database.InitializeDatabase()
	defer database.CloseDB()
	key1, _ := CreateEnrollmentKey(1, time.Time{}, nil, nil, nil, false, uuid.Nil, false, false)
	key2, _ := CreateEnrollmentKey(0, time.Now().Add(time.Minute<<4), nil, nil, nil, false, uuid.Nil, false, false)
	key3, _ := CreateEnrollmentKey(0, time.Time{}, nil, nil, nil, true, uuid.Nil, false, false)
	t.Run("Check if valid use key can be used", func(t *testing.T) {
		assert.Equal(t, key1.UsesRemaining, 1)
		ok := TryToUseEnrollmentKey(key1)
		assert.True(t, ok)
		assert.Equal(t, 0, key1.UsesRemaining)
	})

	t.Run("Check if valid time key can be used", func(t *testing.T) {
		assert.True(t, !key2.Expiration.IsZero())
		ok := TryToUseEnrollmentKey(key2)
		assert.True(t, ok)
	})

	t.Run("Check if valid unlimited key can be used", func(t *testing.T) {
		assert.True(t, key3.Unlimited)
		ok := TryToUseEnrollmentKey(key3)
		assert.True(t, ok)
	})

	t.Run("check invalid key can not be used", func(t *testing.T) {
		ok := TryToUseEnrollmentKey(key1)
		assert.False(t, ok)
	})
}

func removeAllEnrollments() {
	database.DeleteAllRecords(database.ENROLLMENT_KEYS_TABLE_NAME)
}

//Test that cheks if it can tokenize
//Test that cheks if it can't tokenize

func TestTokenize_EnrollmentKeys(t *testing.T) {
	db.InitializeDB(schema.ListModels()...)
	defer db.CloseDB()

	database.InitializeDatabase()
	defer database.CloseDB()
	newKey, _ := CreateEnrollmentKey(0, time.Time{}, []string{"mynet", "skynet"}, nil, nil, true, uuid.Nil, false, false)
	const defaultValue = "MwE5MwE5MwE5MwE5MwE5MwE5MwE5MwE5"
	const b64value = "eyJzZXJ2ZXIiOiJhcGkubXlzZXJ2ZXIuY29tIiwidmFsdWUiOiJNd0U1TXdFNU13RTVNd0U1TXdFNU13RTVNd0U1TXdFNSJ9"
	const serverAddr = "api.myserver.com"
	t.Run("Can_Not_Tokenize_Nil_Key", func(t *testing.T) {
		err := Tokenize(nil, "ServerAddress")
		assert.NotNil(t, err)
		assert.Equal(t, err, EnrollmentErrors.FailedToTokenize)
	})
	t.Run("Can_Not_Tokenize_Empty_Server_Address", func(t *testing.T) {
		err := Tokenize(newKey, "")
		assert.NotNil(t, err)
		assert.Equal(t, err, EnrollmentErrors.FailedToTokenize)
	})

	t.Run("Can_Tokenize", func(t *testing.T) {
		err := Tokenize(newKey, serverAddr)
		assert.Nil(t, err)
		assert.True(t, len(newKey.Token) > 0)
	})

	t.Run("Is_Correct_B64_Token", func(t *testing.T) {
		newKey.Value = defaultValue
		err := Tokenize(newKey, serverAddr)
		assert.Nil(t, err)
		assert.Equal(t, newKey.Token, b64value)
	})
	removeAllEnrollments()
}

func TestDeTokenize_EnrollmentKeys(t *testing.T) {
	db.InitializeDB(schema.ListModels()...)
	defer db.CloseDB()

	database.InitializeDatabase()
	defer database.CloseDB()
	newKey, _ := CreateEnrollmentKey(0, time.Time{}, []string{"mynet", "skynet"}, nil, nil, true, uuid.Nil, false, false)
	const b64Value = "eyJzZXJ2ZXIiOiJhcGkubXlzZXJ2ZXIuY29tIiwidmFsdWUiOiJNd0U1TXdFNU13RTVNd0U1TXdFNU13RTVNd0U1TXdFNSJ9"
	const serverAddr = "api.myserver.com"

	t.Run("Can_Not_DeTokenize", func(t *testing.T) {
		value, err := DeTokenize("")
		assert.Nil(t, value)
		assert.NotNil(t, err)
		assert.Equal(t, err, EnrollmentErrors.FailedToDeTokenize)
	})
	t.Run("Can_Not_Find_Key", func(t *testing.T) {
		value, err := DeTokenize(b64Value)
		assert.Nil(t, value)
		assert.NotNil(t, err)
		assert.Equal(t, err, EnrollmentErrors.NoKeyFound)
	})
	t.Run("Can_DeTokenize", func(t *testing.T) {
		err := Tokenize(newKey, serverAddr)
		assert.Nil(t, err)
		output, err := DeTokenize(newKey.Token)
		assert.Nil(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, newKey.Value, output.Value)
	})

	removeAllEnrollments()
}
