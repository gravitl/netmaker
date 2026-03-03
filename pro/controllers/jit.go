package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/pro/email"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
)

func JITHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/jit", logic.SecurityCheck(true,
		http.HandlerFunc(handleJIT))).Methods(http.MethodPost, http.MethodGet)

	r.HandleFunc("/api/v1/jit", logic.SecurityCheck(true,
		http.HandlerFunc(deleteJITGrant))).Methods(http.MethodDelete)

	r.HandleFunc("/api/v1/jit_user/networks", logic.SecurityCheck(false,
		http.HandlerFunc(getUserJITNetworks))).Methods(http.MethodGet)

	r.HandleFunc("/api/v1/jit_user/request", logic.SecurityCheck(false,
		http.HandlerFunc(requestJITAccess))).Methods(http.MethodPost)
}

// @Summary     List JIT requests for a network
// @Router      /api/v1/jit [get]
// @Tags        JIT
// @Security    oauth
// @Produce     json
// @Param       network query string true "Network ID"
// @Param       status query string false "Filter by status (pending, approved, denied, expired)"
// @Param       page query int false "Page number"
// @Param       per_page query int false "Items per page"
// @Success     200 {array} schema.JITRequest
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
//
// @Summary     Handle JIT operations (enable, disable, approve, deny)
// @Router      /api/v1/jit [post]
// @Tags        JIT
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       network query string true "Network ID"
// @Param       body body models.JITOperationRequest true "JIT operation request"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func handleJIT(w http.ResponseWriter, r *http.Request) {
	// Check if JIT feature is enabled
	featureFlags := logic.GetFeatureFlags()
	if !featureFlags.EnableJIT {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("JIT feature is not enabled"), "forbidden"))
		return
	}

	networkID := r.URL.Query().Get("network")
	if networkID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), "badrequest"))
		return
	}

	username := r.Header.Get("user")
	if username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user not found in request"), "unauthorized"))
		return
	}

	user, err := logic.GetUser(username)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "unauthorized"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		handleJITGet(w, r, networkID, user)
	case http.MethodPost:
		handleJITPost(w, r, networkID, user)
	default:
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("method not allowed"), "badrequest"))
	}
}

// handleJITGet - handles GET requests for JIT status/requests
func handleJITGet(w http.ResponseWriter, r *http.Request, networkID string, user *models.User) {
	statusFilter := r.URL.Query().Get("status") // "pending", "approved", "denied", "expired", or empty for all

	// Parse pagination parameters (default to 0, db.SetPagination will apply defaults)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	// Apply defaults if not provided (matching db.SetPagination logic)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	ctx := db.WithContext(r.Context())
	requests, total, err := proLogic.GetNetworkJITRequestsPaginated(ctx, networkID, statusFilter, page, pageSize)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// Calculate pagination metadata
	totalPages := (int(total) + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	response := map[string]interface{}{
		"data":        requests,
		"page":        page,
		"per_page":    pageSize,
		"total":       total,
		"total_pages": totalPages,
	}

	logic.ReturnSuccessResponseWithJson(w, r, response, "fetched JIT requests")
}

// handleJITPost - handles POST requests for JIT operations
func handleJITPost(w http.ResponseWriter, r *http.Request, networkID string, user *models.User) {
	var req models.JITOperationRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Log(0, "error decoding request body:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	switch req.Action {
	case "enable":
		handleEnableJIT(w, r, networkID, user)
	case "disable":
		handleDisableJIT(w, r, networkID, user)
	case "approve":
		handleApproveRequest(w, r, networkID, user, req.RequestID, req.ExpiresAt)
	case "deny":
		handleDenyRequest(w, r, networkID, user, req.RequestID)
	default:
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid action"), "badrequest"))
	}
}

// handleEnableJIT - enables JIT on a network
func handleEnableJIT(w http.ResponseWriter, r *http.Request, networkID string, user *models.User) {
	// Check if user is admin
	if !proLogic.IsNetworkAdmin(user, networkID) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("only network admins can enable JIT"), "forbidden"))
		return
	}

	if err := proLogic.EnableJITOnNetwork(networkID); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logic.LogEvent(&models.Event{
		Action: models.Update,
		Source: models.Subject{
			ID:   user.UserName,
			Name: user.UserName,
			Type: models.UserSub,
		},
		TriggeredBy: user.UserName,
		Target: models.Subject{
			ID:   networkID,
			Name: networkID,
			Type: models.NetworkSub,
		},
		NetworkID: models.NetworkID(networkID),
		Origin:    models.Dashboard,
	})

	logic.ReturnSuccessResponse(w, r, "JIT enabled on network")
}

// handleDisableJIT - disables JIT on a network
func handleDisableJIT(w http.ResponseWriter, r *http.Request, networkID string, user *models.User) {
	// Check if user is admin
	if !proLogic.IsNetworkAdmin(user, networkID) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("only network admins can disable JIT"), "forbidden"))
		return
	}

	if err := proLogic.DisableJITOnNetwork(networkID); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logic.LogEvent(&models.Event{
		Action: models.Update,
		Source: models.Subject{
			ID:   user.UserName,
			Name: user.UserName,
			Type: models.UserSub,
		},
		TriggeredBy: user.UserName,
		Target: models.Subject{
			ID:   networkID,
			Name: networkID,
			Type: models.NetworkSub,
		},
		NetworkID: models.NetworkID(networkID),
		Origin:    models.Dashboard,
	})

	logic.ReturnSuccessResponse(w, r, "JIT disabled on network")
}

// handleApproveRequest - approves a JIT request
func handleApproveRequest(w http.ResponseWriter, r *http.Request, networkID string, user *models.User, requestID string, expiresAtEpoch int64) {
	// Check if user is admin
	if !proLogic.IsNetworkAdmin(user, networkID) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("only network admins can approve requests"), "forbidden"))
		return
	}

	if requestID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("request_id is required"), "badrequest"))
		return
	}

	if expiresAtEpoch <= 0 {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("expires_at is required and must be a valid Unix epoch timestamp"), "badrequest"))
		return
	}

	// Convert epoch to time.Time
	expiresAt := time.Unix(expiresAtEpoch, 0).UTC()
	now := time.Now().UTC()

	// Validate that expires_at is in the future
	if expiresAt.Before(now) || expiresAt.Equal(now) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("expires_at must be in the future"), "badrequest"))
		return
	}

	grant, req, err := proLogic.ApproveJITRequest(requestID, expiresAt, user.UserName)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	// Send approval email to user
	go func() {
		network, _ := logic.GetNetwork(networkID)
		if err := email.SendJITApprovalEmail(grant, req, network); err != nil {
			slog.Error("failed to send approval notification", "error", err)
		}
	}()
	logic.LogEvent(&models.Event{
		Action: models.Update,
		Source: models.Subject{
			ID:   user.UserName,
			Name: user.UserName,
			Type: models.UserSub,
		},
		TriggeredBy: user.UserName,
		Target: models.Subject{
			ID:   requestID,
			Name: networkID,
			Type: models.NetworkSub,
		},
		NetworkID: models.NetworkID(networkID),
		Origin:    models.Dashboard,
	})

	logic.ReturnSuccessResponseWithJson(w, r, grant, "JIT request approved")
}

// handleDenyRequest - denies a JIT request
func handleDenyRequest(w http.ResponseWriter, r *http.Request, networkID string, user *models.User, requestID string) {
	// Check if user is admin
	if !proLogic.IsNetworkAdmin(user, networkID) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("only network admins can deny requests"), "forbidden"))
		return
	}

	if requestID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("request_id is required"), "badrequest"))
		return
	}

	request, err := proLogic.DenyJITRequest(requestID, user.UserName)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	// Send denial email to requester
	go func() {
		network, _ := logic.GetNetwork(networkID)
		if err := email.SendJITDeniedEmail(request, network); err != nil {
			slog.Error("failed to send JIT denied notification", "error", err)
		}
	}()

	logic.LogEvent(&models.Event{
		Action: models.Update,
		Source: models.Subject{
			ID:   user.UserName,
			Name: user.UserName,
			Type: models.UserSub,
		},
		TriggeredBy: user.UserName,
		Target: models.Subject{
			ID:   requestID,
			Name: networkID,
			Type: models.NetworkSub,
		},
		NetworkID: models.NetworkID(networkID),
		Origin:    models.Dashboard,
	})

	logic.ReturnSuccessResponse(w, r, "JIT request denied")
}

// @Summary     Delete/revoke a JIT grant
// @Router      /api/v1/jit [delete]
// @Tags        JIT
// @Security    oauth
// @Produce     json
// @Param       network query string true "Network ID"
// @Param       grant_id query string true "Grant ID to revoke"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func deleteJITGrant(w http.ResponseWriter, r *http.Request) {
	// Check if JIT feature is enabled
	featureFlags := logic.GetFeatureFlags()
	if !featureFlags.EnableJIT {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("JIT feature is not enabled"), "forbidden"))
		return
	}

	networkID := r.URL.Query().Get("network")
	grantID := r.URL.Query().Get("grant_id")

	if networkID == "" || grantID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network and grant_id are required"), "badrequest"))
		return
	}

	username := r.Header.Get("user")
	if username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user not found in request"), "unauthorized"))
		return
	}

	user, err := logic.GetUser(username)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "unauthorized"))
		return
	}

	// Check if user is admin
	if !proLogic.IsNetworkAdmin(user, networkID) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("only network admins can revoke grants"), "forbidden"))
		return
	}

	ctx := db.WithContext(r.Context())
	grant := schema.JITGrant{ID: grantID}
	if err := grant.Get(ctx); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if grant.NetworkID != networkID {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("grant does not belong to this network"), "badrequest"))
		return
	}

	// Delete all grants for this user on this network (in case there are multiple)
	if err := proLogic.DeactivateUserGrantsOnNetwork(networkID, grant.UserID); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// Update associated request status to "expired" for all approved requests from this user
	request := schema.JITRequest{
		NetworkID: networkID,
		UserID:    grant.UserID,
	}
	allRequests, err := request.ListByNetwork(ctx)
	var revokedRequest *schema.JITRequest
	if err == nil {
		for _, req := range allRequests {
			if req.UserID == grant.UserID && req.Status == "approved" {
				req.Status = "expired"
				req.RevokedAt = time.Now().UTC()
				if err := req.Update(ctx); err != nil {
					logger.Log(0, "failed to update request status when revoking grant:", err.Error())
					// Don't fail the operation, just log
				} else {
					// Use the first approved request for email notification
					if revokedRequest == nil {
						revokedRequest = &req
					}
				}
			}
		}
	}

	// Send email notification to user
	if revokedRequest != nil {
		network, err := logic.GetNetwork(networkID)
		if err == nil {
			if err := email.SendJITExpirationEmail(&grant, revokedRequest, network, true, user.UserName); err != nil {
				slog.Warn("failed to send revocation email", "grant_id", grantID, "user", revokedRequest.UserName, "error", err)
			}
		}
	}

	// Disconnect user's ext clients from the network
	if err := proLogic.DisconnectUserExtClientsFromNetwork(networkID, grant.UserID); err != nil {
		logger.Log(0, "failed to disconnect ext clients when revoking grant:", err.Error())
	}

	logic.LogEvent(&models.Event{
		Action: models.Delete,
		Source: models.Subject{
			ID:   user.UserName,
			Name: user.UserName,
			Type: models.UserSub,
		},
		TriggeredBy: user.UserName,
		Target: models.Subject{
			ID:   grantID,
			Name: networkID,
			Type: models.NetworkSub,
		},
		NetworkID: models.NetworkID(networkID),
		Origin:    models.Dashboard,
	})

	logic.ReturnSuccessResponse(w, r, "JIT grant revoked")
}

// @Summary     Get user JIT networks status
// @Router      /api/v1/jit_user/networks [get]
// @Tags        JIT
// @Security    oauth
// @Produce     json
// @Success     200 {array} models.UserJITNetworkStatus
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func getUserJITNetworks(w http.ResponseWriter, r *http.Request) {
	// Check if JIT feature is enabled
	featureFlags := logic.GetFeatureFlags()
	if !featureFlags.EnableJIT {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("JIT feature is not enabled"), "forbidden"))
		return
	}

	username := r.Header.Get("user")
	if username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user not found in request"), "unauthorized"))
		return
	}

	user, err := logic.GetUser(username)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "unauthorized"))
		return
	}

	// Get all networks user has access to
	allNetworks, err := logic.GetNetworks()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// Filter networks by user role
	userNetworks := logic.FilterNetworksByRole(allNetworks, *user)

	// Build response with JIT status for each network
	networksWithJITStatus, err := proLogic.GetUserJITNetworksStatus(userNetworks, user.UserName)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, networksWithJITStatus, "fetched user JIT network status")
}

// @Summary     Request JIT access to a network
// @Router      /api/v1/jit_user/request [post]
// @Tags        JIT
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       network query string true "Network ID"
// @Param       body body models.JITAccessRequest true "JIT access request"
// @Success     200 {object} schema.JITRequest
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func requestJITAccess(w http.ResponseWriter, r *http.Request) {
	// Check if JIT feature is enabled
	featureFlags := logic.GetFeatureFlags()
	if !featureFlags.EnableJIT {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("JIT feature is not enabled"), "forbidden"))
		return
	}

	username := r.Header.Get("user")
	if username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user not found in request"), "unauthorized"))
		return
	}
	network := r.URL.Query().Get("network")

	user, err := logic.GetUser(username)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "unauthorized"))
		return
	}

	var req models.JITAccessRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Log(0, "error decoding request body:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	req.NetworkID = network
	// Validate required fields
	if req.NetworkID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network_id is required"), "badrequest"))
		return
	}

	if req.Reason == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("reason is required"), "badrequest"))
		return
	}
	// Check if user has access to the network by role
	allNetworks, err := logic.GetNetworks()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// Filter networks by user role
	userNetworks := logic.FilterNetworksByRole(allNetworks, *user)
	hasAccess := false
	for _, network := range userNetworks {
		if network.NetID == req.NetworkID {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user does not have access to this network"), "forbidden"))
		return
	}

	// Create the JIT request
	request, err := proLogic.CreateJITRequest(req.NetworkID, user.UserName, req.Reason)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	// Send email notifications to network admins
	go func() {
		network, _ := logic.GetNetwork(req.NetworkID)
		if err := email.SendJITRequestEmails(request, network); err != nil {
			slog.Error("failed to send JIT request notifications", "error", err)
		}
	}()

	logic.LogEvent(&models.Event{
		Action: models.Create,
		Source: models.Subject{
			ID:   user.UserName,
			Name: user.UserName,
			Type: models.UserSub,
		},
		TriggeredBy: user.UserName,
		Target: models.Subject{
			ID:   request.ID,
			Name: req.NetworkID,
			Type: models.NetworkSub,
		},
		NetworkID: models.NetworkID(req.NetworkID),
		Origin:    models.ClientApp,
	})

	logic.ReturnSuccessResponseWithJson(w, r, request, "JIT access request created")
}
