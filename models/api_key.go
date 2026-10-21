package models

import (
	"time"

	"github.com/gravitl/netmaker/schema"
)

// APIKeyNetworkScope describes which networks an API key can access.
type APIKeyNetworkScope struct {
	Type       string   `json:"type"`
	NetworkIDs []string `json:"network_ids,omitempty"`
}

// CreateAPIKeyRequest is the body for POST /api/v1/api-keys.
type CreateAPIKeyRequest struct {
	Name         string                  `json:"name"`
	ExpiresAt    *time.Time              `json:"expires_at"`
	NetworkScope APIKeyNetworkScope      `json:"network_scope"`
	Permission   schema.APIKeyPermission `json:"permission"`
}

// RenameAPIKeyRequest is the body for PATCH /api/v1/api-keys/{id}.
type RenameAPIKeyRequest struct {
	Name string `json:"name"`
}

// APIKeyResponse is the safe representation of an API key (never includes the secret hash).
type APIKeyResponse struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	KeyPrefix    string                  `json:"key_prefix"`
	Permission   schema.APIKeyPermission `json:"permission"`
	NetworkScope APIKeyNetworkScope      `json:"network_scope"`
	CreatedBy    string                  `json:"created_by"`
	CreatedAt    time.Time               `json:"created_at"`
	ExpiresAt    *time.Time              `json:"expires_at"`
	LastUsedAt   *time.Time              `json:"last_used_at,omitempty"`
	Revoked      bool                    `json:"revoked"`
}

// CreateAPIKeyResponse is returned only from the create endpoint and includes the one-time secret.
type CreateAPIKeyResponse struct {
	APIKeyResponse
	Key string `json:"key"`
}
