package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	proAuth "github.com/gravitl/netmaker/pro/auth"
	"github.com/gravitl/netmaker/pro/email"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/utils"
	"golang.org/x/exp/slog"
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
	r.HandleFunc("/api/v1/users/groups", logic.SecurityCheck(true, http.HandlerFunc(listUserGroups))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/group", logic.SecurityCheck(true, http.HandlerFunc(getUserGroup))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/group", logic.SecurityCheck(true, http.HandlerFunc(createUserGroup))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/users/group", logic.SecurityCheck(true, http.HandlerFunc(updateUserGroup))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/users/group", logic.SecurityCheck(true, http.HandlerFunc(deleteUserGroup))).Methods(http.MethodDelete)

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
}

// swagger:route POST /api/v1/users/invite-signup user userInviteSignUp
//
// user signup via invite.
//
//	Schemes: https
//
//	Responses:
//		200: ReturnSuccessResponse
func userInviteSignUp(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	code := r.URL.Query().Get("invite_code")
	in, err := logic.GetUserInvite(email)
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
	_, err = logic.GetUser(email)
	if err == nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user already exists"), "badrequest"))
		return
	}
	var user models.User
	err = json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		logger.Log(0, user.UserName, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if user.UserName != email {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("username not matching with invite"), "badrequest"))
		return
	}
	if user.Password == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("password cannot be empty"), "badrequest"))
		return
	}

	user.UserGroups = in.UserGroups
	user.PlatformRoleID = models.UserRoleID(in.PlatformRoleID)
	if user.PlatformRoleID == "" {
		user.PlatformRoleID = models.ServiceUser
	}
	user.NetworkRoles = in.NetworkRoles
	err = logic.CreateUser(&user)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	// delete invite
	logic.DeleteUserInvite(email)
	logic.DeletePendingUser(email)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	logic.ReturnSuccessResponse(w, r, "created user successfully "+email)
}

// swagger:route GET /api/v1/users/invite user userInviteVerify
//
// verfies user invite.
//
//	Schemes: https
//
//	Responses:
//		200: ReturnSuccessResponse
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

// swagger:route POST /api/v1/users/invite user inviteUsers
//
// invite users.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
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
		caller, err := logic.GetUser(callerUserName)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "notfound"))
			return
		}
		if inviteReq.PlatformRoleID == models.SuperAdminRole.String() {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("super admin cannot be invited"), "badrequest"))
			return
		}
		if inviteReq.PlatformRoleID == "" {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("platform role id cannot be empty"), "badrequest"))
			return
		}
		if (inviteReq.PlatformRoleID == models.AdminRole.String() ||
			inviteReq.PlatformRoleID == models.SuperAdminRole.String()) && caller.PlatformRoleID != models.SuperAdminRole {
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
	_, err = logic.GetRole(models.UserRoleID(inviteReq.PlatformRoleID))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	for _, inviteeEmail := range inviteReq.UserEmails {
		// check if user with email exists, then ignore
		if !email.IsValid(inviteeEmail) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid email "+inviteeEmail), "badrequest"))
			return
		}
		_, err := logic.GetUser(inviteeEmail)
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
			Action: models.Create,
			Source: models.Subject{
				ID:   r.Header.Get("user"),
				Name: r.Header.Get("user"),
				Type: models.UserSub,
			},
			Target: models.Subject{
				ID:   inviteeEmail,
				Name: inviteeEmail,
				Type: models.UserInviteSub,
			},
			Origin: models.Dashboard,
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

// swagger:route GET /api/v1/users/invites user listUserInvites
//
// lists all pending invited users.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: ReturnSuccessResponseWithJson
func listUserInvites(w http.ResponseWriter, r *http.Request) {
	usersInvites, err := logic.ListUserInvites()
	if err != nil {
		logger.Log(0, "failed to fetch users: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, usersInvites, "fetched pending user invites")
}

// swagger:route DELETE /api/v1/users/invite user deleteUserInvite
//
// delete pending invite.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: ReturnSuccessResponse
func deleteUserInvite(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("invitee_email")
	err := logic.DeleteUserInvite(email)
	if err != nil {
		logger.Log(0, "failed to delete user invite: ", email, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: models.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		Target: models.Subject{
			ID:   email,
			Name: email,
			Type: models.UserInviteSub,
		},
		Origin: models.Dashboard,
	})
	logic.ReturnSuccessResponse(w, r, "deleted user invite")
}

// swagger:route DELETE /api/v1/users/invites user deleteAllUserInvites
//
// deletes all pending invites.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: ReturnSuccessResponse
func deleteAllUserInvites(w http.ResponseWriter, r *http.Request) {
	err := database.DeleteAllRecords(database.USER_INVITES_TABLE_NAME)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to delete all pending user invites "+err.Error()), "internal"))
		return
	}
	logic.ReturnSuccessResponse(w, r, "cleared all pending user invites")
}

// swagger:route GET /api/v1/user/groups user listUserGroups
//
// Get all user groups.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func listUserGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := proLogic.ListUserGroups()
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, groups, "successfully fetched user groups")
}

// swagger:route GET /api/v1/user/group user getUserGroup
//
// Get user group.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func getUserGroup(w http.ResponseWriter, r *http.Request) {

	gid := r.URL.Query().Get("group_id")
	if gid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("group id is required"), "badrequest"))
		return
	}
	group, err := proLogic.GetUserGroup(models.UserGroupID(gid))
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, group, "successfully fetched user group")
}

// swagger:route POST /api/v1/user/group user createUserGroup
//
// Create user groups.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func createUserGroup(w http.ResponseWriter, r *http.Request) {
	var userGroupReq models.CreateGroupReq
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
	err = proLogic.CreateUserGroup(userGroupReq.Group)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	networks, err := logic.GetNetworks()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	for _, network := range networks {
		acl := models.Acl{
			ID:          uuid.New().String(),
			Name:        fmt.Sprintf("%s group", userGroupReq.Group.Name),
			MetaData:    "This Policy allows user group to communicate with all gateways",
			Default:     false,
			ServiceType: models.Any,
			NetworkID:   models.NetworkID(network.NetID),
			Proto:       models.ALL,
			RuleType:    models.UserPolicy,
			Src: []models.AclPolicyTag{
				{
					ID:    models.UserGroupAclID,
					Value: userGroupReq.Group.ID.String(),
				},
			},
			Dst: []models.AclPolicyTag{
				{
					ID:    models.NodeTagID,
					Value: fmt.Sprintf("%s.%s", models.NetworkID(network.NetID), models.GwTagName),
				}},
			AllowedDirection: models.TrafficDirectionUni,
			Enabled:          true,
			CreatedBy:        "auto",
			CreatedAt:        time.Now().UTC(),
		}
		err = logic.InsertAcl(acl)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}

	for _, userID := range userGroupReq.Members {
		user, err := logic.GetUser(userID)
		if err != nil {
			continue
		}
		if len(user.UserGroups) == 0 {
			user.UserGroups = make(map[models.UserGroupID]struct{})
		}
		user.UserGroups[userGroupReq.Group.ID] = struct{}{}
		logic.UpsertUser(*user)
	}
	logic.LogEvent(&models.Event{
		Action: models.Create,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		Target: models.Subject{
			ID:   userGroupReq.Group.ID.String(),
			Name: userGroupReq.Group.Name,
			Type: models.UserGroupSub,
		},
		Origin: models.Dashboard,
	})
	logic.ReturnSuccessResponseWithJson(w, r, userGroupReq.Group, "created user group")
}

// swagger:route PUT /api/v1/user/group user updateUserGroup
//
// Update user group.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func updateUserGroup(w http.ResponseWriter, r *http.Request) {
	var userGroup models.UserGroup
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
	err = proLogic.UpdateUserGroup(userGroup)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: models.Update,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		Target: models.Subject{
			ID:   userGroup.ID.String(),
			Name: userGroup.Name,
			Type: models.UserGroupSub,
		},
		Diff: models.Diff{
			Old: currUserG,
			New: userGroup,
		},
		Origin: models.Dashboard,
	})
	// reset configs for service user
	go proLogic.UpdatesUserGwAccessOnGrpUpdates(currUserG.NetworkRoles, userGroup.NetworkRoles)
	logic.ReturnSuccessResponseWithJson(w, r, userGroup, "updated user group")
}

// swagger:route DELETE /api/v1/user/group user deleteUserGroup
//
// delete user group.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
//
// @Summary     Delete user group.
// @Router      /api/v1/user/group [delete]
// @Tags        Users
// @Param       group_id query string true "group id required to delete the role"
// @Success     200 {string} string
// @Failure     500 {object} models.ErrorResponse
func deleteUserGroup(w http.ResponseWriter, r *http.Request) {

	gid := r.URL.Query().Get("group_id")
	if gid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("group id is required"), "badrequest"))
		return
	}
	userG, err := proLogic.GetUserGroup(models.UserGroupID(gid))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to fetch group details"), "badrequest"))
		return
	}
	if userG.Default {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("cannot delete default user group"), "badrequest"))
		return
	}
	err = proLogic.DeleteUserGroup(models.UserGroupID(gid))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: models.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		Target: models.Subject{
			ID:   userG.ID.String(),
			Name: userG.Name,
			Type: models.UserGroupSub,
		},
		Origin: models.Dashboard,
	})
	go proLogic.UpdatesUserGwAccessOnGrpUpdates(userG.NetworkRoles, make(map[models.NetworkID]map[models.UserRoleID]struct{}))
	logic.ReturnSuccessResponseWithJson(w, r, nil, "deleted user group")
}

// @Summary     lists all user roles.
// @Router      /api/v1/user/roles [get]
// @Tags        Users
// @Param       role_id query string true "roleid required to get the role details"
// @Success     200 {object}  []models.UserRolePermissionTemplate
// @Failure     500 {object} models.ErrorResponse
func ListRoles(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	var roles []models.UserRolePermissionTemplate
	var err error
	if platform == "true" {
		roles, err = logic.ListPlatformRoles()
	} else {
		roles, err = proLogic.ListNetworkRoles()
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

// @Summary     Get user role permission template.
// @Router      /api/v1/user/role [get]
// @Tags        Users
// @Param       role_id query string true "roleid required to get the role details"
// @Success     200 {object} models.UserRolePermissionTemplate
// @Failure     500 {object} models.ErrorResponse
func getRole(w http.ResponseWriter, r *http.Request) {
	rid := r.URL.Query().Get("role_id")
	if rid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("role is required"), "badrequest"))
		return
	}
	role, err := logic.GetRole(models.UserRoleID(rid))
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, role, "successfully fetched user role permission templates")
}

// @Summary     Create user role permission template.
// @Router      /api/v1/user/role [post]
// @Tags        Users
// @Param       body body models.UserRolePermissionTemplate true "user role template"
// @Success     200 {object}  models.UserRolePermissionTemplate
// @Failure     500 {object} models.ErrorResponse
func createRole(w http.ResponseWriter, r *http.Request) {
	var userRole models.UserRolePermissionTemplate
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
	userRole.GlobalLevelAccess = make(map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope)
	err = proLogic.CreateRole(userRole)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: models.Create,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		Target: models.Subject{
			ID:   userRole.ID.String(),
			Name: userRole.Name,
			Type: models.UserRoleSub,
		},
		Origin: models.ClientApp,
	})
	logic.ReturnSuccessResponseWithJson(w, r, userRole, "created user role")
}

// @Summary     Update user role permission template.
// @Router      /api/v1/user/role [put]
// @Tags        Users
// @Param       body body models.UserRolePermissionTemplate true "user role template"
// @Success     200 {object} models.UserRolePermissionTemplate
// @Failure     500 {object} models.ErrorResponse
func updateRole(w http.ResponseWriter, r *http.Request) {
	var userRole models.UserRolePermissionTemplate
	err := json.NewDecoder(r.Body).Decode(&userRole)
	if err != nil {
		slog.Error("error decoding request body", "error",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	currRole, err := logic.GetRole(userRole.ID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = proLogic.ValidateUpdateRoleReq(&userRole)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	userRole.GlobalLevelAccess = make(map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope)
	err = proLogic.UpdateRole(userRole)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: models.Update,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		Target: models.Subject{
			ID:   userRole.ID.String(),
			Name: userRole.Name,
			Type: models.UserRoleSub,
		},
		Diff: models.Diff{
			Old: currRole,
			New: userRole,
		},
		Origin: models.Dashboard,
	})
	// reset configs for service user
	go proLogic.UpdatesUserGwAccessOnRoleUpdates(currRole.NetworkLevelAccess, userRole.NetworkLevelAccess, string(userRole.NetworkID))
	logic.ReturnSuccessResponseWithJson(w, r, userRole, "updated user role")
}

// @Summary     Delete user role permission template.
// @Router      /api/v1/user/role [delete]
// @Tags        Users
// @Param       role_id query string true "roleid required to delete the role"
// @Success     200 {string} string
// @Failure     500 {object} models.ErrorResponse
func deleteRole(w http.ResponseWriter, r *http.Request) {

	rid := r.URL.Query().Get("role_id")
	if rid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("role is required"), "badrequest"))
		return
	}
	role, err := logic.GetRole(models.UserRoleID(rid))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("role is required"), "badrequest"))
		return
	}
	err = proLogic.DeleteRole(models.UserRoleID(rid), false)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: models.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		Target: models.Subject{
			ID:   role.ID.String(),
			Name: role.Name,
			Type: models.UserRoleSub,
		},
		Origin: models.Dashboard,
	})
	go proLogic.UpdatesUserGwAccessOnRoleUpdates(role.NetworkLevelAccess, make(map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope), role.NetworkID.String())
	logic.ReturnSuccessResponseWithJson(w, r, nil, "deleted user role")
}

// @Summary     Attach user to a remote access gateway
// @Router      /api/users/{username}/remote_access_gw/{remote_access_gateway_id} [post]
// @Tags        PRO
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
	user, err := logic.GetUser(username)
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
	if user.PlatformRoleID == models.AdminRole || user.PlatformRoleID == models.SuperAdminRole {
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
	if user.RemoteGwIDs == nil {
		user.RemoteGwIDs = make(map[string]struct{})
	}
	user.RemoteGwIDs[node.ID.String()] = struct{}{}
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

	json.NewEncoder(w).Encode(logic.ToReturnUser(*user))
}

// @Summary     Remove user from a remote access gateway
// @Router      /api/users/{username}/remote_access_gw/{remote_access_gateway_id} [delete]
// @Tags        PRO
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
	user, err := logic.GetUser(username)
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
	delete(user.RemoteGwIDs, remoteGwID)
	go func(user models.User, remoteGwID string) {
		extclients, err := logic.GetAllExtClients()
		if err != nil {
			slog.Error("failed to fetch extclients", "error", err)
			return
		}
		for _, extclient := range extclients {
			if extclient.OwnerID == user.UserName && remoteGwID == extclient.IngressGatewayID {
				err = logic.DeleteExtClientAndCleanup(extclient)
				if err != nil {
					slog.Error("failed to delete extclient",
						"id", extclient.ClientID, "owner", user.UserName, "error", err)
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
	}(*user, remoteGwID)

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
	json.NewEncoder(w).Encode(logic.ToReturnUser(*user))
}

// @Summary     Get Users Remote Access Gw Networks.
// @Router      /api/users/{username}/remote_access_gw [get]
// @Tags        Users
// @Param       username path string true "Username to fetch all the gateways with access"
// @Success     200 {object} map[string][]models.UserRemoteGws
// @Failure     500 {object} models.ErrorResponse
func getUserRemoteAccessNetworks(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	username := r.Header.Get("user")
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch user %s, error: %v", username, err), "badrequest"))
		return
	}
	userGws := make(map[string][]models.UserRemoteGws)
	networks := []models.Network{}
	networkMap := make(map[string]struct{})
	userGwNodes := proLogic.GetUserRAGNodes(*user)
	for _, node := range userGwNodes {
		network, err := logic.GetNetwork(node.Network)
		if err != nil {
			slog.Error("failed to get node network", "error", err)
			continue
		}
		if _, ok := networkMap[network.NetID]; ok {
			continue
		}
		networkMap[network.NetID] = struct{}{}
		networks = append(networks, network)
	}

	slog.Debug("returned user gws", "user", username, "gws", userGws)
	logic.ReturnSuccessResponseWithJson(w, r, networks, "fetched user accessible networks")
}

// @Summary     Get Users Remote Access Gw Networks.
// @Router      /api/users/{username}/remote_access_gw [get]
// @Tags        Users
// @Param       username path string true "Username to fetch all the gateways with access"
// @Success     200 {object} map[string][]models.UserRemoteGws
// @Failure     500 {object} models.ErrorResponse
func getUserRemoteAccessNetworkGateways(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	username := r.Header.Get("user")
	user, err := logic.GetUser(username)
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

	userGwNodes := proLogic.GetUserRAGNodes(*user)
	for _, node := range userGwNodes {
		if node.Network != network {
			continue
		}

		host, err := logic.GetHost(node.HostID.String())
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

// @Summary     Get Users Remote Access Gw Networks.
// @Router      /api/users/{username}/remote_access_gw [get]
// @Tags        Users
// @Param       username path string true "Username to fetch all the gateways with access"
// @Success     200 {object} map[string][]models.UserRemoteGws
// @Failure     500 {object} models.ErrorResponse
func getRemoteAccessGatewayConf(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	username := r.Header.Get("user")
	user, err := logic.GetUser(username)
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

	userGwNodes := proLogic.GetUserRAGNodes(*user)
	if _, ok := userGwNodes[remoteGwID]; !ok {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("access denied"), "forbidden"))
		return
	}
	node, err := logic.GetNodeByID(remoteGwID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch gw node %s, error: %v", remoteGwID, err), "badrequest"))
		return
	}
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch gw host %s, error: %v", remoteGwID, err), "badrequest"))
		return
	}
	network, err := logic.GetNetwork(node.Network)
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
		if extClient.Network != network.NetID || extClient.IngressGatewayID != node.ID.String() {
			continue
		}
		if extClient.RemoteAccessClientID == req.RemoteAccessClientID && extClient.OwnerID == username {
			userConf = extClient
			userConf.AllowedIPs = logic.GetExtclientAllowedIPs(extClient)
		}
	}
	if userConf.ClientID == "" {
		// create a new conf
		userConf.OwnerID = user.UserName
		userConf.RemoteAccessClientID = req.RemoteAccessClientID
		userConf.IngressGatewayID = node.ID.String()

		// set extclient dns to ingressdns if extclient dns is not explicitly set
		if (userConf.DNS == "") && (node.IngressDNS != "") {
			userConf.DNS = node.IngressDNS
		}

		userConf.Network = node.Network
		host, err := logic.GetHost(node.HostID.String())
		if err != nil {
			logger.Log(0, r.Header.Get("user"),
				fmt.Sprintf("failed to get ingress gateway host for node [%s] info: %v", node.ID, err))
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		listenPort := logic.GetPeerListenPort(host)
		if host.EndpointIP.To4() == nil {
			userConf.IngressGatewayEndpoint = fmt.Sprintf("[%s]:%d", host.EndpointIPv6.String(), listenPort)
		} else {
			userConf.IngressGatewayEndpoint = fmt.Sprintf("%s:%d", host.EndpointIP.String(), listenPort)
		}
		userConf.Enabled = true
		parentNetwork, err := logic.GetNetwork(node.Network)
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
		DnsAddress:        node.IngressDNS,
		Addresses:         utils.NoEmptyStringToCsv(node.Address.String(), node.Address6.String()),
	}

	slog.Debug("returned user gw config", "user", user.UserName, "gws", userGw)
	logic.ReturnSuccessResponseWithJson(w, r, userGw, "fetched user config to gw "+remoteGwID)
}

// @Summary     Get Users Remote Access Gw.
// @Router      /api/users/{username}/remote_access_gw [get]
// @Tags        Users
// @Param       username path string true "Username to fetch all the gateways with access"
// @Success     200 {object} map[string][]models.UserRemoteGws
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
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch user %s, error: %v", username, err), "badrequest"))
		return
	}
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
	userGwNodes := proLogic.GetUserRAGNodes(*user)
	for _, extClient := range allextClients {
		node, ok := userGwNodes[extClient.IngressGatewayID]
		if !ok {
			continue
		}
		if extClient.RemoteAccessClientID == req.RemoteAccessClientID && extClient.OwnerID == username {

			host, err := logic.GetHost(node.HostID.String())
			if err != nil {
				continue
			}
			network, err := logic.GetNetwork(node.Network)
			if err != nil {
				slog.Error("failed to get node network", "error", err)
				continue
			}
			nodesWithStatus := logic.AddStatusToNodes([]models.Node{node}, false)
			if len(nodesWithStatus) > 0 {
				node = nodesWithStatus[0]
			}

			gws := userGws[node.Network]
			if extClient.DNS == "" {
				extClient.DNS = node.IngressDNS
			}
			extClient.AllowedIPs = logic.GetExtclientAllowedIPs(extClient)
			gws = append(gws, models.UserRemoteGws{
				GwID:              node.ID.String(),
				GWName:            host.Name,
				Network:           node.Network,
				GwClient:          extClient,
				Connected:         true,
				IsInternetGateway: node.IsInternetGateway,
				GwPeerPublicKey:   host.PublicKey.String(),
				GwListenPort:      logic.GetPeerListenPort(host),
				Metadata:          node.Metadata,
				AllowedEndpoints:  getAllowedRagEndpoints(&node, host),
				NetworkAddresses:  []string{network.AddressRange, network.AddressRange6},
				Status:            node.Status,
				DnsAddress:        node.IngressDNS,
				Addresses:         utils.NoEmptyStringToCsv(node.Address.String(), node.Address6.String()),
			})
			userGws[node.Network] = gws
			delete(userGwNodes, node.ID.String())
		}
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
		host, err := logic.GetHost(node.HostID.String())
		if err != nil {
			continue
		}
		nodesWithStatus := logic.AddStatusToNodes([]models.Node{node}, false)
		if len(nodesWithStatus) > 0 {
			node = nodesWithStatus[0]
		}
		network, err := logic.GetNetwork(node.Network)
		if err != nil {
			slog.Error("failed to get node network", "error", err)
		}
		gws := userGws[node.Network]

		gws = append(gws, models.UserRemoteGws{
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
			DnsAddress:        node.IngressDNS,
			Addresses:         utils.NoEmptyStringToCsv(node.Address.String(), node.Address6.String()),
		})
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

// @Summary     List users attached to an remote access gateway
// @Router      /api/nodes/{network}/{nodeid}/ingress/users [get]
// @Tags        PRO
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

func getAllowedRagEndpoints(ragNode *models.Node, ragHost *models.Host) []string {
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
// @Success     200 {array} models.User
// @Failure     500 {object} models.ErrorResponse
func getPendingUsers(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	users, err := logic.ListPendingUsers()
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
// @Param       username path string true "Username of the pending user to approve"
// @Success     200 {string} string
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
			if err = logic.CreateUser(&models.User{
				UserName:       user.UserName,
				Password:       newPass,
				PlatformRoleID: models.ServiceUser,
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
	logic.ReturnSuccessResponse(w, r, "approved "+username)
}

// @Summary     Delete a pending user
// @Router      /api/users_pending/user/{username} [delete]
// @Tags        Users
// @Param       username path string true "Username of the pending user to delete"
// @Success     200 {string} string
// @Failure     500 {object} models.ErrorResponse
func deletePendingUser(w http.ResponseWriter, r *http.Request) {
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
			err = logic.DeletePendingUser(username)
			if err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to delete pending user: %s", err), "internal"))
				return
			}
			break
		}
	}
	logic.ReturnSuccessResponse(w, r, "deleted pending "+username)
}

// @Summary     Delete all pending users
// @Router      /api/users_pending [delete]
// @Tags        Users
// @Success     200 {string} string
// @Failure     500 {object} models.ErrorResponse
func deleteAllPendingUsers(w http.ResponseWriter, r *http.Request) {
	// set header.
	err := database.DeleteAllRecords(database.PENDING_USERS_TABLE_NAME)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to delete all pending users "+err.Error()), "internal"))
		return
	}
	logic.ReturnSuccessResponse(w, r, "cleared all pending users")
}
