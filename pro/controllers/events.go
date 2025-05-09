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
	r.HandleFunc("/api/v1/network/activity", logic.SecurityCheck(true, http.HandlerFunc(listNetworkActivity))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/user/activity", logic.SecurityCheck(true, http.HandlerFunc(listUserActivity))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/activity", logic.SecurityCheck(true, http.HandlerFunc(listActivity))).Methods(http.MethodGet)
}

// @Summary     list activity.
// @Router      /api/v1/activity [get]
// @Tags        Activity
// @Param       network_id query string true "network_id required to get the network events"
// @Success     200 {object}  models.ReturnSuccessResponseWithJson
// @Failure     500 {object} models.ErrorResponse
func listNetworkActivity(w http.ResponseWriter, r *http.Request) {
	netID := r.URL.Query().Get("network_id")
	// Parse query parameters with defaults
	if netID == "" {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "network_id param is missing",
		})
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	ctx := db.WithContext(r.Context())
	netActivity, err := (&schema.Event{NetworkID: models.NetworkID(netID)}).ListByNetwork(db.SetPagination(ctx, page, pageSize))
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, netActivity, "successfully fetched network activity")
}

// @Summary     list activity.
// @Router      /api/v1/activity [get]
// @Tags        Activity
// @Param       network_id query string true "network_id required to get the network events"
// @Success     200 {object}  models.ReturnSuccessResponseWithJson
// @Failure     500 {object} models.ErrorResponse
func listUserActivity(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	// Parse query parameters with defaults
	if username == "" {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "username param is missing",
		})
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	ctx := db.WithContext(r.Context())
	userActivity, err := (&schema.Event{TriggeredBy: username}).ListByUser(db.SetPagination(ctx, page, pageSize))
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, userActivity, "successfully fetched user activity "+username)
}

// @Summary     list activity.
// @Router      /api/v1/activity [get]
// @Tags        Activity
// @Success     200 {object}  models.ReturnSuccessResponseWithJson
// @Failure     500 {object} models.ErrorResponse
func listActivity(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	network := r.URL.Query().Get("network_id")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	ctx := db.WithContext(r.Context())
	var err error
	var events []schema.Event
	e := &schema.Event{TriggeredBy: username, NetworkID: models.NetworkID(network)}
	if username != "" && network != "" {
		events, err = e.ListByUserAndNetwork(db.SetPagination(ctx, page, pageSize))
	} else if username != "" && network == "" {
		events, err = e.ListByUser(db.SetPagination(ctx, page, pageSize))
	} else if username == "" && network != "" {
		events, err = e.ListByNetwork(db.SetPagination(ctx, page, pageSize))
	} else {
		events, err = e.List(db.SetPagination(ctx, page, pageSize))
	}
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, events, "successfully fetched all events ")
}
