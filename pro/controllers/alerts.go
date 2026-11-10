package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/middleware"
	"github.com/gravitl/netmaker/models"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/gorm"
)

// AlertHandlers registers all HTTP routes for Alert Rules, Channels, and History.
func AlertHandlers(r *mux.Router) {
	// Alert Rules
	r.HandleFunc("/api/v1/alerts/rules", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(listAlertRules)))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/alerts/rules", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(createAlertRule)))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/alerts/rules/{id}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(getAlertRule)))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/alerts/rules/{id}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(updateAlertRule)))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/alerts/rules/{id}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(deleteAlertRule)))).Methods(http.MethodDelete)

	// Notification Channels
	r.HandleFunc("/api/v1/alerts/channels", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(listNotificationChannels)))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/alerts/channels", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(createNotificationChannel)))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/alerts/channels/{id}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(getNotificationChannel)))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/alerts/channels/{id}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(updateNotificationChannel)))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/alerts/channels/{id}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(deleteNotificationChannel)))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/alerts/channels/{id}/test", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(testNotificationChannel)))).Methods(http.MethodPost)

	// Alert History & Acknowledgment
	r.HandleFunc("/api/v1/alerts/history", middleware.Scope(scope.TenantScope, logic.SecurityCheck(false, http.HandlerFunc(listAlertHistory)))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/alerts/{id}/ack", middleware.Scope(scope.TenantScope, logic.SecurityCheck(false, http.HandlerFunc(acknowledgeAlert)))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/alerts/{id}/resolve", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(resolveAlertManual)))).Methods(http.MethodPost)
}

// ==================== Alert Rules Handlers ====================

func listAlertRules(w http.ResponseWriter, r *http.Request) {
	networkID := r.URL.Query().Get("network_id")
	ctx := db.WithContext(r.Context())

	rules, err := (&schema.AlertRule{}).ListByTenant(ctx, networkID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, rules, "successfully fetched alert rules")
}

func createAlertRule(w http.ResponseWriter, r *http.Request) {
	var rule schema.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid request payload: " + err.Error(),
		})
		return
	}

	if rule.Name == "" {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "name is required",
		})
		return
	}
	if rule.EventType == "" {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "event_type is required",
		})
		return
	}
	if rule.Severity == "" {
		rule.Severity = "warning"
	}
	if rule.CooldownMinutes <= 0 {
		rule.CooldownMinutes = 15
	}

	ctx := db.WithContext(r.Context())
	if err := rule.Create(ctx); err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, rule, "successfully created alert rule")
}

func getAlertRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := db.WithContext(r.Context())
	rule := &schema.AlertRule{ID: id}
	if err := rule.Get(ctx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusNotFound,
				Message: "alert rule not found",
			})
			return
		}
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, rule, "successfully fetched alert rule")
}

func updateAlertRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var updateReq schema.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid request payload: " + err.Error(),
		})
		return
	}

	ctx := db.WithContext(r.Context())
	existing := &schema.AlertRule{ID: id}
	if err := existing.Get(ctx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusNotFound,
				Message: "alert rule not found",
			})
			return
		}
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	updateReq.ID = id
	updateReq.UpdatedAt = time.Now().UTC()
	if err := updateReq.Update(ctx); err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	refetched := &schema.AlertRule{ID: id}
	_ = refetched.Get(ctx)
	logic.ReturnSuccessResponseWithJson(w, r, refetched, "successfully updated alert rule")
}

func deleteAlertRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := db.WithContext(r.Context())
	rule := &schema.AlertRule{ID: id}
	if err := rule.Delete(ctx); err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponse(w, r, "successfully deleted alert rule")
}

// ==================== Notification Channels Handlers ====================

func listNotificationChannels(w http.ResponseWriter, r *http.Request) {
	ctx := db.WithContext(r.Context())

	channels, err := (&schema.NotificationChannel{}).ListByTenant(ctx)
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, channels, "successfully fetched notification channels")
}

func createNotificationChannel(w http.ResponseWriter, r *http.Request) {
	var ch schema.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid request payload: " + err.Error(),
		})
		return
	}

	if ch.Name == "" {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "name is required",
		})
		return
	}
	switch ch.Type {
	case schema.ChannelEmail, schema.ChannelWebhook, schema.ChannelSlack, schema.ChannelTeams:
		// Valid type
	default:
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("unsupported channel type: %s (supported: email, webhook, slack, teams)", ch.Type),
		})
		return
	}

	ctx := db.WithContext(r.Context())
	if err := ch.Create(ctx); err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, ch, "successfully created notification channel")
}

func getNotificationChannel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := db.WithContext(r.Context())
	ch := &schema.NotificationChannel{ID: id}
	if err := ch.Get(ctx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusNotFound,
				Message: "notification channel not found",
			})
			return
		}
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, ch, "successfully fetched notification channel")
}

func updateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var updateReq schema.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid request payload: " + err.Error(),
		})
		return
	}

	ctx := db.WithContext(r.Context())
	existing := &schema.NotificationChannel{ID: id}
	if err := existing.Get(ctx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusNotFound,
				Message: "notification channel not found",
			})
			return
		}
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	updateReq.ID = id
	updateReq.UpdatedAt = time.Now().UTC()
	if err := updateReq.Update(ctx); err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	refetched := &schema.NotificationChannel{ID: id}
	_ = refetched.Get(ctx)
	logic.ReturnSuccessResponseWithJson(w, r, refetched, "successfully updated notification channel")
}

func deleteNotificationChannel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := db.WithContext(r.Context())
	ch := &schema.NotificationChannel{ID: id}
	if err := ch.Delete(ctx); err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponse(w, r, "successfully deleted notification channel")
}

func testNotificationChannel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := db.WithContext(r.Context())
	ch := &schema.NotificationChannel{ID: id}
	if err := ch.Get(ctx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusNotFound,
				Message: "notification channel not found",
			})
			return
		}
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	if err := proLogic.TestNotificationChannel(ctx, ch); err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("test notification failed: %v", err),
		})
		return
	}

	logic.ReturnSuccessResponse(w, r, "test notification sent successfully")
}

// ==================== Alert History Handlers ====================

func listAlertHistory(w http.ResponseWriter, r *http.Request) {
	status := schema.AlertStatus(r.URL.Query().Get("status"))
	networkID := r.URL.Query().Get("network_id")
	severity := r.URL.Query().Get("severity")
	fromDateStr := r.URL.Query().Get("from_date")
	toDateStr := r.URL.Query().Get("to_date")

	var fromDate, toDate time.Time
	var err error
	if fromDateStr != "" && toDateStr != "" {
		fromDate, err = time.Parse(time.RFC3339, fromDateStr)
		if err != nil {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusBadRequest,
				Message: "invalid from_date: " + err.Error(),
			})
			return
		}
		toDate, err = time.Parse(time.RFC3339, toDateStr)
		if err != nil {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusBadRequest,
				Message: "invalid to_date: " + err.Error(),
			})
			return
		}
	}

	ctx := db.WithContext(r.Context())
	records, err := (&schema.AlertRecord{}).List(ctx, status, networkID, severity, fromDate, toDate)
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, records, "successfully fetched alert history")
}

type ackRequest struct {
	AcknowledgedBy string `json:"acknowledged_by"`
}

func acknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var req ackRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	username := req.AcknowledgedBy
	if username == "" {
		tokenUsername, err := logic.GetUserNameFromToken(r.Context(), r.Header.Get("Authorization"))
		if err == nil && tokenUsername != "" {
			username = tokenUsername
		} else {
			username = "operator"
		}
	}

	ctx := db.WithContext(r.Context())
	alert := &schema.AlertRecord{ID: id}
	if err := alert.Get(ctx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusNotFound,
				Message: "alert not found",
			})
			return
		}
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	if err := alert.Acknowledge(ctx, username); err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	refetched := &schema.AlertRecord{ID: id}
	_ = refetched.Get(ctx)
	logic.ReturnSuccessResponseWithJson(w, r, refetched, "alert successfully acknowledged")
}

func resolveAlertManual(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	ctx := db.WithContext(r.Context())
	alert := &schema.AlertRecord{ID: id}
	if err := alert.Get(ctx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, models.ErrorResponse{
				Code:    http.StatusNotFound,
				Message: "alert not found",
			})
			return
		}
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	if err := alert.Resolve(ctx); err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	refetched := &schema.AlertRecord{ID: id}
	_ = refetched.Get(ctx)
	logic.ReturnSuccessResponseWithJson(w, r, refetched, "alert successfully resolved")
}

