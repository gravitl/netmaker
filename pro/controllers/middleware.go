package controllers

import (
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/servercfg"
	"net/http"
)

func OnlyServerAPIWhenUnlicensedMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if servercfg.ErrLicenseValidation != nil && request.URL.Path != "/api/server/status" {
			logic.ReturnErrorResponse(writer, request, logic.FormatError(servercfg.ErrLicenseValidation, "forbidden"))
			return
		}
		handler.ServeHTTP(writer, request)
	})
}
