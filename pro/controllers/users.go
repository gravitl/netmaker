package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	proAuth "github.com/gravitl/netmaker/pro/auth"
	"github.com/gravitl/netmaker/pro/email"
	"github.com/gravitl/netmaker/pro/idp"
	"github.com/gravitl/netmaker/pro/idp/azure"
	"github.com/gravitl/netmaker/pro/idp/google"
	"github.com/gravitl/netmaker/pro/idp/okta"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/utils"
	"golang.org/x/exp/slog"
	"gorm.io/datatypes"
)

func UserHandlers(r *mux.Router) {

	r.HandleFunc("/api/oauth/login", proAuth.HandleAuthLogin).Methods(http.MethodGet)
	r.HandleFunc("/api/oauth/callback", proAuth.HandleAuthCallback).Methods(http.MethodGet)
	r.HandleFunc("/api/oauth/headless", proAuth.HandleHeadlessSSO)
	r.HandleFunc("/api/oauth/register/{regKey}", proAuth.RegisterHostSSO).Methods(http.MethodGet)

	// User Role Handlers
	r.HandleFunc("/api/v1/users/role", logic.SecurityCheck(true, http.HandlerFunc(getRole))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/role", logic.SecurityCheck(true, http.HandlerFunc(createRole))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/users/role", logic.SecurityCheck(true, http.HandlerFunc(updateRole))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/users/role", logic.SecurityCheck(true, http.HandlerFunc(deleteRole))).Methods(http.MethodDelete)

	// User Group Handlers
	r.HandleFunc("/api/v1/users/groups", logic.SecurityCheck(true, http.HandlerFunc(getUserGroups))).Methods(http.MethodGet)
	r.HandleFunc("/api/v2/users/groups", logic.SecurityCheck(true, http.HandlerFunc(listUserGroups))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/group", logic.SecurityCheck(true, http.HandlerFunc(getUserGroup))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/group", logic.SecurityCheck(true, http.HandlerFunc(createUserGroup))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/users/group", logic.SecurityCheck(true, http.HandlerFunc(updateUserGroup))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/users/group", logic.SecurityCheck(true, http.HandlerFunc(deleteUserGroup))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/users/add_network_user", logic.SecurityCheck(true, http.HandlerFunc(addUsertoNetwork))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/users/remove_network_user", logic.SecurityCheck(true, http.HandlerFunc(removeUserfromNetwork))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/users/unassigned_network_users", logic.SecurityCheck(true, http.HandlerFunc(listUnAssignedNetUsers))).Methods(http.MethodGet)

	// User Invite Handlers
	r.HandleFunc("/api/v1/users/invite", userInviteVerify).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/invite-signup", userInviteSignUp).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/users/invite", logic.SecurityCheck(true, http.HandlerFunc(inviteUsers))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/users/invites", logic.SecurityCheck(true, http.HandlerFunc(listUserInvites))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/invite", logic.SecurityCheck(true, http.HandlerFunc(deleteUserInvite))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/users/invites", logic.SecurityCheck(true, http.HandlerFunc(deleteAllUserInvites))).Methods(http.MethodDelete)

	r.HandleFunc("/api/users_pending", logic.SecurityCheck(true, http.HandlerFunc(getPendingUsers))).Methods(http.MethodGet)
	r.HandleFunc("/api/users_pending", logic.SecurityCheck(true, http.HandlerFunc(deleteAllPendingUsers))).Methods(http.MethodDelete)
	r.HandleFunc("/api/users_pending/user/{username}", logic.SecurityCheck(true, http.HandlerFunc(deletePendingUser))).Methods(http.MethodDelete)
	r.HandleFunc("/api/users_pending/user/{username}", logic.SecurityCheck(true, http.HandlerFunc(approvePendingUser))).Methods(http.MethodPost)

	r.HandleFunc("/api/users/{username}/remote_access_gw/{remote_access_gateway_id}", logic.SecurityCheck(true, http.HandlerFunc(attachUserToRemoteAccessGw))).Methods(http.MethodPost)
	r.HandleFunc("/api/users/{username}/remote_access_gw/{remote_access_gateway_id}", logic.SecurityCheck(true, http.HandlerFunc(removeUserFromRemoteAccessGW))).Methods(http.MethodDelete)
	r.HandleFunc("/api/users/{username}/remote_access_gw", logic.SecurityCheck(false, logic.ContinueIfUserMatch(http.HandlerFunc(getUserRemoteAccessGwsV1)))).Methods(http.MethodGet)
	r.HandleFunc("/api/users/ingress/{ingress_id}", logic.SecurityCheck(true, http.HandlerFunc(ingressGatewayUsers))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/network_ip", logic.SecurityCheck(true, http.HandlerFunc(userNetworkMapping))).Methods(http.MethodGet)

	r.HandleFunc("/api/idp/sync", logic.SecurityCheck(true, http.HandlerFunc(syncIDP))).Methods(http.MethodPost)
	r.HandleFunc("/api/idp/sync/test", logic.SecurityCheck(true, http.HandlerFunc(testIDPSync))).Methods(http.MethodPost)
	r.HandleFunc("/api/idp/sync/status", logic.SecurityCheck(true, http.HandlerFunc(getIDPSyncStatus))).Methods(http.MethodGet)
	r.HandleFunc("/api/idp", logic.SecurityCheck(true, http.HandlerFunc(removeIDPIntegration))).Methods(http.MethodDelete)
}

// @Summary     User signup via invite
// @Router      /api/v1/users/invite-signup [post]
// @Tags        Users
// @Accept      json
// @Produce     json
// @Param       email query string true "Invitee email"
// @Param       invite_code query string true "Invite code"
// @Param       body body models.User true "User signup data"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
func userInviteSignUp(w http.ResponseWriter, r *http.Request) {
	emailID := r.URL.Query().Get("email")
	code := r.URL.Query().Get("invite_code")
	in, err := logic.GetUserInvite(emailID)
	if err != nil {
		logger.Log(0, "failed to fetch users: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if code != in.InviteCode {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid invite code"), "badrequest"))
		return
	}
	// check if user already exists
	userCheck := &schema.User{Username: emailID}
	err = userCheck.Get(r.Context())
	if err == nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user already exists"), "badrequest"))
		return
	}
	var user schema.User
	err = json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		logger.Log(0, user.Username, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if user.Username != emailID {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("username not matching with invite"), "badrequest"))
		return
	}
	if user.Password == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("password cannot be empty"), "badrequest"))
		return
	}

	user.UserGroups = datatypes.NewJSONType(in.UserGroups)
	user.PlatformRoleID = schema.UserRoleID(in.PlatformRoleID)
	if user.PlatformRoleID == "" {
		user.PlatformRoleID = schema.ServiceUser
	}
	err = logic.CreateUser(&user)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	// delete invite
	logic.DeleteUserInvite(emailID)
	logic.DeletePendingUser(emailID)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	logic.ReturnSuccessResponse(w, r, "created user successfully "+emailID)
}

// @Summary     Verify user invite
// @Router      /api/v1/users/invite [get]
// @Tags        Users
// @Produce     json
// @Param       email query string true "Invitee email"
// @Param       invite_code query string true "Invite code"
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func userInviteVerify(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	code := r.URL.Query().Get("invite_code")
	err := logic.ValidateAndApproveUserInvite(email, code)
	if err != nil {
		logger.Log(0, "failed to fetch users: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponse(w, r, "invite is valid")
}

// @Summary     Invite users
// @Router      /api/v1/users/invite [post]
// @Tags        Users
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body models.InviteUsersReq true "Invite users request"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
func inviteUsers(w http.ResponseWriter, r *http.Request) {
	var inviteReq models.InviteUsersReq
	err := json.NewDecoder(r.Body).Decode(&inviteReq)
	if err != nil {
		slog.Error("error decoding request body", "error",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	callerUserName := r.Header.Get("user")
	if r.Header.Get("ismaster") != "yes" {
		caller := &schema.User{Username: callerUserName}
		err = caller.Get(r.Context())
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "notfound"))
			return
		}
		if inviteReq.PlatformRoleID == schema.SuperAdminRole.String() {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("super admin cannot be invited"), "badrequest"))
			return
		}
		if inviteReq.PlatformRoleID == "" {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("platform role id cannot be empty"), "badrequest"))
			return
		}
		if (inviteReq.PlatformRoleID == schema.AdminRole.String() ||
			inviteReq.PlatformRoleID == schema.SuperAdminRole.String()) && caller.PlatformRoleID != schema.SuperAdminRole {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("only superadmin can invite admin users"), "forbidden"))
			return
		}
	}

	//validate Req
	err = proLogic.IsGroupsValid(inviteReq.UserGroups)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = proLogic.IsNetworkRolesValid(inviteReq.NetworkRoles)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	// check platform role
	roleCheck := &schema.UserRole{ID: schema.UserRoleID(inviteReq.PlatformRoleID)}
	err = roleCheck.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	for _, inviteeEmail := range inviteReq.UserEmails {
		inviteeEmail = strings.ToLower(inviteeEmail)
		// check if user with email exists, then ignore
		if !email.IsValid(inviteeEmail) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid email "+inviteeEmail), "badrequest"))
			return
		}
		inviteeCheck := &schema.User{Username: inviteeEmail}
		err = inviteeCheck.Get(r.Context())
		if err == nil {
			// user exists already, so ignore
			continue
		}
		invite := models.UserInvite{
			Email:          inviteeEmail,
			PlatformRoleID: inviteReq.PlatformRoleID,
			UserGroups:     inviteReq.UserGroups,
			NetworkRoles:   inviteReq.NetworkRoles,
			InviteCode:     logic.RandomString(8),
		}
		frontendURL := strings.TrimSuffix(servercfg.GetFrontendURL(), "/")
		if frontendURL == "" {
			frontendURL = fmt.Sprintf("https://dashboard.%s", servercfg.GetNmBaseDomain())
		}
		u, err := url.Parse(fmt.Sprintf("%s/invite?email=%s&invite_code=%s",
			frontendURL, url.QueryEscape(invite.Email), url.QueryEscape(invite.InviteCode)))
		if err != nil {
			slog.Error("failed to parse to invite url", "error", err)
			return
		}
		if servercfg.DeployedByOperator() {
			u, err = url.Parse(fmt.Sprintf("%s/invite?tenant_id=%s&email=%s&invite_code=%s",
				proLogic.GetAccountsUIHost(), url.QueryEscape(servercfg.GetNetmakerTenantID()), url.QueryEscape(invite.Email), url.QueryEscape(invite.InviteCode)))
			if err != nil {
				slog.Error("failed to parse to invite url", "error", err)
				return
			}
		}
		invite.InviteURL = u.String()
		err = logic.InsertUserInvite(invite)
		if err != nil {
			slog.Error("failed to insert invite for user", "email", invite.Email, "error", err)
		}
		logic.LogEvent(&models.Event{
			Action: schema.Create,
			Source: models.Subject{
				ID:   callerUserName,
				Name: callerUserName,
				Type: schema.UserSub,
				Info: invite,
			},
			TriggeredBy: callerUserName,
			Target: models.Subject{
				ID:   inviteeEmail,
				Name: inviteeEmail,
				Type: schema.UserInviteSub,
			},
			Origin: schema.Dashboard,
		})
		// notify user with magic link
		go func(invite models.UserInvite) {
			// Set E-Mail body. You can set plain text or html with text/html

			e := email.UserInvitedMail{
				BodyBuilder:    &email.EmailBodyBuilderWithH1HeadlineAndImage{},
				InviteURL:      invite.InviteURL,
				PlatformRoleID: invite.PlatformRoleID,
			}
			n := email.Notification{
				RecipientMail: invite.Email,
			}
			err = email.GetClient().SendEmail(context.Background(), n, e)
			if err != nil {
				slog.Error("failed to send email invite", "user", invite.Email, "error", err)
			}
		}(invite)
	}

	logic.ReturnSuccessResponse(w, r, "triggered user invites")
}

// @Summary     List all pending user invites
// @Router      /api/v1/users/invites [get]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Success     200 {array} models.UserInvite
// @Failure     500 {object} models.ErrorResponse
func listUserInvites(w http.ResponseWriter, r *http.Request) {
	usersInvites, err := logic.ListUserInvites()
	if err != nil {
		logger.Log(0, "failed to fetch users: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, usersInvites, "fetched pending user invites")
}

// @Summary     Delete a pending user invite
// @Router      /api/v1/users/invite [delete]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Param       invitee_email query string true "Invitee email to delete"
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func deleteUserInvite(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("invitee_email")
	err := logic.DeleteUserInvite(email)
	if err != nil {
		logger.Log(0, "failed to delete user invite: ", email, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: schema.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   email,
			Name: email,
			Type: schema.UserInviteSub,
		},
		Origin: schema.Dashboard,
		Diff: models.Diff{
			Old: models.UserInvite{
				Email: email,
			},
			New: nil,
		},
	})
	logic.ReturnSuccessResponse(w, r, "deleted user invite")
}

// @Summary     Delete all pending user invites
// @Router      /api/v1/users/invites [delete]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func deleteAllUserInvites(w http.ResponseWriter, r *http.Request) {
	err := database.DeleteAllRecords(database.USER_INVITES_TABLE_NAME)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to delete all pending user invites "+err.Error()), "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: schema.DeleteAll,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   "All Invites",
			Name: "All Invites",
			Type: schema.UserInviteSub,
		},
		Origin: schema.Dashboard,
	})
	logic.ReturnSuccessResponse(w, r, "cleared all pending user invites")
}

// @Summary     List all user groups
// @Router      /api/v1/users/groups [get]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Success     200 {array} models.UserGroup
// @Failure     500 {object} models.ErrorResponse
func getUserGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := (&schema.UserGroup{}).ListAll(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, groups, "successfully fetched user groups")
}

// @Summary     List all user groups
// @Router      /api/v2/users/groups [get]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Param       default query string false "Filter Default / Custom Groups" Enums(true, false)
// @Param       page query int false "Page number"
// @Param       per_page query int false "Items per page"
// @Success     200 {array} models.UserGroup
// @Failure     500 {object} models.ErrorResponse
func listUserGroups(w http.ResponseWriter, r *http.Request) {
	var defaultGroups []interface{}
	for _, filter := range r.URL.Query()["default"] {
		var value bool
		if filter == "true" {
			value = true
		}

		if filter == "false" {
			value = false
		}

		defaultGroups = append(defaultGroups, value)
	}

	var page, pageSize int
	if r.URL.Query().Has("page") {
		page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	} else {
		page = 1
	}

	if r.URL.Query().Has("per_page") {
		pageSize, _ = strconv.Atoi(r.URL.Query().Get("per_page"))
	} else {
		pageSize = 10
	}

	groups, err := (&schema.UserGroup{}).ListAll(
		r.Context(),
		dbtypes.WithFilter("default", defaultGroups...),
		dbtypes.InAscOrder("name"),
		dbtypes.WithPagination(page, pageSize),
	)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	total, err := (&schema.UserGroup{}).Count(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	response := models.PaginatedResponse{
		Data:       groups,
		Page:       page,
		PerPage:    pageSize,
		Total:      total,
		TotalPages: totalPages,
	}

	logic.ReturnSuccessResponseWithJson(w, r, response, "successfully fetched user groups")
}

// @Summary     Get a user group
// @Router      /api/v1/users/group [get]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Param       group_id query string true "Group ID"
// @Success     200 {object} models.UserGroup
// @Failure     500 {object} models.ErrorResponse
func getUserGroup(w http.ResponseWriter, r *http.Request) {

	gid := r.URL.Query().Get("group_id")
	if gid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("group id is required"), "badrequest"))
		return
	}
	group, err := proLogic.GetUserGroup(schema.UserGroupID(gid))
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, group, "successfully fetched user group")
}

// @Summary     Create a user group
// @Router      /api/v1/users/group [post]
// @Tags        Users
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body models.CreateGroupReq true "Create group request"
// @Success     200 {object} models.UserGroup
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func createUserGroup(w http.ResponseWriter, r *http.Request) {
	type CreateGroupReq struct {
		Group   schema.UserGroup `json:"user_group"`
		Members []string         `json:"members"`
	}

	var userGroupReq CreateGroupReq
	err := json.NewDecoder(r.Body).Decode(&userGroupReq)
	if err != nil {
		slog.Error("error decoding request body", "error",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = proLogic.ValidateCreateGroupReq(userGroupReq.Group)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = proLogic.CreateUserGroup(&userGroupReq.Group)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	for _, userID := range userGroupReq.Members {
		user := &schema.User{Username: userID}
		err = user.Get(r.Context())
		if err != nil {
			continue
		}
		if len(user.UserGroups.Data()) == 0 {
			user.UserGroups = datatypes.NewJSONType(make(map[schema.UserGroupID]struct{}))
		}
		user.UserGroups.Data()[userGroupReq.Group.ID] = struct{}{}
		logic.UpsertUser(*user)
	}
	logic.LogEvent(&models.Event{
		Action: schema.Create,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   userGroupReq.Group.ID.String(),
			Name: userGroupReq.Group.Name,
			Type: schema.UserGroupSub,
		},
		Origin: schema.Dashboard,
	})
	go mq.PublishPeerUpdate(false)
	logic.ReturnSuccessResponseWithJson(w, r, userGroupReq.Group, "created user group")
}

// @Summary     Update a user group
// @Router      /api/v1/users/group [put]
// @Tags        Users
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body models.UserGroup true "User group update data"
// @Success     200 {object} models.UserGroup
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func updateUserGroup(w http.ResponseWriter, r *http.Request) {
	var userGroup schema.UserGroup
	err := json.NewDecoder(r.Body).Decode(&userGroup)
	if err != nil {
		slog.Error("error decoding request body", "error",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	// fetch curr group
	currUserG, err := proLogic.GetUserGroup(userGroup.ID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if currUserG.Default {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("cannot update default user group"), "badrequest"))
		return
	}
	err = proLogic.ValidateUpdateGroupReq(userGroup)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	userGroup.ExternalIdentityProviderID = currUserG.ExternalIdentityProviderID

	err = proLogic.UpdateUserGroup(userGroup)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: schema.Update,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   userGroup.ID.String(),
			Name: userGroup.Name,
			Type: schema.UserGroupSub,
		},
		Diff: models.Diff{
			Old: currUserG,
			New: userGroup,
		},
		Origin: schema.Dashboard,
	})
	replacePeers := false
	go func() {
		currAllNetworksRole, currAllNetworksRoleExists := currUserG.NetworkRoles.Data()[schema.AllNetworks]
		newAllNetworksRole, newAllNetworksRoleExists := userGroup.NetworkRoles.Data()[schema.AllNetworks]

		var removeAllNetworksCurrRoleAcls bool
		var addAllNetworksNewRoleAcls bool
		var updateSpecifiedNetworksAcls bool
		if currAllNetworksRoleExists {
			if newAllNetworksRoleExists {
				if !reflect.DeepEqual(currAllNetworksRole, newAllNetworksRole) {
					removeAllNetworksCurrRoleAcls = true
					addAllNetworksNewRoleAcls = true
				}
			} else {
				removeAllNetworksCurrRoleAcls = true
			}
		} else {
			if newAllNetworksRoleExists {
				addAllNetworksNewRoleAcls = true
			} else {
				updateSpecifiedNetworksAcls = true
			}
		}

		networksAdded := make([]schema.NetworkID, 0)
		networksRemoved := make([]schema.NetworkID, 0)

		for networkID := range userGroup.NetworkRoles.Data() {
			if networkID == schema.AllNetworks {
				continue
			}

			if _, ok := currUserG.NetworkRoles.Data()[networkID]; !ok {
				networksAdded = append(networksAdded, networkID)
			}
		}

		for networkID := range currUserG.NetworkRoles.Data() {
			if networkID == schema.AllNetworks {
				continue
			}

			if _, ok := userGroup.NetworkRoles.Data()[networkID]; !ok {
				networksRemoved = append(networksRemoved, networkID)
			}
		}

		if removeAllNetworksCurrRoleAcls || addAllNetworksNewRoleAcls {
			const globalNetworkAdmin = "global-network-admin"
			networks, _ := (&schema.Network{}).ListAll(r.Context())
			for _, network := range networks {
				if removeAllNetworksCurrRoleAcls {
					currRole := schema.NetworkUser
					_, ok := currAllNetworksRole[globalNetworkAdmin]
					if ok {
						currRole = schema.NetworkAdmin
					}

					aclID := fmt.Sprintf("%s.%s-grp", network.Name, currRole)
					acl, err := logic.GetAcl(aclID)
					if err == nil {
						var hasGroupSrc bool
						newAclSrc := make([]models.AclPolicyTag, 0)
						for _, src := range acl.Src {
							if src.ID == models.UserGroupAclID && src.Value == userGroup.ID.String() {
								hasGroupSrc = true
							} else {
								newAclSrc = append(newAclSrc, src)
							}
						}

						if hasGroupSrc {
							acl.Src = newAclSrc
							_ = logic.UpsertAcl(acl)
						}
					}
				}

				if addAllNetworksNewRoleAcls {
					newRole := schema.NetworkUser
					_, ok := newAllNetworksRole[globalNetworkAdmin]
					if ok {
						newRole = schema.NetworkAdmin
					}

					aclID := fmt.Sprintf("%s.%s-grp", network.Name, newRole)
					acl, err := logic.GetAcl(aclID)
					if err == nil {
						var hasGroupSrc bool
						for _, src := range acl.Src {
							if src.ID == models.UserGroupAclID && src.Value == userGroup.ID.String() {
								hasGroupSrc = true
							}
						}

						if !hasGroupSrc {
							acl.Src = append(acl.Src, models.AclPolicyTag{
								ID:    models.UserGroupAclID,
								Value: userGroup.ID.String(),
							})
							_ = logic.UpsertAcl(acl)
						}
					}
				}
			}
		}

		if updateSpecifiedNetworksAcls {
			for _, networkID := range networksAdded {
				// ensure the network exists.
				network := &schema.Network{Name: networkID.String()}
				err := network.Get(r.Context())
				if err != nil {
					continue
				}

				// insert acl if the network is added to the group.
				acl := models.Acl{
					ID:          uuid.New().String(),
					Name:        fmt.Sprintf("%s group", userGroup.Name),
					MetaData:    "This Policy allows user group to communicate with all gateways",
					Default:     false,
					ServiceType: models.Any,
					NetworkID:   schema.NetworkID(network.Name),
					Proto:       models.ALL,
					RuleType:    models.UserPolicy,
					Src: []models.AclPolicyTag{
						{
							ID:    models.UserGroupAclID,
							Value: userGroup.ID.String(),
						},
					},
					Dst: []models.AclPolicyTag{
						{
							ID:    models.NodeTagID,
							Value: fmt.Sprintf("%s.%s", schema.NetworkID(network.Name), models.GwTagName),
						}},
					AllowedDirection: models.TrafficDirectionUni,
					Enabled:          true,
					CreatedBy:        "auto",
					CreatedAt:        time.Now().UTC(),
				}
				_ = logic.InsertAcl(acl)
				replacePeers = true
			}

			// since this group doesn't have a role for this network,
			// there is no point in having this group as src in any
			// of the network's acls.
			for _, networkID := range networksRemoved {
				acls, err := logic.ListAclsByNetwork(networkID)
				if err != nil {
					continue
				}

				for _, acl := range acls {
					var hasGroupSrc bool
					newAclSrc := make([]models.AclPolicyTag, 0)
					for _, src := range acl.Src {
						if src.ID == models.UserGroupAclID && src.Value == userGroup.ID.String() {
							hasGroupSrc = true
						} else {
							newAclSrc = append(newAclSrc, src)
						}
					}

					if hasGroupSrc {
						if len(newAclSrc) == 0 {
							// no other src exists, delete acl.
							_ = logic.DeleteAcl(acl)
						} else {
							// other sources exist, update acl.
							acl.Src = newAclSrc
							_ = logic.UpsertAcl(acl)
						}
						replacePeers = true
					}
				}
			}
		}
	}()

	// reset configs for service user
	go proLogic.UpdatesUserGwAccessOnGrpUpdates(userGroup.ID, currUserG.NetworkRoles.Data(), userGroup.NetworkRoles.Data())
	go mq.PublishPeerUpdate(replacePeers)
	logic.ReturnSuccessResponseWithJson(w, r, userGroup, "updated user group")
}

// @Summary     List unassigned network users
// @Router      /api/v1/users/unassigned_network_users [get]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Param       network_id query string true "Network ID"
// @Success     200 {array} models.ReturnUser
// @Failure     400 {object} models.ErrorResponse
func listUnAssignedNetUsers(w http.ResponseWriter, r *http.Request) {
	netID := r.URL.Query().Get("network_id")
	if netID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), logic.BadReq))
		return
	}
	var unassignedUsers []models.ReturnUser
	users, _ := logic.GetUsers()
	for _, user := range users {
		if user.PlatformRoleID != schema.ServiceUser {
			continue
		}
		skipUser := false
		for userGID := range user.UserGroups {
			userG, err := proLogic.GetUserGroup(userGID)
			if err != nil {
				continue
			}
			if _, ok := userG.NetworkRoles.Data()[schema.NetworkID(netID)]; ok {
				skipUser = true
				break
			}
		}
		if skipUser {
			continue
		}
		unassignedUsers = append(unassignedUsers, user)
	}
	logic.ReturnSuccessResponseWithJson(w, r, unassignedUsers, "returned unassigned network service users")
}

// @Summary     Add user to network
// @Router      /api/v1/users/add_network_user [put]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Param       username query string true "Username"
// @Param       network_id query string true "Network ID"
// @Success     200 {object} models.User
// @Failure     400 {object} models.ErrorResponse
func addUsertoNetwork(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("username is required"), logic.BadReq))
		return
	}
	netID := r.URL.Query().Get("network_id")
	if netID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), logic.BadReq))
		return
	}
	user := &schema.User{Username: username}
	err := user.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	if user.PlatformRoleID != schema.ServiceUser {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("can only add service users"), logic.BadReq))
		return
	}
	oldUser := *user
	user.UserGroups.Data()[proLogic.GetDefaultNetworkUserGroupID(schema.NetworkID(netID))] = struct{}{}
	logic.UpsertUser(*user)
	logic.LogEvent(&models.Event{
		Action: schema.Update,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   user.Username,
			Name: user.Username,
			Type: schema.UserSub,
		},
		Diff: models.Diff{
			Old: oldUser,
			New: user,
		},
		Origin: schema.Dashboard,
	})

	logic.ReturnSuccessResponseWithJson(w, r, user, "updated user group")
}

// @Summary     Remove user from network
// @Router      /api/v1/users/remove_network_user [put]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Param       username query string true "Username"
// @Param       network_id query string true "Network ID"
// @Success     200 {object} models.User
// @Failure     400 {object} models.ErrorResponse
func removeUserfromNetwork(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("username is required"), logic.BadReq))
		return
	}
	netID := r.URL.Query().Get("network_id")
	if netID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), logic.BadReq))
		return
	}
	user := &schema.User{Username: username}
	err := user.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	if user.PlatformRoleID != schema.ServiceUser {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("can only add service users"), logic.BadReq))
		return
	}
	oldUser := *user
	delete(user.UserGroups.Data(), proLogic.GetDefaultNetworkUserGroupID(schema.NetworkID(netID)))
	logic.UpsertUser(*user)
	logic.LogEvent(&models.Event{
		Action: schema.Update,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   user.Username,
			Name: user.Username,
			Type: schema.UserSub,
		},
		Diff: models.Diff{
			Old: oldUser,
			New: user,
		},
		Origin: schema.Dashboard,
	})

	logic.ReturnSuccessResponseWithJson(w, r, user, "updated user group")
}

// @Summary     Delete user group
// @Router      /api/v1/users/group [delete]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Param       group_id query string true "Group ID required to delete the group"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func deleteUserGroup(w http.ResponseWriter, r *http.Request) {

	gid := r.URL.Query().Get("group_id")
	if gid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("group id is required"), "badrequest"))
		return
	}
	userG, err := proLogic.GetUserGroup(schema.UserGroupID(gid))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to fetch group details"), "badrequest"))
		return
	}
	if userG.Default {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("cannot delete default user group"), "badrequest"))
		return
	}
	err = proLogic.DeleteAndCleanUpGroup(&userG)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// TODO: log event in proLogic.DeleteAndCleanUpGroup so that all deletions are logged.
	logic.LogEvent(&models.Event{
		Action: schema.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   userG.ID.String(),
			Name: userG.Name,
			Type: schema.UserGroupSub,
		},
		Origin: schema.Dashboard,
		Diff: models.Diff{
			Old: userG,
			New: nil,
		},
	})

	logic.ReturnSuccessResponseWithJson(w, r, nil, "deleted user group")
}

// @Summary     List all user roles
// @Router      /api/v1/users/roles [get]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Param       platform query string false "If true, lists platform roles. Otherwise, lists network roles."
// @Success     200 {object}  []schema.UserRole
// @Failure     500 {object} models.ErrorResponse
func ListRoles(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	var roles []schema.UserRole
	var err error
	if platform == "true" {
		roles, err = (&schema.UserRole{}).ListPlatformRoles(r.Context())
	} else {
		roles, err = (&schema.UserRole{}).ListNetworkRoles(r.Context())
	}
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, roles, "successfully fetched user roles permission templates")
}

// @Summary     Get user role permission template
// @Router      /api/v1/users/role [get]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Param       role_id query string true "Role ID required to get the role details"
// @Success     200 {object} schema.UserRole
// @Failure     500 {object} models.ErrorResponse
func getRole(w http.ResponseWriter, r *http.Request) {
	rid := r.URL.Query().Get("role_id")
	if rid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("role is required"), "badrequest"))
		return
	}
	role := &schema.UserRole{ID: schema.UserRoleID(rid)}
	err := role.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, role, "successfully fetched user role permission templates")
}

// @Summary     Create user role permission template
// @Router      /api/v1/users/role [post]
// @Tags        Users
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body schema.UserRole true "User role template"
// @Success     200 {object}  schema.UserRole
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func createRole(w http.ResponseWriter, r *http.Request) {
	var userRole schema.UserRole
	err := json.NewDecoder(r.Body).Decode(&userRole)
	if err != nil {
		slog.Error("error decoding request body", "error",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = proLogic.ValidateCreateRoleReq(&userRole)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	userRole.Default = false
	userRole.GlobalLevelAccess = datatypes.NewJSONType(make(schema.ResourceAccess))
	err = proLogic.CreateRole(&userRole)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: schema.Create,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   userRole.ID.String(),
			Name: userRole.Name,
			Type: schema.UserRoleSub,
		},
		Origin: schema.ClientApp,
	})
	logic.ReturnSuccessResponseWithJson(w, r, userRole, "created user role")
}

// @Summary     Update user role permission template
// @Router      /api/v1/users/role [put]
// @Tags        Users
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body schema.UserRole true "User role template"
// @Success     200 {object} schema.UserRole
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func updateRole(w http.ResponseWriter, r *http.Request) {
	var userRole schema.UserRole
	err := json.NewDecoder(r.Body).Decode(&userRole)
	if err != nil {
		slog.Error("error decoding request body", "error",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	currRole := &schema.UserRole{ID: userRole.ID}
	err = currRole.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = proLogic.ValidateUpdateRoleReq(&userRole)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	userRole.GlobalLevelAccess = datatypes.NewJSONType(make(schema.ResourceAccess))
	err = userRole.Update(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: schema.Update,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   userRole.ID.String(),
			Name: userRole.Name,
			Type: schema.UserRoleSub,
		},
		Diff: models.Diff{
			Old: currRole,
			New: userRole,
		},
		Origin: schema.Dashboard,
	})
	// reset configs for service user
	go proLogic.UpdatesUserGwAccessOnRoleUpdates(currRole.NetworkLevelAccess.Data(), userRole.NetworkLevelAccess.Data(), string(userRole.NetworkID))
	logic.ReturnSuccessResponseWithJson(w, r, userRole, "updated user role")
}

// @Summary     Delete user role permission template
// @Router      /api/v1/users/role [delete]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Param       role_id query string true "Role ID required to delete the role"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func deleteRole(w http.ResponseWriter, r *http.Request) {

	rid := r.URL.Query().Get("role_id")
	if rid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("role is required"), "badrequest"))
		return
	}
	role := &schema.UserRole{ID: schema.UserRoleID(rid)}
	err := role.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("role is required"), "badrequest"))
		return
	}
	err = proLogic.DeleteRole(schema.UserRoleID(rid), false)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: schema.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   role.ID.String(),
			Name: role.Name,
			Type: schema.UserRoleSub,
		},
		Origin: schema.Dashboard,
		Diff: models.Diff{
			Old: role,
			New: nil,
		},
	})
	go proLogic.UpdatesUserGwAccessOnRoleUpdates(role.NetworkLevelAccess.Data(), make(map[schema.RsrcType]map[schema.RsrcID]schema.RsrcPermissionScope), role.NetworkID.String())
	logic.ReturnSuccessResponseWithJson(w, r, nil, "deleted user role")
}

// @Summary     Attach user to a remote access gateway
// @Router      /api/users/{username}/remote_access_gw/{remote_access_gateway_id} [post]
// @Tags        Users
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       username path string true "Username"
// @Param       remote_access_gateway_id path string true "Remote Access Gateway ID"
// @Success     200 {object} models.ReturnUser
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func attachUserToRemoteAccessGw(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	username := params["username"]
	remoteGwID := params["remote_access_gateway_id"]
	if username == "" || remoteGwID == "" {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("required params `username` and `remote_access_gateway_id`"),
				"badrequest",
			),
		)
		return
	}
	user := &schema.User{Username: username}
	err := user.Get(r.Context())
	if err != nil {
		slog.Error("failed to fetch user: ", "username", username, "error", err.Error())
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				fmt.Errorf("failed to fetch user %s, error: %v", username, err),
				"badrequest",
			),
		)
		return
	}
	if user.PlatformRoleID == schema.AdminRole || user.PlatformRoleID == schema.SuperAdminRole {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("superadmins/admins have access to all gateways"), "badrequest"))
		return
	}
	node, err := logic.GetNodeByID(remoteGwID)
	if err != nil {
		slog.Error("failed to fetch gateway node", "nodeID", remoteGwID, "error", err)
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				fmt.Errorf("failed to fetch remote access gateway node, error: %v", err),
				"badrequest",
			),
		)
		return
	}
	if !node.IsIngressGateway {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("node is not a remote access gateway"), "badrequest"),
		)
		return
	}
	err = logic.UpsertUser(*user)
	if err != nil {
		slog.Error("failed to update user's gateways", "user", username, "error", err)
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				fmt.Errorf("failed to fetch remote access gateway node,error: %v", err),
				"badrequest",
			),
		)
		return
	}

	json.NewEncoder(w).Encode(logic.ToReturnUser(user))
}

// @Summary     Remove user from a remote access gateway
// @Router      /api/users/{username}/remote_access_gw/{remote_access_gateway_id} [delete]
// @Tags        Users
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       username path string true "Username"
// @Param       remote_access_gateway_id path string true "Remote Access Gateway ID"
// @Success     200 {object} models.ReturnUser
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func removeUserFromRemoteAccessGW(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	username := params["username"]
	remoteGwID := params["remote_access_gateway_id"]
	if username == "" || remoteGwID == "" {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("required params `username` and `remote_access_gateway_id`"),
				"badrequest",
			),
		)
		return
	}
	user := &schema.User{Username: username}
	err := user.Get(r.Context())
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				fmt.Errorf("failed to fetch user %s, error: %v", username, err),
				"badrequest",
			),
		)
		return
	}
	go func(user *schema.User, remoteGwID string) {
		extclients, err := logic.GetAllExtClients()
		if err != nil {
			slog.Error("failed to fetch extclients", "error", err)
			return
		}
		for _, extclient := range extclients {
			if extclient.OwnerID == user.Username && remoteGwID == extclient.IngressGatewayID {
				err = logic.DeleteExtClientAndCleanup(extclient)
				if err != nil {
					slog.Error("failed to delete extclient",
						"id", extclient.ClientID, "owner", user.Username, "error", err)
				} else {
					if err := mq.PublishDeletedClientPeerUpdate(&extclient); err != nil {
						slog.Error("error setting ext peers: " + err.Error())
					}
				}
			}
		}
		if servercfg.IsDNSMode() {
			logic.SetDNS()
		}
	}(user, remoteGwID)

	err = logic.UpsertUser(*user)
	if err != nil {
		slog.Error("failed to update user gateways", "user", username, "error", err)
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("failed to fetch remote access gaetway node "+err.Error()),
				"badrequest",
			),
		)
		return
	}
	json.NewEncoder(w).Encode(logic.ToReturnUser(user))
}

func getUserRemoteAccessNetworks(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	username := r.Header.Get("user")
	user := &schema.User{Username: username}
	err := user.Get(r.Context())
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch user %s, error: %v", username, err), "badrequest"))
		return
	}
	userGws := make(map[string][]models.UserRemoteGws)
	var networks []schema.Network
	networkMap := make(map[string]struct{})
	userGwNodes := proLogic.GetUserRAGNodes(user)
	for _, node := range userGwNodes {
		network := &schema.Network{Name: node.Network}
		err := network.Get(r.Context())
		if err != nil {
			slog.Error("failed to get node network", "error", err)
			continue
		}
		if _, ok := networkMap[network.Name]; ok {
			continue
		}
		networkMap[network.Name] = struct{}{}
		networks = append(networks, *network)
	}

	slog.Debug("returned user gws", "user", username, "gws", userGws)
	logic.ReturnSuccessResponseWithJson(w, r, networks, "fetched user accessible networks")
}

func getUserRemoteAccessNetworkGateways(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	username := r.Header.Get("user")
	user := &schema.User{Username: username}
	err := user.Get(r.Context())
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch user %s, error: %v", username, err), "badrequest"))
		return
	}
	network := params["network"]
	if network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("required params network"), "badrequest"))
		return
	}
	userGws := []models.UserRAGs{}

	userGwNodes := proLogic.GetUserRAGNodes(user)
	for _, node := range userGwNodes {
		if node.Network != network {
			continue
		}

		host := &schema.Host{
			ID: node.HostID,
		}
		err = host.Get(r.Context())
		if err != nil {
			continue
		}

		userGws = append(userGws, models.UserRAGs{
			GwID:              node.ID.String(),
			GWName:            host.Name,
			Network:           node.Network,
			IsInternetGateway: node.IsInternetGateway,
			Metadata:          node.Metadata,
		})

	}

	slog.Debug("returned user gws", "user", username, "gws", userGws)
	logic.ReturnSuccessResponseWithJson(w, r, userGws, "fetched user accessible gateways in network "+network)
}

func getRemoteAccessGatewayConf(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	username := r.Header.Get("user")
	user := &schema.User{Username: username}
	err := user.Get(r.Context())
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch user %s, error: %v", username, err), "badrequest"))
		return
	}
	remoteGwID := params["access_point_id"]
	if remoteGwID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("required params access_point_id"), "badrequest"))
		return
	}
	var req models.UserRemoteGwsReq
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		slog.Error("error decoding request body: ", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	userGwNodes := proLogic.GetUserRAGNodes(user)
	if _, ok := userGwNodes[remoteGwID]; !ok {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("access denied"), "forbidden"))
		return
	}
	node, err := logic.GetNodeByID(remoteGwID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch gw node %s, error: %v", remoteGwID, err), "badrequest"))
		return
	}
	host := &schema.Host{
		ID: node.HostID,
	}
	err = host.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch gw host %s, error: %v", remoteGwID, err), "badrequest"))
		return
	}
	network := &schema.Network{Name: node.Network}
	err = network.Get(r.Context())
	if err != nil {
		slog.Error("failed to get node network", "error", err)
	}
	var userConf models.ExtClient
	allextClients, err := logic.GetAllExtClients()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	for _, extClient := range allextClients {
		if extClient.Network != network.Name || extClient.IngressGatewayID != node.ID.String() {
			continue
		}
		if extClient.RemoteAccessClientID == req.RemoteAccessClientID && extClient.OwnerID == username {
			userConf = extClient
			userConf.AllowedIPs = logic.GetExtclientAllowedIPs(extClient)
		}
	}
	if userConf.ClientID == "" {
		// create a new conf
		userConf.OwnerID = user.Username
		userConf.RemoteAccessClientID = req.RemoteAccessClientID
		userConf.IngressGatewayID = node.ID.String()
		logic.SetDNSOnWgConfig(&node, &userConf)

		userConf.Network = node.Network
		listenPort := logic.GetPeerListenPort(host)
		if host.EndpointIP.To4() == nil {
			userConf.IngressGatewayEndpoint = fmt.Sprintf("[%s]:%d", host.EndpointIPv6.String(), listenPort)
		} else {
			userConf.IngressGatewayEndpoint = fmt.Sprintf("%s:%d", host.EndpointIP.String(), listenPort)
		}
		userConf.Enabled = true
		parentNetwork := &schema.Network{Name: node.Network}
		err = parentNetwork.Get(r.Context())
		if err == nil { // check if parent network default ACL is enabled (yes) or not (no)
			userConf.Enabled = parentNetwork.DefaultACL == "yes"
		}
		userConf.Tags = make(map[models.TagID]struct{})
		// userConf.Tags[models.TagID(fmt.Sprintf("%s.%s", userConf.Network,
		// 	models.RemoteAccessTagName))] = struct{}{}
		if err = logic.CreateExtClient(&userConf); err != nil {
			slog.Error(
				"failed to create extclient",
				"user",
				r.Header.Get("user"),
				"network",
				node.Network,
				"error",
				err,
			)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}
	userGw := models.UserRemoteGws{
		GwID:              node.ID.String(),
		GWName:            host.Name,
		Network:           node.Network,
		GwClient:          userConf,
		Connected:         true,
		IsInternetGateway: node.IsInternetGateway,
		GwPeerPublicKey:   host.PublicKey.String(),
		GwListenPort:      logic.GetPeerListenPort(host),
		Metadata:          node.Metadata,
		AllowedEndpoints:  getAllowedRagEndpoints(&node, host),
		NetworkAddresses:  []string{network.AddressRange, network.AddressRange6},
		ManageDNS:         host.DNS == "yes",
		DnsAddress:        node.IngressDNS,
		Addresses:         utils.NoEmptyStringToCsv(node.Address.String(), node.Address6.String()),
	}

	slog.Debug("returned user gw config", "user", user.Username, "gws", userGw)
	logic.ReturnSuccessResponseWithJson(w, r, userGw, "fetched user config to gw "+remoteGwID)
}

// @Summary     Get user remote access gateways
// @Router      /api/users/{username}/remote_access_gw [get]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Param       username path string true "Username to fetch all the gateways with access"
// @Param       device_id query string false "Device ID"
// @Param       remote_access_clientid query string false "Remote access client ID"
// @Param       from_mobile query string false "If 'true', returns array format"
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func getUserRemoteAccessGwsV1(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	username := params["username"]
	if username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("required params username"), "badrequest"))
		return
	}
	user := &schema.User{Username: username}
	err := user.Get(r.Context())
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch user %s, error: %v", username, err), "badrequest"))
		return
	}
	deviceID := r.URL.Query().Get("device_id")
	remoteAccessClientID := r.URL.Query().Get("remote_access_clientid")
	var req models.UserRemoteGwsReq
	if remoteAccessClientID == "" {
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			slog.Error("error decoding request body: ", "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}
	reqFromMobile := r.URL.Query().Get("from_mobile") == "true"
	if req.RemoteAccessClientID == "" && remoteAccessClientID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("remote access client id cannot be empty"), "badrequest"))
		return
	}
	if req.RemoteAccessClientID == "" {
		req.RemoteAccessClientID = remoteAccessClientID
	}
	userGws := make(map[string][]models.UserRemoteGws)
	allextClients, err := logic.GetAllExtClients()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	userGwNodes := proLogic.GetUserRAGNodes(user)

	userExtClients := make(map[string][]models.ExtClient)

	// group all extclients of the requesting user by ingress
	// gateway.
	for _, extClient := range allextClients {
		// filter our extclients that don't belong to this user.
		if extClient.OwnerID != username {
			continue
		}

		if extClient.RemoteAccessClientID == "" {
			continue
		}

		_, ok := userExtClients[extClient.IngressGatewayID]
		if !ok {
			userExtClients[extClient.IngressGatewayID] = []models.ExtClient{}
		}

		userExtClients[extClient.IngressGatewayID] = append(userExtClients[extClient.IngressGatewayID], extClient)
	}

	for ingressGatewayID, extClients := range userExtClients {
		logic.SortExtClient(extClients)

		node, ok := userGwNodes[ingressGatewayID]
		if !ok {
			continue
		}

		var gwClient models.ExtClient
		var found bool
		if deviceID != "" {
			for _, extClient := range extClients {
				if extClient.DeviceID == deviceID {
					gwClient = extClient
					found = true
					break
				}
			}
		}

		if !found && req.RemoteAccessClientID != "" {
			for _, extClient := range extClients {
				if extClient.RemoteAccessClientID == req.RemoteAccessClientID {
					gwClient = extClient
					found = true
					break
				}
			}
		}

		if !found && len(extClients) > 0 && deviceID == "" {
			// TODO: prevent ip clashes.
			gwClient = extClients[0]
		}

		host := &schema.Host{
			ID: node.HostID,
		}
		err = host.Get(r.Context())
		if err != nil {
			continue
		}
		network := &schema.Network{Name: node.Network}
		err = network.Get(r.Context())
		if err != nil {
			slog.Error("failed to get node network", "error", err)
			continue
		}
		nodesWithStatus := logic.AddStatusToNodes([]models.Node{node}, false)
		if len(nodesWithStatus) > 0 {
			node = nodesWithStatus[0]
		}

		gws := userGws[node.Network]
		if gwClient.DNS == "" {
			logic.SetDNSOnWgConfig(&node, &gwClient)
		}

		gwClient.IngressGatewayEndpoint = utils.GetExtClientEndpoint(
			host.EndpointIP,
			host.EndpointIPv6,
			logic.GetPeerListenPort(host),
		)
		gwClient.AllowedIPs = logic.GetExtclientAllowedIPs(gwClient)
		gw := models.UserRemoteGws{
			GwID:              node.ID.String(),
			GWName:            host.Name,
			Network:           node.Network,
			GwClient:          gwClient,
			Connected:         true,
			IsInternetGateway: node.IsInternetGateway,
			GwPeerPublicKey:   host.PublicKey.String(),
			GwListenPort:      logic.GetPeerListenPort(host),
			Metadata:          node.Metadata,
			AllowedEndpoints:  getAllowedRagEndpoints(&node, host),
			NetworkAddresses:  []string{network.AddressRange, network.AddressRange6},
			Status:            node.Status,
			ManageDNS:         host.DNS == "yes",
			DnsAddress:        node.IngressDNS,
			Addresses:         utils.NoEmptyStringToCsv(node.Address.String(), node.Address6.String()),
		}
		hNs := logic.GetNameserversForNode(&node)
		for _, nsI := range hNs {
			if nsI.IsFallback {
				// skip fallback nameservers for user remote access gws.
				continue
			}
			gw.Nameservers = append(gw.Nameservers, nsI)
			gw.MatchDomains = append(gw.MatchDomains, nsI.MatchDomain)
			if nsI.IsSearchDomain {
				gw.SearchDomains = append(gw.SearchDomains, nsI.MatchDomain)
			}
		}
		gw.MatchDomains = append(gw.MatchDomains, logic.GetEgressDomainsByAccessForUser(user, schema.NetworkID(node.Network))...)
		gws = append(gws, gw)
		userGws[node.Network] = gws
		delete(userGwNodes, node.ID.String())
	}

	// add remaining gw nodes to resp
	for gwID := range userGwNodes {
		node, err := logic.GetNodeByID(gwID)
		if err != nil {
			continue
		}
		if !node.IsIngressGateway {
			continue
		}
		if node.PendingDelete {
			continue
		}
		host := &schema.Host{
			ID: node.HostID,
		}
		err = host.Get(r.Context())
		if err != nil {
			continue
		}
		nodesWithStatus := logic.AddStatusToNodes([]models.Node{node}, false)
		if len(nodesWithStatus) > 0 {
			node = nodesWithStatus[0]
		}
		network := &schema.Network{Name: node.Network}
		err = network.Get(r.Context())
		if err != nil {
			slog.Error("failed to get node network", "error", err)
		}
		gws := userGws[node.Network]
		gw := models.UserRemoteGws{
			GwID:              node.ID.String(),
			GWName:            host.Name,
			Network:           node.Network,
			IsInternetGateway: node.IsInternetGateway,
			GwPeerPublicKey:   host.PublicKey.String(),
			GwListenPort:      logic.GetPeerListenPort(host),
			Metadata:          node.Metadata,
			AllowedEndpoints:  getAllowedRagEndpoints(&node, host),
			NetworkAddresses:  []string{network.AddressRange, network.AddressRange6},
			Status:            node.Status,
			ManageDNS:         host.DNS == "yes",
			DnsAddress:        node.IngressDNS,
			Addresses:         utils.NoEmptyStringToCsv(node.Address.String(), node.Address6.String()),
		}
		hNs := logic.GetNameserversForNode(&node)
		for _, nsI := range hNs {
			if nsI.IsFallback {
				// skip fallback nameservers for user remote access gws.
				continue
			}
			gw.Nameservers = append(gw.Nameservers, nsI)
			gw.MatchDomains = append(gw.MatchDomains, nsI.MatchDomain)
			if nsI.IsSearchDomain {
				gw.SearchDomains = append(gw.SearchDomains, nsI.MatchDomain)
			}
		}
		gw.MatchDomains = append(gw.MatchDomains, logic.GetEgressDomainsByAccessForUser(user, schema.NetworkID(node.Network))...)
		gws = append(gws, gw)
		userGws[node.Network] = gws
	}

	if reqFromMobile {
		// send resp in array format
		userGwsArr := []models.UserRemoteGws{}
		for _, userGwI := range userGws {
			userGwsArr = append(userGwsArr, userGwI...)
		}
		logic.ReturnSuccessResponseWithJson(w, r, userGwsArr, "fetched gateways for user"+username)
		return
	}
	slog.Debug("returned user gws", "user", username, "gws", userGws)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userGws)
}

// @Summary     List users attached to a remote access gateway
// @Router      /api/users/ingress/{ingress_id} [get]
// @Tags        Users
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       ingress_id path string true "Ingress Gateway ID"
// @Success     200 {array} models.IngressGwUsers
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func ingressGatewayUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	ingressID := params["ingress_id"]
	node, err := logic.GetNodeByID(ingressID)
	if err != nil {
		slog.Error("failed to get ingress node", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	gwUsers, err := logic.GetIngressGwUsers(node)
	if err != nil {
		slog.Error(
			"failed to get users on ingress gateway",
			"nodeid",
			ingressID,
			"network",
			node.Network,
			"user",
			r.Header.Get("user"),
			"error",
			err,
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(gwUsers)
}

// @Summary     List users network IP mappings
// @Router      /api/v1/users/network_ip [get]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.UserIPMap
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func userNetworkMapping(w http.ResponseWriter, r *http.Request) {

	extclients, err := logic.GetAllExtClients()
	if err != nil {
		slog.Error(
			"failed to get users on ingress gateway",
			"user",
			r.Header.Get("user"),
			"error",
			err,
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	userMapping := models.UserIPMap{
		Mappings: make(map[string]models.UserMapping),
	}
	for _, extclient := range extclients {
		if extclient.DeviceID != "" || extclient.RemoteAccessClientID != "" {
			if extclient.OwnerID == "" {
				continue
			}
			user := &schema.User{Username: extclient.OwnerID}
			err = user.Get(r.Context())
			if err != nil {
				continue
			}
			if user.AccountDisabled {
				continue
			}
			userIPMap := models.UserMapping{
				User: user.Username,
			}
			if extclient.Address != "" {
				if len(user.UserGroups.Data()) > 0 {
					for grpID := range user.UserGroups.Data() {
						userIPMap.Groups = append(userIPMap.Groups, grpID.String())
					}
				}
			}
			userMapping.Mappings[extclient.Address] = userIPMap
		}
	}
	logic.ReturnSuccessResponseWithJson(w, r, userMapping, "returned user network ip map")
}

func getAllowedRagEndpoints(ragNode *models.Node, ragHost *schema.Host) []string {
	endpoints := []string{}
	if len(ragHost.EndpointIP) > 0 {
		endpoints = append(endpoints, ragHost.EndpointIP.String())
	}
	if len(ragHost.EndpointIPv6) > 0 {
		endpoints = append(endpoints, ragHost.EndpointIPv6.String())
	}
	if servercfg.IsPro {
		for _, ip := range ragNode.AdditionalRagIps {
			endpoints = append(endpoints, ip.String())
		}
	}
	return endpoints
}

// @Summary     Get all pending users
// @Router      /api/users_pending [get]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Success     200 {array} models.ReturnUser
// @Failure     500 {object} models.ErrorResponse
func getPendingUsers(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	users, err := logic.ListPendingReturnUsers()
	if err != nil {
		logger.Log(0, "failed to fetch users: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logic.SortUsers(users[:])
	logger.Log(2, r.Header.Get("user"), "fetched pending users")
	json.NewEncoder(w).Encode(users)
}

// @Summary     Approve a pending user
// @Router      /api/users_pending/user/{username} [post]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Param       username path string true "Username of the pending user to approve"
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func approvePendingUser(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	username := params["username"]
	users, err := logic.ListPendingUsers()

	if err != nil {
		logger.Log(0, "failed to fetch users: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	for _, user := range users {
		if user.UserName == username {
			var newPass, fetchErr = logic.FetchPassValue("")
			if fetchErr != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(fetchErr, "internal"))
				return
			}
			if err = logic.CreateUser(&schema.User{
				Username:                   user.UserName,
				ExternalIdentityProviderID: user.ExternalIdentityProviderID,
				Password:                   newPass,
				AuthType:                   user.AuthType,
				PlatformRoleID:             schema.ServiceUser,
			}); err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to create user: %s", err), "internal"))
				return
			}
			err = logic.DeletePendingUser(username)
			if err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to delete pending user: %s", err), "internal"))
				return
			}
			break
		}
	}
	logic.LogEvent(&models.Event{
		Action: schema.Create,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   username,
			Name: username,
			Type: schema.PendingUserSub,
		},
		Origin: schema.Dashboard,
	})
	logic.ReturnSuccessResponse(w, r, "approved "+username)
}

// @Summary     Delete a pending user
// @Router      /api/users_pending/user/{username} [delete]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Param       username path string true "Username of the pending user to delete"
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func deletePendingUser(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	username := params["username"]
	users, err := logic.ListPendingReturnUsers()

	if err != nil {
		logger.Log(0, "failed to fetch users: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	for _, user := range users {
		if user.UserName == username {
			err = logic.DeletePendingUser(username)
			if err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to delete pending user: %s", err), "internal"))
				return
			}
			break
		}
	}
	logic.LogEvent(&models.Event{
		Action: schema.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   username,
			Name: username,
			Type: schema.PendingUserSub,
		},
		Origin: schema.Dashboard,
		Diff: models.Diff{
			Old: schema.User{
				Username: username,
			},
			New: nil,
		},
	})
	logic.ReturnSuccessResponse(w, r, "deleted pending "+username)
}

// @Summary     Delete all pending users
// @Router      /api/users_pending [delete]
// @Tags        Users
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func deleteAllPendingUsers(w http.ResponseWriter, r *http.Request) {
	// set header.
	err := database.DeleteAllRecords(database.PENDING_USERS_TABLE_NAME)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to delete all pending users "+err.Error()), "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: schema.DeleteAll,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.PendingUserSub,
		},
		Origin: schema.Dashboard,
	})
	logic.ReturnSuccessResponse(w, r, "cleared all pending users")
}

// @Summary     Sync users and groups from IDP
// @Router      /api/idp/sync [post]
// @Tags        IDP
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.SuccessResponse
func syncIDP(w http.ResponseWriter, r *http.Request) {
	go func() {
		err := proAuth.SyncFromIDP()
		if err != nil {
			logger.Log(0, "failed to sync from idp: ", err.Error())
		} else {
			logger.Log(0, "sync from idp complete")
		}
	}()

	logic.ReturnSuccessResponse(w, r, "starting sync from idp")
}

// @Summary     Test IDP Sync Credentials
// @Router      /api/idp/sync/test [post]
// @Tags        IDP
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body models.IDPSyncTestRequest true "IDP sync test request"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
func testIDPSync(w http.ResponseWriter, r *http.Request) {
	var req models.IDPSyncTestRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		err = fmt.Errorf("failed to decode request body: %v", err)
		logger.Log(0, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	var idpClient idp.Client
	switch req.AuthProvider {
	case "google":
		idpClient, err = google.NewGoogleWorkspaceClient(req.GoogleAdminEmail, req.GoogleSACredsJson)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	case "azure-ad":
		idpClient = azure.NewAzureEntraIDClient(req.ClientID, req.ClientSecret, req.AzureTenantID)
	case "okta":
		idpClient, err = okta.NewOktaClient(req.OktaOrgURL, req.OktaAPIToken)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	default:
		err = fmt.Errorf("invalid auth provider: %s", req.AuthProvider)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	err = idpClient.Verify()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	logic.ReturnSuccessResponse(w, r, "idp sync test successful")
}

// @Summary     Get IDP sync status
// @Router      /api/idp/sync/status [get]
// @Tags        IDP
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.IDPSyncStatus
func getIDPSyncStatus(w http.ResponseWriter, r *http.Request) {
	logic.ReturnSuccessResponseWithJson(w, r, proAuth.GetIDPSyncStatus(), "idp sync status retrieved")
}

// @Summary     Remove IDP integration
// @Router      /api/idp [delete]
// @Tags        IDP
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func removeIDPIntegration(w http.ResponseWriter, r *http.Request) {
	superAdmin, err := logic.GetSuperAdmin()
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("failed to get superadmin: %v", err), "internal"),
		)
		return
	}

	if superAdmin.AuthType == schema.OAuth {
		err := fmt.Errorf(
			"cannot remove IdP integration because an OAuth user has the super-admin role; transfer the super-admin role to another user first",
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	settings := logic.GetServerSettings()
	settings.AuthProvider = ""
	settings.OIDCIssuer = ""
	settings.ClientID = ""
	settings.ClientSecret = ""
	settings.SyncEnabled = false
	settings.GoogleAdminEmail = ""
	settings.GoogleSACredsJson = ""
	settings.AzureTenant = ""
	settings.OktaOrgURL = ""
	settings.OktaAPIToken = ""
	settings.UserFilters = nil
	settings.GroupFilters = nil
	settings.IDPSyncInterval = ""

	err = logic.UpsertServerSettings(settings)
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("failed to remove idp integration: %v", err), "internal"),
		)
		return
	}

	proAuth.ResetAuthProvider()
	proAuth.ResetIDPSyncHook()

	go func() {
		err := proAuth.SyncFromIDP()
		if err != nil {
			logger.Log(0, "failed to sync from idp: ", err.Error())
		} else {
			logger.Log(0, "sync from idp complete")
		}
	}()

	logic.ReturnSuccessResponse(w, r, "removed idp integration successfully")
}
