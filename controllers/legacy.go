package controller

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
)

func legacyHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/legacy/nodes", logic.SecurityCheck(true, http.HandlerFunc(wipeLegacyNodes))).
		Methods(http.MethodDelete)
}

// @Summary     Delete all legacy nodes from DB.
// @Router      /api/v1/legacy/nodes [delete]
// @Tags        Nodes
// @Security    oauth2
// @Success     200 {string} string "Wiped all legacy nodes."
// @Failure     400 {object} models.ErrorResponse
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
