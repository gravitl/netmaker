package controllers

import (
	"net/http"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/servercfg"
)

var limitedApis = map[string]struct{}{
	"/api/server/status":          {},
	"/api/server/onboarding":      {},
	"/api/emqx/hosts":             {},
	"/api/users/adm/authenticate": {},
}

func OnlyServerAPIWhenUnlicensedMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := servercfg.ErrLicenseValidation(request.Context()); err != nil {
			if _, ok := limitedApis[request.URL.Path]; !ok {
				logic.ReturnErrorResponse(writer, request, logic.FormatError(err, "forbidden"))
				return
			}
		}
		handler.ServeHTTP(writer, request)
	})
}
