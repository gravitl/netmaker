package functions

import (
	"encoding/json"
	"net/http"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func ListTenants() []schema.Tenant {
	resp := request[models.SuccessResponse](http.MethodGet, "/api/v1/tenants", nil)
	var tenants []schema.Tenant
	data, _ := json.Marshal(resp.Response)
	_ = json.Unmarshal(data, &tenants)
	return tenants
}
