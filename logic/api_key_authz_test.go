package logic

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
)

func TestHasPermissionHierarchy(t *testing.T) {
	assert.True(t, HasPermission(schema.APIKeyPermissionRead, schema.APIKeyPermissionRead))
	assert.False(t, HasPermission(schema.APIKeyPermissionRead, schema.APIKeyPermissionModify))
	assert.False(t, HasPermission(schema.APIKeyPermissionRead, schema.APIKeyPermissionFullAccess))

	assert.True(t, HasPermission(schema.APIKeyPermissionModify, schema.APIKeyPermissionRead))
	assert.True(t, HasPermission(schema.APIKeyPermissionModify, schema.APIKeyPermissionModify))
	assert.False(t, HasPermission(schema.APIKeyPermissionModify, schema.APIKeyPermissionFullAccess))

	assert.True(t, HasPermission(schema.APIKeyPermissionFullAccess, schema.APIKeyPermissionRead))
	assert.True(t, HasPermission(schema.APIKeyPermissionFullAccess, schema.APIKeyPermissionModify))
	assert.True(t, HasPermission(schema.APIKeyPermissionFullAccess, schema.APIKeyPermissionFullAccess))
}

func TestNetworkInScope(t *testing.T) {
	all := &APIKeyAuthContext{Scope: models.APIKeyNetworkScope{Type: schema.APIKeyNetworkScopeAll}}
	assert.True(t, NetworkInScope(all, "any-net"))

	selected := &APIKeyAuthContext{Scope: models.APIKeyNetworkScope{
		Type:       schema.APIKeyNetworkScopeSelected,
		NetworkIDs: []string{"net-a", "net-b"},
	}}
	assert.True(t, NetworkInScope(selected, "net-a"))
	assert.True(t, NetworkInScope(selected, "net-b"))
	assert.False(t, NetworkInScope(selected, "net-c"))
	assert.False(t, NetworkInScope(nil, "net-a"))
}

func TestRequiredPermissionForRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/networks", nil)
	assert.Equal(t, schema.APIKeyPermissionRead, RequiredPermissionForRequest(r))

	r = httptest.NewRequest(http.MethodPost, "/api/networks", nil)
	assert.Equal(t, schema.APIKeyPermissionModify, RequiredPermissionForRequest(r))

	var captured schema.APIKeyPermission
	router := mux.NewRouter()
	router.HandleFunc("/api/networks/{networkname}", func(w http.ResponseWriter, r *http.Request) {
		captured = RequiredPermissionForRequest(r)
	}).Methods(http.MethodDelete)
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodDelete, "/api/networks/prod", nil))
	assert.Equal(t, schema.APIKeyPermissionFullAccess, captured)

	router = mux.NewRouter()
	router.HandleFunc("/api/v1/acls", func(w http.ResponseWriter, r *http.Request) {
		captured = RequiredPermissionForRequest(r)
	}).Methods(http.MethodDelete)
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodDelete, "/api/v1/acls", nil))
	assert.Equal(t, schema.APIKeyPermissionModify, captured)
}
