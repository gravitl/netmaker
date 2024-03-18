package controllers

import (
	"net/http"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/servercfg"
)

var limitedApis = map[string]struct{}{
	"/api/server/status": {},
	"/api/emqx/hosts":    {},
}

func OnlyServerAPIWhenUnlicensedMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if servercfg.ErrLicenseValidation != nil {
			if _, ok := limitedApis[request.URL.Path]; !ok {
				logic.ReturnErrorResponse(writer, request, logic.FormatError(servercfg.ErrLicenseValidation, "forbidden"))
				return
			}
		}
		handler.ServeHTTP(writer, request)
	})
}
