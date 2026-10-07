package logic

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestNetwork(t *testing.T, ctx context.Context, name string) *schema.Network {
	t.Helper()
	net := &schema.Network{
		ID:           uuid.NewString(),
		TenantID:     scope.ID(ctx),
		Name:         name,
		AddressRange: "10.100.0.0/16",
		CreatedAt:    time.Now().UTC(),
	}
	require.NoError(t, net.Create(ctx))
	t.Cleanup(func() { _ = net.Delete(ctx) })
	return net
}

func TestCreateAndAuthenticateAPIKey(t *testing.T) {
	ctx := scopedTestContext(t)
	netA := createTestNetwork(t, ctx, "apikey-net-a-"+uuid.NewString()[:8])

	key, secret, err := CreateAPIKey(ctx, &models.CreateAPIKeyRequest{
		Name:       "terraform",
		Permission: schema.APIKeyPermissionModify,
		NetworkScope: models.APIKeyNetworkScope{
			Type:       schema.APIKeyNetworkScopeSelected,
			NetworkIDs: []string{netA.ID},
		},
	}, "admin")
	require.NoError(t, err)
	require.NotEmpty(t, secret)
	assert.True(t, IsAPIKeySecret(secret))
	assert.Equal(t, secret[:apiKeyPrefixLen], key.KeyPrefix)
	assert.NotEmpty(t, key.KeyHash)
	stored, err := GetAPIKey(ctx, key.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, stored.KeyHash)
	safe := ToAPIKeyResponse(stored)
	assert.Equal(t, key.KeyPrefix, safe.KeyPrefix)
	assert.False(t, safe.Revoked)
	t.Cleanup(func() {
		now := time.Now().UTC()
		stored.RevokedAt = &now
		_ = stored.Update(ctx)
	})

	auth, err := AuthenticateAPIKey(ctx, secret)
	require.NoError(t, err)
	assert.Equal(t, key.ID, auth.APIKeyID)
	assert.Equal(t, schema.APIKeyPermissionModify, auth.Permission)
	assert.Equal(t, schema.APIKeyNetworkScopeSelected, auth.Scope.Type)
	assert.Contains(t, auth.Scope.NetworkIDs, netA.ID)

	_, err = AuthenticateAPIKey(ctx, "nm_live_notavalidsecretxxxxxxxxxxxxxx")
	assert.Error(t, err)
}

func TestAuthenticateAPIKeyExpiredAndRevoked(t *testing.T) {
	ctx := scopedTestContext(t)

	past := time.Now().UTC().Add(-time.Hour)
	_, secret, err := CreateAPIKey(ctx, &models.CreateAPIKeyRequest{
		Name:       "expired-key",
		Permission: schema.APIKeyPermissionRead,
		ExpiresAt:  &past,
		NetworkScope: models.APIKeyNetworkScope{
			Type: schema.APIKeyNetworkScopeAll,
		},
	}, "admin")
	// Create rejects past expires_at
	require.Error(t, err)
	assert.Empty(t, secret)

	future := time.Now().UTC().Add(time.Hour)
	key, secret, err := CreateAPIKey(ctx, &models.CreateAPIKeyRequest{
		Name:       "soon-expire",
		Permission: schema.APIKeyPermissionRead,
		ExpiresAt:  &future,
		NetworkScope: models.APIKeyNetworkScope{
			Type: schema.APIKeyNetworkScopeAll,
		},
	}, "admin")
	require.NoError(t, err)

	// Force expire in DB
	expired := time.Now().UTC().Add(-time.Minute)
	key.ExpiresAt = &expired
	require.NoError(t, key.Update(ctx))
	_, err = AuthenticateAPIKey(ctx, secret)
	assert.ErrorIs(t, err, Unauthorized_Err)

	key2, secret2, err := CreateAPIKey(ctx, &models.CreateAPIKeyRequest{
		Name:       "revoke-me",
		Permission: schema.APIKeyPermissionRead,
		NetworkScope: models.APIKeyNetworkScope{
			Type: schema.APIKeyNetworkScopeAll,
		},
	}, "admin")
	require.NoError(t, err)
	_, err = RevokeAPIKey(ctx, key2.ID)
	require.NoError(t, err)
	_, err = AuthenticateAPIKey(ctx, secret2)
	assert.ErrorIs(t, err, Unauthorized_Err)

	// Revoke is non-reversible
	_, err = RevokeAPIKey(ctx, key2.ID)
	assert.Error(t, err)
}

func TestAPIKeyTenantIsolation(t *testing.T) {
	ctx := scopedTestContext(t)
	key, secret, err := CreateAPIKey(ctx, &models.CreateAPIKeyRequest{
		Name:       "tenant-a-key",
		Permission: schema.APIKeyPermissionRead,
		NetworkScope: models.APIKeyNetworkScope{
			Type: schema.APIKeyNetworkScopeAll,
		},
	}, "admin")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = RevokeAPIKey(ctx, key.ID)
	})

	otherTenantCtx := scope.WithContext(ctx, scope.TenantScope, "other-tenant-"+uuid.NewString())
	_, err = AuthenticateAPIKey(otherTenantCtx, secret)
	assert.ErrorIs(t, err, Unauthorized_Err)

	auth, err := AuthenticateAPIKey(ctx, secret)
	require.NoError(t, err)
	assert.Equal(t, key.TenantID, auth.TenantID)
}

func TestAPIKeyNetworkScopeAndPermissions(t *testing.T) {
	ctx := scopedTestContext(t)
	netA := createTestNetwork(t, ctx, "scope-a-"+uuid.NewString()[:8])
	netB := createTestNetwork(t, ctx, "scope-b-"+uuid.NewString()[:8])
	netC := createTestNetwork(t, ctx, "scope-c-"+uuid.NewString()[:8])

	key, secret, err := CreateAPIKey(ctx, &models.CreateAPIKeyRequest{
		Name:       "selected-modify",
		Permission: schema.APIKeyPermissionModify,
		NetworkScope: models.APIKeyNetworkScope{
			Type:       schema.APIKeyNetworkScopeSelected,
			NetworkIDs: []string{netA.ID, netB.ID},
		},
	}, "admin")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = RevokeAPIKey(ctx, key.ID) })

	auth, err := AuthenticateAPIKey(ctx, secret)
	require.NoError(t, err)
	ctx = WithAPIKeyContext(ctx, auth)

	require.NoError(t, AuthorizeAPIKey(ctx, netA.Name, schema.APIKeyPermissionRead))
	require.NoError(t, AuthorizeAPIKey(ctx, netA.Name, schema.APIKeyPermissionModify))
	assert.Error(t, AuthorizeAPIKey(ctx, netA.Name, schema.APIKeyPermissionFullAccess))
	assert.Error(t, AuthorizeAPIKey(ctx, netC.Name, schema.APIKeyPermissionRead))

	allNetworks := []schema.Network{*netA, *netB, *netC}
	filtered := FilterNetworksByAPIKey(ctx, allNetworks)
	require.Len(t, filtered, 2)
	ids := map[string]bool{}
	for _, n := range filtered {
		ids[n.ID] = true
	}
	assert.True(t, ids[netA.ID])
	assert.True(t, ids[netB.ID])
	assert.False(t, ids[netC.ID])

	// all-scope includes newly created networks
	allKey, allSecret, err := CreateAPIKey(ctx, &models.CreateAPIKeyRequest{
		Name:       "all-read",
		Permission: schema.APIKeyPermissionRead,
		NetworkScope: models.APIKeyNetworkScope{
			Type: schema.APIKeyNetworkScopeAll,
		},
	}, "admin")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = RevokeAPIKey(ctx, allKey.ID) })
	allAuth, err := AuthenticateAPIKey(ctx, allSecret)
	require.NoError(t, err)
	allCtx := WithAPIKeyContext(ctx, allAuth)
	newNet := createTestNetwork(t, ctx, "scope-new-"+uuid.NewString()[:8])
	require.NoError(t, AuthorizeAPIKey(allCtx, newNet.Name, schema.APIKeyPermissionRead))
	assert.Error(t, AuthorizeAPIKey(allCtx, newNet.Name, schema.APIKeyPermissionModify))

	fullKey, fullSecret, err := CreateAPIKey(ctx, &models.CreateAPIKeyRequest{
		Name:       "full",
		Permission: schema.APIKeyPermissionFullAccess,
		NetworkScope: models.APIKeyNetworkScope{
			Type: schema.APIKeyNetworkScopeAll,
		},
	}, "admin")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = RevokeAPIKey(ctx, fullKey.ID) })
	fullAuth, err := AuthenticateAPIKey(ctx, fullSecret)
	require.NoError(t, err)
	fullCtx := WithAPIKeyContext(ctx, fullAuth)
	require.NoError(t, AuthorizeAPIKey(fullCtx, netA.Name, schema.APIKeyPermissionFullAccess))
}

func TestValidateCreateAPIKeyRequest(t *testing.T) {
	ctx := scopedTestContext(t)
	net := createTestNetwork(t, ctx, "validate-net-"+uuid.NewString()[:8])

	_, err := ValidateCreateAPIKeyRequest(ctx, &models.CreateAPIKeyRequest{
		Name:       "",
		Permission: schema.APIKeyPermissionRead,
		NetworkScope: models.APIKeyNetworkScope{
			Type: schema.APIKeyNetworkScopeAll,
		},
	})
	assert.Error(t, err)

	_, err = ValidateCreateAPIKeyRequest(ctx, &models.CreateAPIKeyRequest{
		Name:       "ok",
		Permission: "write",
		NetworkScope: models.APIKeyNetworkScope{
			Type: schema.APIKeyNetworkScopeAll,
		},
	})
	assert.Error(t, err)

	_, err = ValidateCreateAPIKeyRequest(ctx, &models.CreateAPIKeyRequest{
		Name:       "ok",
		Permission: schema.APIKeyPermissionRead,
		NetworkScope: models.APIKeyNetworkScope{
			Type: schema.APIKeyNetworkScopeSelected,
		},
	})
	assert.Error(t, err)

	scopeOut, err := ValidateCreateAPIKeyRequest(ctx, &models.CreateAPIKeyRequest{
		Name:       "ok",
		Permission: schema.APIKeyPermissionRead,
		NetworkScope: models.APIKeyNetworkScope{
			Type:       schema.APIKeyNetworkScopeSelected,
			NetworkIDs: []string{net.Name},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{net.ID}, scopeOut.NetworkIDs)

	// all scope ignores network_ids
	scopeOut, err = ValidateCreateAPIKeyRequest(ctx, &models.CreateAPIKeyRequest{
		Name:       "ok",
		Permission: schema.APIKeyPermissionRead,
		NetworkScope: models.APIKeyNetworkScope{
			Type:       schema.APIKeyNetworkScopeAll,
			NetworkIDs: []string{net.ID},
		},
	})
	require.NoError(t, err)
	assert.Empty(t, scopeOut.NetworkIDs)
}

func TestRenameAPIKey(t *testing.T) {
	ctx := scopedTestContext(t)
	key, _, err := CreateAPIKey(ctx, &models.CreateAPIKeyRequest{
		Name:       "old-name",
		Permission: schema.APIKeyPermissionRead,
		NetworkScope: models.APIKeyNetworkScope{
			Type: schema.APIKeyNetworkScopeAll,
		},
	}, "admin")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = RevokeAPIKey(ctx, key.ID) })

	renamed, err := RenameAPIKey(ctx, key.ID, "new-name")
	require.NoError(t, err)
	assert.Equal(t, "new-name", renamed.Name)

	_, err = RevokeAPIKey(ctx, key.ID)
	require.NoError(t, err)
	_, err = RenameAPIKey(ctx, key.ID, "again")
	assert.Error(t, err)
}
