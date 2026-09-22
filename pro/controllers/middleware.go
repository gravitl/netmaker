package controllers

import (
	"context"
	"net/http"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
)

var limitedApis = map[string]struct{}{
	"/api/server/status":          {},
	"/api/server/onboarding":      {},
	"/api/emqx/hosts":             {},
	"/api/users/adm/authenticate": {},
	"/api/v1/auth/discover":       {},
}

func OnlyServerAPIWhenUnlicensedMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		tenantID := request.Header.Get(scope.HeaderTenantID)
		if tenantID == "" {
			tenantID = request.URL.Query().Get(scope.QueryTenantID)
		}

		ctx := db.WithContext(context.TODO())
		if tenantID == "" {
			tenant, err := logic.SoleTenant(ctx)
			if err == nil {
				tenantID = tenant.ID
			}
		}

		if tenantID != "" {
			ctx = scope.WithContext(ctx, scope.TenantScope, tenantID)
		}

		if err := servercfg.ErrLicenseValidation(ctx); err != nil {
			if _, ok := limitedApis[request.URL.Path]; !ok {
				logic.ReturnErrorResponse(writer, request, logic.FormatError(err, "forbidden"))
				return
			}
		}
		handler.ServeHTTP(writer, request)
	})
}
