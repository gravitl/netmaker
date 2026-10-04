package logic

import (
	"context"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// permissionRank maps hierarchical API key permissions to comparable ranks.
func permissionRank(p schema.APIKeyPermission) int {
	switch p {
	case schema.APIKeyPermissionRead:
		return 1
	case schema.APIKeyPermissionModify:
		return 2
	case schema.APIKeyPermissionFullAccess:
		return 3
	default:
		return 0
	}
}

// HasPermission reports whether actual satisfies required under the hierarchy
// read < modify < full_access.
func HasPermission(actual, required schema.APIKeyPermission) bool {
	return permissionRank(actual) >= permissionRank(required)
}

// NetworkInScope reports whether networkID is listed in the key's selected scope (exact match only).
// Prefer NetworkInScopeCtx when a tenant-scoped DB lookup is available.
func NetworkInScope(auth *APIKeyAuthContext, networkID string) bool {
	if auth == nil || networkID == "" {
		return false
	}
	if auth.Scope.Type == schema.APIKeyNetworkScopeAll {
		return true
	}
	if auth.Scope.Type != schema.APIKeyNetworkScopeSelected {
		return false
	}
	for _, id := range auth.Scope.NetworkIDs {
		if id == networkID {
			return true
		}
	}
	return false
}

// resolveNetwork finds a network by UUID or name within the current tenant scope.
func resolveNetwork(ctx context.Context, networkID string) (*schema.Network, error) {
	if networkID == "" {
		return nil, errors.New("network id is required")
	}
	byID := &schema.Network{ID: networkID}
	if err := byID.Get(ctx); err == nil {
		return byID, nil
	}
	byName := &schema.Network{Name: networkID}
	if err := byName.Get(ctx); err != nil {
		return nil, err
	}
	return byName, nil
}

// NetworkInScopeCtx reports whether networkID (name or UUID) is within the API key scope.
func NetworkInScopeCtx(ctx context.Context, auth *APIKeyAuthContext, networkID string) bool {
	if auth == nil || networkID == "" {
		return false
	}
	if auth.Scope.Type == schema.APIKeyNetworkScopeAll {
		return true
	}
	if auth.Scope.Type != schema.APIKeyNetworkScopeSelected {
		return false
	}

	resolvedID := networkID
	resolvedName := networkID
	if net, err := resolveNetwork(ctx, networkID); err == nil {
		resolvedID = net.ID
		resolvedName = net.Name
	}

	for _, id := range auth.Scope.NetworkIDs {
		if id == networkID || id == resolvedID || id == resolvedName {
			return true
		}
	}
	return false
}

// AuthorizeAPIKey verifies tenant-bound API key access to a network at the required permission.
func AuthorizeAPIKey(ctx context.Context, networkID string, required schema.APIKeyPermission) error {
	auth := APIKeyFromContext(ctx)
	if auth == nil {
		return errors.New(Forbidden_Msg)
	}
	if networkID != "" {
		net, err := resolveNetwork(ctx, networkID)
		if err != nil {
			return errors.New(Forbidden_Msg)
		}
		if auth.TenantID != "" && net.TenantID != "" && net.TenantID != auth.TenantID {
			return errors.New(Forbidden_Msg)
		}
		if !NetworkInScopeCtx(ctx, auth, networkID) {
			return errors.New(Forbidden_Msg)
		}
	}
	if !HasPermission(auth.Permission, required) {
		return errors.New(Forbidden_Msg)
	}
	return nil
}

// FilterNetworksByAPIKey returns only networks visible to the API key.
func FilterNetworksByAPIKey(ctx context.Context, networks []schema.Network) []schema.Network {
	auth := APIKeyFromContext(ctx)
	if auth == nil {
		return []schema.Network{}
	}
	if auth.Scope.Type == schema.APIKeyNetworkScopeAll {
		return networks
	}
	allowed := make(map[string]struct{}, len(auth.Scope.NetworkIDs))
	for _, id := range auth.Scope.NetworkIDs {
		allowed[id] = struct{}{}
		if net, err := resolveNetwork(ctx, id); err == nil {
			allowed[net.ID] = struct{}{}
			allowed[net.Name] = struct{}{}
		}
	}
	filtered := make([]schema.Network, 0, len(networks))
	for _, n := range networks {
		if _, ok := allowed[n.ID]; ok {
			filtered = append(filtered, n)
			continue
		}
		if _, ok := allowed[n.Name]; ok {
			filtered = append(filtered, n)
		}
	}
	return filtered
}

// RequiredPermissionForRequest maps an HTTP request to the minimum API key permission.
func RequiredPermissionForRequest(r *http.Request) schema.APIKeyPermission {
	if r == nil {
		return schema.APIKeyPermissionFullAccess
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return schema.APIKeyPermissionRead
	}
	if r.Method == http.MethodDelete {
		if route, err := mux.CurrentRoute(r).GetPathTemplate(); err == nil && route == "/api/networks/{networkname}" {
			return schema.APIKeyPermissionFullAccess
		}
		return schema.APIKeyPermissionModify
	}
	return schema.APIKeyPermissionModify
}

func isAPIKeyAllowedResource(targetRsrc string) bool {
	switch targetRsrc {
	case schema.NetworkRsrc.String(),
		schema.HostRsrc.String(),
		schema.DnsRsrc.String(),
		schema.NameserverRsrc.String(),
		schema.AclRsrc.String(),
		schema.EgressGwRsrc.String(),
		schema.ExtClientsRsrc.String(),
		schema.GatewayRsrc.String(),
		schema.RelayRsrc.String(),
		schema.RemoteAccessGwRsrc.String(),
		schema.TagRsrc.String(),
		schema.MetricRsrc.String(),
		schema.FailOverRsrc.String(),
		schema.PostureCheckRsrc.String():
		return true
	default:
		return false
	}
}

// AuthorizeAPIKeyRequest enforces API key resource allow-list, network scope, and permission for an HTTP request.
func AuthorizeAPIKeyRequest(ctx context.Context, r *http.Request) error {
	auth := APIKeyFromContext(ctx)
	if auth == nil {
		return errors.New(Forbidden_Msg)
	}

	targetRsrc := r.Header.Get("TARGET_RSRC")
	if !isAPIKeyAllowedResource(targetRsrc) {
		return errors.New(Forbidden_Msg)
	}

	required := RequiredPermissionForRequest(r)
	netID := r.Header.Get("NET_ID")
	if netID == "" && targetRsrc == schema.NetworkRsrc.String() {
		netID = r.Header.Get("TARGET_RSRC_ID")
	}

	// Creating a network is only allowed for all-networks keys with modify+.
	if r.Method == http.MethodPost && targetRsrc == schema.NetworkRsrc.String() && netID == "" {
		if auth.Scope.Type != schema.APIKeyNetworkScopeAll {
			return errors.New(Forbidden_Msg)
		}
		if !HasPermission(auth.Permission, schema.APIKeyPermissionModify) {
			return errors.New(Forbidden_Msg)
		}
		return nil
	}

	// Collection / tenant-level network listing: permission only; filtering happens in handlers.
	if netID == "" {
		if !HasPermission(auth.Permission, required) {
			return errors.New(Forbidden_Msg)
		}
		return nil
	}

	return AuthorizeAPIKey(ctx, netID, required)
}

// APIKeyHasNetworkAccess is a convenience for device/network helpers.
func APIKeyHasNetworkAccess(ctx context.Context, networkID string, required schema.APIKeyPermission) bool {
	return AuthorizeAPIKey(ctx, networkID, required) == nil
}

// EnforceAPIKeyNetworkIfPresent is a no-op for user auth; for API keys it enforces scope + permission.
func EnforceAPIKeyNetworkIfPresent(ctx context.Context, networkID string, required schema.APIKeyPermission) error {
	if !IsAPIKeyAuth(ctx) {
		return nil
	}
	return AuthorizeAPIKey(ctx, networkID, required)
}

// EventSourceForRequest returns the audit event source subject for the current caller.
func EventSourceForRequest(ctx context.Context, username string) models.Subject {
	if auth := APIKeyFromContext(ctx); auth != nil {
		return models.Subject{
			ID:   auth.APIKeyID,
			Name: APIKeyActorName(auth.Name),
			Type: schema.APIKeySub,
		}
	}
	return models.Subject{
		ID:   username,
		Name: username,
		Type: schema.UserSub,
	}
}
