package controllers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func EventHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/network/activity", logic.SecurityCheck(true, http.HandlerFunc(listNetworkActivity))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/user/activity", logic.SecurityCheck(false, http.HandlerFunc(listUserActivity))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/activity", logic.SecurityCheck(true, http.HandlerFunc(listActivity))).Methods(http.MethodGet)
}

// @Summary     List network activity
// @Router      /api/v1/network/activity [get]
// @Tags        Activity
// @Security    oauth
// @Produce     json
// @Param       network_id query string true "Network ID required to get the network events"
// @Param       from_date query string false "Start date in RFC3339 format"
// @Param       to_date query string false "End date in RFC3339 format"
// @Param       page query int false "Page number"
// @Param       per_page query int false "Items per page"
// @Success     200 {array} schema.Event
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
	fromDateStr := r.URL.Query().Get("from_date")
	toDateStr := r.URL.Query().Get("to_date")
	var err error
	var fromDate, toDate time.Time
	if fromDateStr != "" && toDateStr != "" {
		fromDate, err = time.Parse(time.RFC3339, fromDateStr)
		if err != nil {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			})
			return
		}
		toDate, err = time.Parse(time.RFC3339, toDateStr)
		if err != nil {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			})
			return
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	ctx := db.WithContext(r.Context())
	netActivity, err := (&schema.Event{NetworkID: models.NetworkID(netID)}).ListByNetwork(db.SetPagination(ctx, page, pageSize), fromDate, toDate)
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, netActivity, "successfully fetched network activity")
}

// @Summary     List user activity
// @Router      /api/v1/user/activity [get]
// @Tags        Activity
// @Security    oauth
// @Produce     json
// @Param       username query string true "Username required to get the user events"
// @Param       from_date query string false "Start date in RFC3339 format"
// @Param       to_date query string false "End date in RFC3339 format"
// @Param       page query int false "Page number"
// @Param       per_page query int false "Items per page"
// @Success     200 {array} schema.Event
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
	caller, err := logic.GetUser(r.Header.Get("user"))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if caller.UserName != username && caller.PlatformRoleID != models.SuperAdminRole && caller.PlatformRoleID != models.AdminRole {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusForbidden,
			Message: "you are not authorized to view this user's activity",
		})
		return
	}
	fromDateStr := r.URL.Query().Get("from_date")
	toDateStr := r.URL.Query().Get("to_date")
	var fromDate, toDate time.Time
	if fromDateStr != "" && toDateStr != "" {
		fromDate, err = time.Parse(time.RFC3339, fromDateStr)
		if err != nil {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			})
			return
		}
		toDate, err = time.Parse(time.RFC3339, toDateStr)
		if err != nil {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			})
			return
		}
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	ctx := db.WithContext(r.Context())
	userActivity, err := (&schema.Event{TriggeredBy: username}).ListByUser(db.SetPagination(ctx, page, pageSize), fromDate, toDate)
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, userActivity, "successfully fetched user activity "+username)
}

// @Summary     List all activity
// @Router      /api/v1/activity [get]
// @Tags        Activity
// @Security    oauth
// @Produce     json
// @Param       username query string false "Filter by username"
// @Param       network_id query string false "Filter by network ID"
// @Param       from_date query string false "Start date in RFC3339 format"
// @Param       to_date query string false "End date in RFC3339 format"
// @Param       page query int false "Page number"
// @Param       per_page query int false "Items per page"
// @Success     200 {array} schema.Event
// @Failure     500 {object} models.ErrorResponse
func listActivity(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	network := r.URL.Query().Get("network_id")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	ctx := db.WithContext(r.Context())
	var err error
	fromDateStr := r.URL.Query().Get("from_date")
	toDateStr := r.URL.Query().Get("to_date")
	var fromDate, toDate time.Time
	if fromDateStr != "" && toDateStr != "" {
		fromDate, err = time.Parse(time.RFC3339, fromDateStr)
		if err != nil {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			})
			return
		}
		toDate, err = time.Parse(time.RFC3339, toDateStr)
		if err != nil {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			})
			return
		}
	}
	var events []schema.Event
	e := &schema.Event{TriggeredBy: username, NetworkID: models.NetworkID(network)}
	if username != "" && network != "" {
		events, err = e.ListByUserAndNetwork(db.SetPagination(ctx, page, pageSize), fromDate, toDate)
	} else if username != "" && network == "" {
		events, err = e.ListByUser(db.SetPagination(ctx, page, pageSize), fromDate, toDate)
	} else if username == "" && network != "" {
		events, err = e.ListByNetwork(db.SetPagination(ctx, page, pageSize), fromDate, toDate)
	} else {
		events, err = e.List(db.SetPagination(ctx, page, pageSize), fromDate, toDate)
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
