package logic

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
)

const (
	apiKeySecretPrefix = "nm_live_"
	// apiKeyPrefixLen is the stored unique prefix used for lookup (includes nm_live_).
	apiKeyPrefixLen = 16
	// apiKeyRandomLen is the length of the random portion after nm_live_.
	apiKeyRandomLen = 40
	apiKeyCharset   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

type apiKeyContextKey struct{}

// APIKeyAuthContext is the authenticated API key identity attached to a request.
type APIKeyAuthContext struct {
	APIKeyID   string
	TenantID   string
	Name       string
	Permission schema.APIKeyPermission
	Scope      models.APIKeyNetworkScope
}

// IsAPIKeySecret reports whether the token looks like a Netmaker API key secret.
func IsAPIKeySecret(token string) bool {
	return strings.HasPrefix(token, apiKeySecretPrefix)
}

// WithAPIKeyContext stores the API key identity on the context.
func WithAPIKeyContext(ctx context.Context, auth *APIKeyAuthContext) context.Context {
	return context.WithValue(ctx, apiKeyContextKey{}, auth)
}

// APIKeyFromContext returns the API key identity if the request was authenticated with an API key.
func APIKeyFromContext(ctx context.Context) *APIKeyAuthContext {
	if ctx == nil {
		return nil
	}
	auth, _ := ctx.Value(apiKeyContextKey{}).(*APIKeyAuthContext)
	return auth
}

// IsAPIKeyAuth reports whether the context carries an API key identity.
func IsAPIKeyAuth(ctx context.Context) bool {
	return APIKeyFromContext(ctx) != nil
}

// APIKeyActorName returns the display actor string for logs and audit events.
func APIKeyActorName(name string) string {
	return "API Key: " + name
}

func generateAPIKeySecret() (fullSecret, prefix string, err error) {
	randomPart := make([]byte, apiKeyRandomLen)
	charsetLen := big.NewInt(int64(len(apiKeyCharset)))
	for i := 0; i < apiKeyRandomLen; i++ {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", "", fmt.Errorf("generate api key secret: %w", err)
		}
		randomPart[i] = apiKeyCharset[n.Int64()]
	}
	fullSecret = apiKeySecretPrefix + string(randomPart)
	if len(fullSecret) < apiKeyPrefixLen {
		return "", "", errors.New("generated api key too short")
	}
	return fullSecret, fullSecret[:apiKeyPrefixLen], nil
}

func hashAPIKeySecret(secret string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), 5)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyAPIKeySecret(hash, secret string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) == nil
}

// ToAPIKeyResponse converts a stored key into a safe API response (no hash/secret).
func ToAPIKeyResponse(key *schema.APIKey) models.APIKeyResponse {
	scopeType := key.NetworkScopeType
	networkIDs := []string(key.NetworkIDs)
	if scopeType == schema.APIKeyNetworkScopeAll {
		networkIDs = nil
	}
	return models.APIKeyResponse{
		ID:         key.ID,
		Name:       key.Name,
		KeyPrefix:  key.KeyPrefix,
		Permission: key.Permission,
		NetworkScope: models.APIKeyNetworkScope{
			Type:       scopeType,
			NetworkIDs: networkIDs,
		},
		CreatedBy:  key.CreatedBy,
		CreatedAt:  key.CreatedAt,
		ExpiresAt:  key.ExpiresAt,
		LastUsedAt: key.LastUsedAt,
		Revoked:    key.RevokedAt != nil,
	}
}

// ValidateCreateAPIKeyRequest validates create payload and returns a normalized scope.
func ValidateCreateAPIKeyRequest(ctx context.Context, req *models.CreateAPIKeyRequest) (models.APIKeyNetworkScope, error) {
	if req == nil {
		return models.APIKeyNetworkScope{}, errors.New("request is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return models.APIKeyNetworkScope{}, errors.New("name is required")
	}
	switch req.Permission {
	case schema.APIKeyPermissionRead, schema.APIKeyPermissionModify, schema.APIKeyPermissionFullAccess:
	default:
		return models.APIKeyNetworkScope{}, errors.New("permission must be read, modify, or full_access")
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.IsZero() && req.ExpiresAt.Before(time.Now().UTC()) {
		return models.APIKeyNetworkScope{}, errors.New("expires_at must be in the future")
	}

	switch req.NetworkScope.Type {
	case schema.APIKeyNetworkScopeAll:
		return models.APIKeyNetworkScope{Type: schema.APIKeyNetworkScopeAll}, nil
	case schema.APIKeyNetworkScopeSelected:
		if len(req.NetworkScope.NetworkIDs) == 0 {
			return models.APIKeyNetworkScope{}, errors.New("network_ids is required when network_scope.type is selected")
		}
		seen := make(map[string]struct{}, len(req.NetworkScope.NetworkIDs))
		normalized := make([]string, 0, len(req.NetworkScope.NetworkIDs))
		for _, id := range req.NetworkScope.NetworkIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			net, err := resolveNetwork(ctx, id)
			if err != nil {
				return models.APIKeyNetworkScope{}, fmt.Errorf("network %q not found in tenant", id)
			}
			if _, ok := seen[net.ID]; ok {
				continue
			}
			seen[id] = struct{}{}
			seen[net.ID] = struct{}{}
			normalized = append(normalized, net.ID)
		}
		if len(normalized) == 0 {
			return models.APIKeyNetworkScope{}, errors.New("network_ids is required when network_scope.type is selected")
		}
		return models.APIKeyNetworkScope{
			Type:       schema.APIKeyNetworkScopeSelected,
			NetworkIDs: normalized,
		}, nil
	default:
		return models.APIKeyNetworkScope{}, errors.New("network_scope.type must be all or selected")
	}
}

// CreateAPIKey generates a secret, persists the hashed key, and returns the one-time plaintext secret.
func CreateAPIKey(ctx context.Context, req *models.CreateAPIKeyRequest, createdBy string) (*schema.APIKey, string, error) {
	normalizedScope, err := ValidateCreateAPIKeyRequest(ctx, req)
	if err != nil {
		return nil, "", err
	}

	secret, prefix, err := generateAPIKeySecret()
	if err != nil {
		return nil, "", err
	}
	hash, err := hashAPIKeySecret(secret)
	if err != nil {
		return nil, "", err
	}

	now := time.Now().UTC()
	key := &schema.APIKey{
		ID:               uuid.NewString(),
		TenantID:         scope.ID(ctx),
		Name:             strings.TrimSpace(req.Name),
		KeyPrefix:        prefix,
		KeyHash:          hash,
		Permission:       req.Permission,
		NetworkScopeType: normalizedScope.Type,
		NetworkIDs:       datatypes.JSONSlice[string](normalizedScope.NetworkIDs),
		CreatedBy:        createdBy,
		CreatedAt:        now,
		ExpiresAt:        req.ExpiresAt,
	}
	if err := key.Create(ctx); err != nil {
		return nil, "", err
	}
	return key, secret, nil
}

// AuthenticateAPIKey validates a Bearer API key secret and returns an auth context.
// Failures are authentication failures (caller should return 401).
func AuthenticateAPIKey(ctx context.Context, fullSecret string) (*APIKeyAuthContext, error) {
	if !IsAPIKeySecret(fullSecret) || len(fullSecret) < apiKeyPrefixLen {
		return nil, Unauthorized_Err
	}

	stored := &schema.APIKey{KeyPrefix: fullSecret[:apiKeyPrefixLen]}
	if err := stored.GetByPrefix(ctx); err != nil {
		return nil, Unauthorized_Err
	}
	if !verifyAPIKeySecret(stored.KeyHash, fullSecret) {
		return nil, Unauthorized_Err
	}
	if stored.RevokedAt != nil {
		return nil, Unauthorized_Err
	}
	if stored.ExpiresAt != nil && !stored.ExpiresAt.IsZero() && time.Now().UTC().After(*stored.ExpiresAt) {
		return nil, Unauthorized_Err
	}
	tenantID := scope.ID(ctx)
	if tenantID != "" && stored.TenantID != "" && stored.TenantID != tenantID {
		return nil, Unauthorized_Err
	}

	now := time.Now().UTC()
	stored.LastUsedAt = &now
	_ = stored.Update(ctx)

	networkIDs := []string(stored.NetworkIDs)
	if stored.NetworkScopeType == schema.APIKeyNetworkScopeAll {
		networkIDs = nil
	}
	return &APIKeyAuthContext{
		APIKeyID:   stored.ID,
		TenantID:   stored.TenantID,
		Name:       stored.Name,
		Permission: stored.Permission,
		Scope: models.APIKeyNetworkScope{
			Type:       stored.NetworkScopeType,
			NetworkIDs: networkIDs,
		},
	}, nil
}

// RenameAPIKey updates only the display name of an API key.
func RenameAPIKey(ctx context.Context, id, name string) (*schema.APIKey, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	key := &schema.APIKey{ID: id}
	if err := key.Get(ctx); err != nil {
		return nil, err
	}
	if key.RevokedAt != nil {
		return nil, errors.New("cannot rename a revoked api key")
	}
	key.Name = name
	if err := key.Update(ctx); err != nil {
		return nil, err
	}
	return key, nil
}

// RevokeAPIKey sets revoked_at. Revocation is immediate and non-reversible in MVP.
func RevokeAPIKey(ctx context.Context, id string) (*schema.APIKey, error) {
	key := &schema.APIKey{ID: id}
	if err := key.Get(ctx); err != nil {
		return nil, err
	}
	if key.RevokedAt != nil {
		return nil, errors.New("api key already revoked")
	}
	now := time.Now().UTC()
	key.RevokedAt = &now
	if err := key.Update(ctx); err != nil {
		return nil, err
	}
	return key, nil
}

// GetAPIKey returns a tenant-scoped API key by ID.
func GetAPIKey(ctx context.Context, id string) (*schema.APIKey, error) {
	key := &schema.APIKey{ID: id}
	if err := key.Get(ctx); err != nil {
		return nil, err
	}
	return key, nil
}

// ListAPIKeys returns all API keys for the current tenant.
func ListAPIKeys(ctx context.Context) ([]schema.APIKey, error) {
	return (&schema.APIKey{}).List(ctx)
}
