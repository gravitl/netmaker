package functions

import (
	"encoding/json"
	"net/http"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func ListOrganizations() []schema.Organization {
	resp := request[models.SuccessResponse](http.MethodGet, "/api/v1/orgs", nil)
	var orgs []schema.Organization
	data, _ := json.Marshal(resp.Response)
	_ = json.Unmarshal(data, &orgs)
	return orgs
}
