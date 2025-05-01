package controllers

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func EventHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/activity", logic.SecurityCheck(true, http.HandlerFunc(listActivity))).Methods(http.MethodGet)
}

// @Summary     list activity.
// @Router      /api/v1/activity [get]
// @Tags        Activity
// @Param       network_id query string true "roleid required to get the role details"
// @Success     200 {object}  models.ReturnSuccessResponseWithJson
// @Failure     500 {object} models.ErrorResponse
func listActivity(w http.ResponseWriter, r *http.Request) {
	netID := r.URL.Query().Get("network_id")
	// Parse query parameters with defaults
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	ctx := db.WithContext(r.Context())
	netActivity, err := (&schema.Event{NetworkID: models.NetworkID(netID)}).List(db.SetPagination(ctx, page, pageSize))
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, netActivity, "successfully fetched network activity")
}
