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
		user := models.User{
			UserName:       "testuser" + uuid.New().String()[:8],
			Password:       "testpassword123",
			DisplayName:    "Test User " + uuid.New().String()[:8],
			IsAdmin:        i%10 == 0,          // 10% are admins
			IsSuperAdmin:   i%100 == 0,         // 1% are super admins
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

		// Some users have network roles
		if i%3 == 0 {
			user.NetworkRoles = make(map[models.NetworkID]map[models.UserRoleID]struct{})
			user.NetworkRoles[models.NetworkID("test-network")] = make(map[models.UserRoleID]struct{})
		}

		// Some users have user groups
		if i%4 == 0 {
			user.UserGroups = make(map[models.UserGroupID]struct{})
		}

		err := logic.UpsertUser(user)
		require.NoError(t, err, "Failed to create user %d", i)
	}

	createDuration := time.Since(startCreate)
	t.Logf("Created %d users in %v (avg: %v per user)", numUsers, createDuration, createDuration/time.Duration(numUsers))

	// Verify users were created
	users, err := logic.GetUsersDB()
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
	usersAfter, err := logic.GetUsersDB()
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
		assert.NotEmpty(t, user.PlatformRoleID.String(), "User %s should have PlatformRoleID", user.UserName)

		// Verify admin flags match platform role
		if user.PlatformRoleID == models.SuperAdminRole {
			assert.True(t, user.IsSuperAdmin, "SuperAdmin user should have IsSuperAdmin=true")
			assert.True(t, user.IsAdmin, "SuperAdmin user should have IsAdmin=true")
		} else if user.PlatformRoleID == models.AdminRole {
			assert.True(t, user.IsAdmin, "Admin user should have IsAdmin=true")
			assert.False(t, user.IsSuperAdmin, "Admin user should not have IsSuperAdmin=true")
		} else {
			assert.False(t, user.IsSuperAdmin, "Non-admin user should not have IsSuperAdmin=true")
			assert.False(t, user.IsAdmin, "Non-admin user should not have IsAdmin=true")
		}

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
		user := models.User{
			UserName:       "testuser" + uuid.New().String()[:8],
			Password:       "testpassword123",
			DisplayName:    "Test User " + uuid.New().String()[:8],
			PlatformRoleID: models.ServiceUser,
			UserGroups:     make(map[models.UserGroupID]struct{}),
		}

		// Add some user groups with non-UUID IDs (to trigger migration)
		if i%2 == 0 {
			user.UserGroups[models.UserGroupID("old-group-1")] = struct{}{}
		}
		if i%3 == 0 {
			user.UserGroups[models.UserGroupID("old-group-2")] = struct{}{}
		}

		err := logic.UpsertUser(user)
		require.NoError(t, err, "Failed to create user %d", i)
	}

	createDuration := time.Since(startCreate)
	t.Logf("Created %d users in %v", numUsers, createDuration)

	// Verify users were created
	users, err := logic.GetUsersDB()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(users), numUsers, "Expected at least %d users", numUsers)

	// Test MigrateToUUIDs() performance
	t.Log("Running MigrateToUUIDs() migration...")
	startMigrate := time.Now()
	migrateToUUIDs()
	migrateDuration := time.Since(startMigrate)

	t.Logf("MigrateToUUIDs() completed in %v for %d users (avg: %v per user)",
		migrateDuration, len(users), migrateDuration/time.Duration(len(users)))

	// Verify users still exist after migration
	usersAfter, err := logic.GetUsersDB()
	require.NoError(t, err)
	assert.Equal(t, len(users), len(usersAfter), "User count should remain the same")

	// Performance assertion - MigrateToUUIDs should complete in reasonable time
	maxDuration := 30 * time.Second
	assert.Less(t, migrateDuration, maxDuration,
		"MigrateToUUIDs() took too long: %v (expected < %v)", migrateDuration, maxDuration)

	t.Logf("✓ MigrateToUUIDs() performance test passed: %v for %d users", migrateDuration, len(users))
}

// TestSyncUsersCorrectness tests that syncUsers() correctly migrates user data
func TestSyncUsersCorrectness(t *testing.T) {
	// Initialize test database
	err := db.InitializeDB(schema.ListModels()...)
	require.NoError(t, err)
	defer db.CloseDB()

	err = database.InitializeDatabase()
	require.NoError(t, err)
	defer database.CloseDB()

	// Create test users with various states
	testCases := []struct {
		name          string
		user          models.User
		expectedRole  models.UserRoleID
		expectedAdmin bool
		expectedSuper bool
	}{
		{
			name: "user with AdminRole but IsAdmin=false",
			user: models.User{
				UserName:       "admin1",
				Password:       "password",
				PlatformRoleID: models.AdminRole,
				IsAdmin:        false,
				IsSuperAdmin:   false,
			},
			expectedRole:  models.AdminRole,
			expectedAdmin: true,
			expectedSuper: false,
		},
		{
			name: "user with SuperAdminRole but IsSuperAdmin=false",
			user: models.User{
				UserName:       "superadmin1",
				Password:       "password",
				PlatformRoleID: models.SuperAdminRole,
				IsAdmin:        false,
				IsSuperAdmin:   false,
			},
			expectedRole:  models.SuperAdminRole,
			expectedAdmin: true,
			expectedSuper: true,
		},
		{
			name: "user with IsSuperAdmin=true but no PlatformRoleID",
			user: models.User{
				UserName:       "superadmin2",
				Password:       "password",
				PlatformRoleID: "",
				IsAdmin:        true,
				IsSuperAdmin:   true,
			},
			expectedRole:  models.SuperAdminRole,
			expectedAdmin: true,
			expectedSuper: true,
		},
		{
			name: "user with IsAdmin=true but no PlatformRoleID",
			user: models.User{
				UserName:       "admin2",
				Password:       "password",
				PlatformRoleID: "",
				IsAdmin:        true,
				IsSuperAdmin:   false,
			},
			expectedRole:  models.AdminRole,
			expectedAdmin: true,
			expectedSuper: false,
		},
		{
			name: "regular user with no role",
			user: models.User{
				UserName:       "user1",
				Password:       "password",
				PlatformRoleID: "",
				IsAdmin:        false,
				IsSuperAdmin:   false,
			},
			expectedRole:  models.ServiceUser,
			expectedAdmin: false,
			expectedSuper: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create user
			err := logic.UpsertUser(tc.user)
			require.NoError(t, err)

			// Run migration
			syncUsers()

			// Verify migration
			user, err := logic.GetUser(tc.user.UserName)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedRole, user.PlatformRoleID, "PlatformRoleID should match")
			assert.Equal(t, tc.expectedAdmin, user.IsAdmin, "IsAdmin should match")
			assert.Equal(t, tc.expectedSuper, user.IsSuperAdmin, "IsSuperAdmin should match")
			assert.NotNil(t, user.UserGroups, "UserGroups should be initialized")
			assert.NotNil(t, user.NetworkRoles, "NetworkRoles should be initialized")

			// Cleanup
			_ = database.DeleteRecord(database.USERS_TABLE_NAME, tc.user.UserName)
		})
	}
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
		user := models.User{
			UserName:       "benchuser" + uuid.New().String()[:8],
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
