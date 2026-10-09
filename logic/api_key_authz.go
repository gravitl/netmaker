package logic

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
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

// isNetworkChildResource reports whether the resource inherits a parent network scope.
func isNetworkChildResource(targetRsrc string) bool {
	switch targetRsrc {
	case schema.HostRsrc.String(),
		schema.DnsRsrc.String(),
		schema.ExtClientsRsrc.String(),
		schema.EgressGwRsrc.String(),
		schema.GatewayRsrc.String(),
		schema.RelayRsrc.String(),
		schema.RemoteAccessGwRsrc.String(),
		schema.FailOverRsrc.String(),
		schema.PostureCheckRsrc.String():
		return true
	default:
		return false
	}
}

// resolveNetworksForResource loads the parent network ID(s) for a network-scoped child resource.
func resolveNetworksForResource(ctx context.Context, targetRsrc, targetRsrcID string) ([]string, error) {
	if targetRsrcID == "" {
		return nil, errors.New("resource id is required")
	}
	switch targetRsrc {
	case schema.HostRsrc.String():
		if _, err := uuid.Parse(targetRsrcID); err != nil {
			return nil, err
		}
		nets := GetHostNetworks(ctx, targetRsrcID)
		return nets, nil
	case schema.ExtClientsRsrc.String():
		client, err := GetExtClientByName(ctx, targetRsrcID)
		if err != nil {
			return nil, err
		}
		if client.Network == "" {
			return nil, errors.New("extclient has no network")
		}
		return []string{client.Network}, nil
	case schema.EgressGwRsrc.String():
		e := schema.Egress{ID: targetRsrcID}
		if err := e.Get(ctx); err != nil {
			return nil, err
		}
		if e.Network == "" {
			return nil, errors.New("egress has no network")
		}
		return []string{e.Network}, nil
	case schema.GatewayRsrc.String(),
		schema.RelayRsrc.String(),
		schema.RemoteAccessGwRsrc.String(),
		schema.FailOverRsrc.String():
		node, err := GetNodeByID(targetRsrcID)
		if err != nil {
			return nil, err
		}
		if node.Network == "" {
			return nil, errors.New("node has no network")
		}
		return []string{node.Network}, nil
	case schema.DnsRsrc.String():
		// DNS routes usually set NET_ID; when TARGET_RSRC_ID is present it is typically the network name.
		return []string{targetRsrcID}, nil
	case schema.PostureCheckRsrc.String():
		p := &schema.PostureCheck{ID: targetRsrcID}
		if err := p.Get(ctx); err != nil {
			return nil, err
		}
		if p.NetworkID == "" {
			return nil, errors.New("posture check has no network")
		}
		return []string{p.NetworkID.String()}, nil
	default:
		return nil, errors.New("unsupported resource type")
	}
}

// AuthorizeAPIKeyNetworks enforces required permission against every listed network.
func AuthorizeAPIKeyNetworks(ctx context.Context, networks []string, required schema.APIKeyPermission) error {
	if len(networks) == 0 {
		auth := APIKeyFromContext(ctx)
		if auth == nil {
			return errors.New(Forbidden_Msg)
		}
		// Host/device with no networks: only all-scope keys may proceed on permission alone.
		if auth.Scope.Type != schema.APIKeyNetworkScopeAll {
			return errors.New(Forbidden_Msg)
		}
		if !HasPermission(auth.Permission, required) {
			return errors.New(Forbidden_Msg)
		}
		return nil
	}
	for _, networkID := range networks {
		if err := AuthorizeAPIKey(ctx, networkID, required); err != nil {
			return err
		}
	}
	return nil
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
	targetRsrcID := r.Header.Get("TARGET_RSRC_ID")
	if netID == "" && targetRsrc == schema.NetworkRsrc.String() {
		netID = targetRsrcID
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

	if netID == "" {
		// Resolve parent network(s) for network-scoped child resources addressed by ID.
		if isNetworkChildResource(targetRsrc) && targetRsrcID != "" {
			networks, err := resolveNetworksForResource(ctx, targetRsrc, targetRsrcID)
			if err != nil {
				return errors.New(Forbidden_Msg)
			}
			return AuthorizeAPIKeyNetworks(ctx, networks, required)
		}

		// Collection / tenant-level ops without a network context:
		// permission only here; handlers filter GET results or enforce mutate per entry
		// (bulkDeleteHosts, updateAllKeys, syncHosts, getAllNodes, getAllExtClients, etc.).
		if !HasPermission(auth.Permission, required) {
			return errors.New(Forbidden_Msg)
		}
		return nil
	}

	return AuthorizeAPIKey(ctx, netID, required)
}

// HostInAPIKeyScope reports whether every network the host belongs to is within the API key scope.
func HostInAPIKeyScope(ctx context.Context, hostID string, required schema.APIKeyPermission) bool {
	if !IsAPIKeyAuth(ctx) {
		return true
	}
	if _, err := uuid.Parse(hostID); err != nil {
		return false
	}
	return AuthorizeAPIKeyNetworks(ctx, GetHostNetworks(ctx, hostID), required) == nil
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
