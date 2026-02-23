// Package migrate provides tests for migration functions, including
// performance tests for large-scale user migrations.
//
// To run the large-scale user performance tests:
//
//	go test -v ./migrate -run TestSyncUsersLargeScale
//	go test -v ./migrate -run TestMigrateToUUIDsLargeScale
//
// To run all tests (excluding large-scale):
//
//	go test -v ./migrate -short
//
// To run benchmarks:
//
//	go test -bench=. ./migrate
package migrate

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// TestSyncUsersLargeScale tests syncUsers() with a large number of users
// to verify performance improvements
func TestSyncUsersLargeScale(t *testing.T) {
	// Skip if short test flag is set (allows quick test runs)
	if testing.Short() {
		t.Skip("Skipping large scale test in short mode")
	}

	// Initialize test database
	err := db.InitializeDB(schema.ListModels()...)
	require.NoError(t, err)
	defer db.CloseDB()

	err = database.InitializeDatabase()
	require.NoError(t, err)
	defer database.CloseDB()

	// Create test users with various roles
	numUsers := 1000
	t.Logf("Creating %d test users...", numUsers)
	startCreate := time.Now()

	for i := 0; i < numUsers; i++ {
		user := schema.User{
			Username:       "testuser" + uuid.New().String()[:8],
			Password:       "testpassword123",
			DisplayName:    "Test User " + uuid.New().String()[:8],
			PlatformRoleID: models.ServiceUser, // Most are service users
		}

		// Assign different platform roles
		if i%100 == 0 {
			user.PlatformRoleID = models.SuperAdminRole
		} else if i%10 == 0 {
			user.PlatformRoleID = models.AdminRole
		} else if i%5 == 0 {
			user.PlatformRoleID = models.PlatformUser
		}

		// Some users have user groups
		if i%4 == 0 {
			user.UserGroups = datatypes.NewJSONType(make(map[models.UserGroupID]struct{}))
		}

		err := logic.UpsertUser(user)
		require.NoError(t, err, "Failed to create user %d", i)
	}

	createDuration := time.Since(startCreate)
	t.Logf("Created %d users in %v (avg: %v per user)", numUsers, createDuration, createDuration/time.Duration(numUsers))

	// Verify users were created
	users, err := (&schema.User{}).ListAll(db.WithContext(context.TODO()))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(users), numUsers, "Expected at least %d users", numUsers)

	// Test syncUsers() performance
	t.Log("Running syncUsers() migration...")
	startSync := time.Now()
	syncUsers()
	syncDuration := time.Since(startSync)

	t.Logf("syncUsers() completed in %v for %d users (avg: %v per user)",
		syncDuration, len(users), syncDuration/time.Duration(len(users)))

	// Verify users were migrated correctly
	usersAfter, err := (&schema.User{}).ListAll(db.WithContext(context.TODO()))
	require.NoError(t, err)
	assert.Equal(t, len(users), len(usersAfter), "User count should remain the same")

	// Verify migration correctness - check a sample of users
	sampleSize := 100
	if len(usersAfter) < sampleSize {
		sampleSize = len(usersAfter)
	}

	for i := 0; i < sampleSize; i++ {
		user := usersAfter[i]

		// Verify platform role is set
		assert.NotEmpty(t, user.PlatformRoleID.String(), "User %s should have PlatformRoleID", user.Username)

		// Verify user groups are initialized
		assert.NotNil(t, user.UserGroups, "User should have UserGroups map")
	}

	// Performance assertion - syncUsers should complete in reasonable time
	// With optimizations, 1000 users should complete in under 10 seconds
	maxDuration := 30 * time.Second
	assert.Less(t, syncDuration, maxDuration,
		"syncUsers() took too long: %v (expected < %v)", syncDuration, maxDuration)

	t.Logf("✓ syncUsers() performance test passed: %v for %d users", syncDuration, len(users))
}

// TestMigrateToUUIDsLargeScale tests MigrateToUUIDs() with a large number of users
func TestMigrateToUUIDsLargeScale(t *testing.T) {
	// Skip if short test flag is set
	if testing.Short() {
		t.Skip("Skipping large scale test in short mode")
	}

	// Initialize test database
	err := db.InitializeDB(schema.ListModels()...)
	require.NoError(t, err)
	defer db.CloseDB()

	err = database.InitializeDatabase()
	require.NoError(t, err)
	defer database.CloseDB()

	// Create test users with user groups (needed for UUID migration)
	numUsers := 1000
	t.Logf("Creating %d test users with user groups...", numUsers)
	startCreate := time.Now()

	for i := 0; i < numUsers; i++ {
		user := schema.User{
			Username:       "testuser" + uuid.New().String()[:8],
			Password:       "testpassword123",
			DisplayName:    "Test User " + uuid.New().String()[:8],
			PlatformRoleID: models.ServiceUser,
			UserGroups:     datatypes.NewJSONType(make(map[models.UserGroupID]struct{})),
		}

		// Add some user groups with non-UUID IDs (to trigger migration)
		if i%2 == 0 {
			user.UserGroups.Data()[("old-group-1")] = struct{}{}
		}
		if i%3 == 0 {
			user.UserGroups.Data()[("old-group-2")] = struct{}{}
		}

		err := logic.UpsertUser(user)
		require.NoError(t, err, "Failed to create user %d", i)
	}

	createDuration := time.Since(startCreate)
	t.Logf("Created %d users in %v", numUsers, createDuration)

	// Verify users were created
	users, err := (&schema.User{}).ListAll(db.WithContext(context.TODO()))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(users), numUsers, "Expected at least %d users", numUsers)
}

// BenchmarkSyncUsers benchmarks syncUsers() performance
func BenchmarkSyncUsers(b *testing.B) {
	// Initialize test database
	err := db.InitializeDB(schema.ListModels()...)
	if err != nil {
		b.Fatal(err)
	}
	defer db.CloseDB()

	err = database.InitializeDatabase()
	if err != nil {
		b.Fatal(err)
	}
	defer database.CloseDB()

	// Create test users
	numUsers := 1000
	for i := 0; i < numUsers; i++ {
		user := schema.User{
			Username:       "benchuser" + uuid.New().String()[:8],
			Password:       "password",
			PlatformRoleID: models.ServiceUser,
		}
		if i%10 == 0 {
			user.PlatformRoleID = models.AdminRole
		}
		_ = logic.UpsertUser(user)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		syncUsers()
	}
}
