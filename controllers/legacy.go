package controller

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
)

func legacyHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/legacy/nodes", logic.SecurityCheck(true, http.HandlerFunc(wipeLegacyNodes))).Methods(http.MethodDelete)
}

// swagger:route DELETE /api/v1/legacy/nodes nodes wipeLegacyNodes
//
// Delete all legacy nodes from DB.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: successResponse
func wipeLegacyNodes(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")
	if err := logic.RemoveAllLegacyNodes(); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		logger.Log(0, "error occurred when removing legacy nodes", err.Error())
	}
	logger.Log(0, r.Header.Get("user"), "wiped legacy nodes")
	logic.ReturnSuccessResponse(w, r, "wiped all legacy nodes")
}
