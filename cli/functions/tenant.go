package functions

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gravitl/netmaker/cli/config"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
)

// ListTenants lists tenants for an organization.
// orgID is sent as X-Organization-ID; if empty, the current context's organization_id is used.
func ListTenants(orgID string) []schema.Tenant {
	if orgID == "" {
		_, ctx := config.GetCurrentContext()
		orgID = ctx.OrganizationId
	}
	if orgID == "" {
		log.Fatal("org_id is required: pass --org_id or set it on the context via `nmctl context set ... --org_id=`")
	}

	resp := requestWithHeaders[models.SuccessResponse](http.MethodGet, "/api/v1/tenants", nil, map[string]string{
		scope.HeaderOrgID: orgID,
	})
	var tenants []schema.Tenant
	data, _ := json.Marshal(resp.Response)
	_ = json.Unmarshal(data, &tenants)
	return tenants
}
