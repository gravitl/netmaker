package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScopeAllowsAPIKeyWithoutTenantHeader(t *testing.T) {
	var sawRequest bool
	handler := Scope(scope.TenantScope, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest = true
		// No tenant bound yet — SecurityCheck will derive it from the key.
		assert.Empty(t, scope.ID(r.Context()))
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/hosts", nil)
	req.Header.Set("Authorization", "Bearer nm_live_abcdefghij1234567890abcdefghijklmnopqrstuv")
	require.True(t, logic.IsAPIKeySecret("nm_live_abcdefghij1234567890abcdefghijklmnopqrstuv"))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.True(t, sawRequest)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequestUsesAPIKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.False(t, requestUsesAPIKey(req))

	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.x")
	assert.False(t, requestUsesAPIKey(req))

	req.Header.Set("Authorization", "Bearer nm_live_abcdefghij1234567890abcdefghijklmnopqrstuv")
	assert.True(t, requestUsesAPIKey(req))
}
