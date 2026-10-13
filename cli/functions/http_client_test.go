package functions

import (
	"net/http"
	"testing"

	"github.com/gravitl/netmaker/cli/config"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
)

func TestApplyScopeHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com/api/networks", nil)
	assert.NoError(t, err)

	applyScopeHeaders(req, config.Context{})
	assert.Empty(t, req.Header.Get(scope.HeaderTenantID))
	assert.Empty(t, req.Header.Get(scope.HeaderOrgID))

	applyScopeHeaders(req, config.Context{
		TenantId:       "tenant-abc",
		OrganizationId: "org-xyz",
	})
	assert.Equal(t, "tenant-abc", req.Header.Get(scope.HeaderTenantID))
	assert.Equal(t, "org-xyz", req.Header.Get(scope.HeaderOrgID))
}
