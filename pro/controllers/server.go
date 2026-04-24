package controllers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/pro/license"
)

func ServerHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/server/license/validation", logic.SecurityCheck(true, http.HandlerFunc(triggerLicenseValidation))).Methods(http.MethodPost)
}

func triggerLicenseValidation(w http.ResponseWriter, r *http.Request) {
	err := license.ValidateLicense()
	if err != nil {
		err = fmt.Errorf("error validating license: %v", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logic.ReturnSuccessResponse(w, r, "license validated successfully")
}
