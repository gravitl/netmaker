package ee_controllers

import (
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/servercfg"
	"net/http"
	"strings"
)

func OnlyServerAPIWhenUnlicensedMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if servercfg.ErrLicenseValidation != nil && !strings.HasPrefix(request.URL.Path, "/api/server") {
			logic.ReturnErrorResponse(writer, request, logic.FormatError(servercfg.ErrLicenseValidation, "unauthorized"))
			return
		}
		handler.ServeHTTP(writer, request)
	})
}
