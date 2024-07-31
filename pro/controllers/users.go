package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

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
	"golang.org/x/exp/slog"
)

func UserHandlers(r *mux.Router) {
	r.HandleFunc("/api/users/{username}/remote_access_gw/{remote_access_gateway_id}", logic.SecurityCheck(true, http.HandlerFunc(attachUserToRemoteAccessGw))).Methods(http.MethodPost)
	r.HandleFunc("/api/users/{username}/remote_access_gw/{remote_access_gateway_id}", logic.SecurityCheck(true, http.HandlerFunc(removeUserFromRemoteAccessGW))).Methods(http.MethodDelete)
	r.HandleFunc("/api/users/{username}/remote_access_gw", logic.SecurityCheck(false, logic.ContinueIfUserMatch(http.HandlerFunc(getUserRemoteAccessGwsV1)))).Methods(http.MethodGet)
	r.HandleFunc("/api/users/ingress/{ingress_id}", logic.SecurityCheck(true, http.HandlerFunc(ingressGatewayUsers))).Methods(http.MethodGet)
	r.HandleFunc("/api/oauth/login", proAuth.HandleAuthLogin).Methods(http.MethodGet)
	r.HandleFunc("/api/oauth/callback", proAuth.HandleAuthCallback).Methods(http.MethodGet)
	r.HandleFunc("/api/oauth/headless", proAuth.HandleHeadlessSSO)
	r.HandleFunc("/api/oauth/register/{regKey}", proAuth.RegisterHostSSO).Methods(http.MethodGet)

	// User Role Handlers
	r.HandleFunc("/api/v1/users/roles", logic.SecurityCheck(true, http.HandlerFunc(listRoles))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/role", getRole).Methods(http.MethodGet)
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
	email, _ := url.QueryUnescape(r.URL.Query().Get("email"))
	code, _ := url.QueryUnescape(r.URL.Query().Get("invite_code"))
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
	email, _ := url.QueryUnescape(r.URL.Query().Get("email"))
	code, _ := url.QueryUnescape(r.URL.Query().Get("invite_code"))
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

	for _, inviteeEmail := range inviteReq.UserEmails {
		// check if user with email exists, then ignore
		_, err := logic.GetUser(inviteeEmail)
		if err == nil {
			// user exists already, so ignore
			continue
		}
		invite := models.UserInvite{
			Email:        inviteeEmail,
			UserGroups:   inviteReq.UserGroups,
			NetworkRoles: inviteReq.NetworkRoles,
			InviteCode:   logic.RandomString(8),
		}
		u, err := url.Parse(fmt.Sprintf("%s/invite?email=%s&invite_code=%s",
			servercfg.GetFrontendURL(), url.QueryEscape(invite.Email), url.QueryEscape(invite.InviteCode)))
		if err != nil {
			slog.Error("failed to parse to invite url", "error", err)
			return
		}
		invite.InviteURL = u.String()
		err = logic.InsertUserInvite(invite)
		if err != nil {
			slog.Error("failed to insert invite for user", "email", invite.Email, "error", err)
		}
		// notify user with magic link
		go func(invite models.UserInvite) {
			// Set E-Mail body. You can set plain text or html with text/html

			e := email.UserInvitedMail{
				BodyBuilder: &email.EmailBodyBuilderWithH1HeadlineAndImage{},
				InviteURL:   invite.InviteURL,
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
	email, _ := url.QueryUnescape(r.URL.Query().Get("invitee_email"))
	err := logic.DeleteUserInvite(email)
	if err != nil {
		logger.Log(0, "failed to delete user invite: ", email, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
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

	gid, _ := url.QueryUnescape(r.URL.Query().Get("group_id"))
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
func deleteUserGroup(w http.ResponseWriter, r *http.Request) {

	gid, _ := url.QueryUnescape(r.URL.Query().Get("group_id"))
	if gid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("role is required"), "badrequest"))
		return
	}
	err := proLogic.DeleteUserGroup(models.UserGroupID(gid))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, nil, "deleted user group")
}

// swagger:route GET /api/v1/user/roles user listRoles
//
// lists all user roles.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func listRoles(w http.ResponseWriter, r *http.Request) {
	platform, _ := url.QueryUnescape(r.URL.Query().Get("platform"))
	var roles []models.UserRolePermissionTemplate
	var err error
	if platform == "true" {
		roles, err = proLogic.ListPlatformRoles()
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

// swagger:route GET /api/v1/user/role user getRole
//
// Get user role permission templates.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func getRole(w http.ResponseWriter, r *http.Request) {
	rid, _ := url.QueryUnescape(r.URL.Query().Get("role_id"))
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

// swagger:route POST /api/v1/user/role user createRole
//
// Create user role permission template.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func createRole(w http.ResponseWriter, r *http.Request) {
	var userRole models.UserRolePermissionTemplate
	err := json.NewDecoder(r.Body).Decode(&userRole)
	if err != nil {
		slog.Error("error decoding request body", "error",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = proLogic.ValidateCreateRoleReq(userRole)
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
	logic.ReturnSuccessResponseWithJson(w, r, userRole, "created user role")
}

// swagger:route PUT /api/v1/user/role user updateRole
//
// Update user role permission template.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func updateRole(w http.ResponseWriter, r *http.Request) {
	var userRole models.UserRolePermissionTemplate
	err := json.NewDecoder(r.Body).Decode(&userRole)
	if err != nil {
		slog.Error("error decoding request body", "error",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = proLogic.ValidateUpdateRoleReq(userRole)
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
	logic.ReturnSuccessResponseWithJson(w, r, userRole, "updated user role")
}

// swagger:route DELETE /api/v1/user/role user deleteRole
//
// Delete user role permission template.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func deleteRole(w http.ResponseWriter, r *http.Request) {

	rid, _ := url.QueryUnescape(r.URL.Query().Get("role_id"))
	if rid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("role is required"), "badrequest"))
		return
	}
	err := proLogic.DeleteRole(models.UserRoleID(rid))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, nil, "created user role")
}

// swagger:route POST /api/users/{username}/remote_access_gw user attachUserToRemoteAccessGateway
//
// Attach User to a remote access gateway.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func attachUserToRemoteAccessGw(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	username := params["username"]
	remoteGwID := params["remote_access_gateway_id"]
	if username == "" || remoteGwID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("required params `username` and `remote_access_gateway_id`"), "badrequest"))
		return
	}
	user, err := logic.GetUser(username)
	if err != nil {
		slog.Error("failed to fetch user: ", "username", username, "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch user %s, error: %v", username, err), "badrequest"))
		return
	}
	if user.PlatformRoleID == models.AdminRole || user.PlatformRoleID == models.SuperAdminRole {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("superadmins/admins have access to all gateways"), "badrequest"))
		return
	}
	node, err := logic.GetNodeByID(remoteGwID)
	if err != nil {
		slog.Error("failed to fetch gateway node", "nodeID", remoteGwID, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch remote access gateway node, error: %v", err), "badrequest"))
		return
	}
	if !node.IsIngressGateway {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("node is not a remote access gateway"), "badrequest"))
		return
	}
	if user.RemoteGwIDs == nil {
		user.RemoteGwIDs = make(map[string]struct{})
	}
	user.RemoteGwIDs[node.ID.String()] = struct{}{}
	err = logic.UpsertUser(*user)
	if err != nil {
		slog.Error("failed to update user's gateways", "user", username, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch remote access gateway node,error: %v", err), "badrequest"))
		return
	}

	json.NewEncoder(w).Encode(logic.ToReturnUser(*user))
}

// swagger:route DELETE /api/users/{username}/remote_access_gw user removeUserFromRemoteAccessGW
//
// Delete User from a remote access gateway.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func removeUserFromRemoteAccessGW(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	username := params["username"]
	remoteGwID := params["remote_access_gateway_id"]
	if username == "" || remoteGwID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("required params `username` and `remote_access_gateway_id`"), "badrequest"))
		return
	}
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch user %s, error: %v", username, err), "badrequest"))
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
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to fetch remote access gaetway node "+err.Error()), "badrequest"))
		return
	}
	json.NewEncoder(w).Encode(logic.ToReturnUser(*user))
}

func getUserRemoteAccessGwsV1(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	logger.Log(0, "------------> 1. getUserRemoteAccessGwsV1")
	var params = mux.Vars(r)
	username := params["username"]
	if username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("required params username"), "badrequest"))
		return
	}
	logger.Log(0, "------------> 2. getUserRemoteAccessGwsV1")
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch user %s, error: %v", username, err), "badrequest"))
		return
	}
	logger.Log(0, "------------> 3. getUserRemoteAccessGwsV1")
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
	logger.Log(0, "------------> 4. getUserRemoteAccessGwsV1")
	reqFromMobile := r.URL.Query().Get("from_mobile") == "true"
	if req.RemoteAccessClientID == "" && remoteAccessClientID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("remote access client id cannot be empty"), "badrequest"))
		return
	}
	if req.RemoteAccessClientID == "" {
		req.RemoteAccessClientID = remoteAccessClientID
	}
	userGws := make(map[string][]models.UserRemoteGws)
	logger.Log(0, "------------> 5. getUserRemoteAccessGwsV1")
	allextClients, err := logic.GetAllExtClients()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(0, "------------> 6. getUserRemoteAccessGwsV1")
	userGwNodes := proLogic.GetUserRAGNodes(*user)
	logger.Log(0, fmt.Sprintf("1. User Gw Nodes: %+v", userGwNodes))
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
			}

			gws := userGws[node.Network]
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
			})
			userGws[node.Network] = gws
			delete(userGwNodes, node.ID.String())
		}
	}
	logger.Log(0, fmt.Sprintf("2. User Gw Nodes: %+v", userGwNodes))
	// add remaining gw nodes to resp
	for gwID := range userGwNodes {
		logger.Log(0, "RAG ---> 1")
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
		logger.Log(0, "RAG ---> 2")
		host, err := logic.GetHost(node.HostID.String())
		if err != nil {
			continue
		}
		network, err := logic.GetNetwork(node.Network)
		if err != nil {
			slog.Error("failed to get node network", "error", err)
		}
		logger.Log(0, "RAG ---> 3")
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

// swagger:route GET "/api/users/{username}/remote_access_gw" nodes getUserRemoteAccessGws
//
// Get an individual node.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func getUserRemoteAccessGws(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	username := params["username"]
	if username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("required params username"), "badrequest"))
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
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to fetch user %s, error: %v", username, err), "badrequest"))
		return
	}
	allextClients, err := logic.GetAllExtClients()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	processedAdminNodeIds := make(map[string]struct{})
	for _, extClient := range allextClients {
		if extClient.RemoteAccessClientID == req.RemoteAccessClientID && extClient.OwnerID == username {
			node, err := logic.GetNodeByID(extClient.IngressGatewayID)
			if err != nil {
				continue
			}
			if node.PendingDelete {
				continue
			}
			if !node.IsIngressGateway {
				continue
			}
			host, err := logic.GetHost(node.HostID.String())
			if err != nil {
				continue
			}
			network, err := logic.GetNetwork(node.Network)
			if err != nil {
				slog.Error("failed to get node network", "error", err)
			}

			if _, ok := user.RemoteGwIDs[node.ID.String()]; (user.PlatformRoleID != models.AdminRole && user.PlatformRoleID != models.SuperAdminRole) && ok {
				gws := userGws[node.Network]
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
				})
				userGws[node.Network] = gws
				delete(user.RemoteGwIDs, node.ID.String())
			} else {
				gws := userGws[node.Network]
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
				})
				userGws[node.Network] = gws
				processedAdminNodeIds[node.ID.String()] = struct{}{}
			}
		}
	}

	// add remaining gw nodes to resp
	if user.PlatformRoleID != models.AdminRole && user.PlatformRoleID != models.SuperAdminRole {
		for gwID := range user.RemoteGwIDs {
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
			})
			userGws[node.Network] = gws
		}
	} else {
		allNodes, err := logic.GetAllNodes()
		if err != nil {
			slog.Error("failed to fetch all nodes", "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		for _, node := range allNodes {
			_, ok := processedAdminNodeIds[node.ID.String()]
			if node.IsIngressGateway && !node.PendingDelete && !ok {
				host, err := logic.GetHost(node.HostID.String())
				if err != nil {
					slog.Error("failed to fetch host", "error", err)
					continue
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
				})
				userGws[node.Network] = gws
			}
		}
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

// swagger:route GET /api/nodes/{network}/{nodeid}/ingress/users users ingressGatewayUsers
//
// Lists all the users attached to an ingress gateway.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
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
		slog.Error("failed to get users on ingress gateway", "nodeid", ingressID, "network", node.Network, "user", r.Header.Get("user"),
			"error", err)
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

// swagger:route GET /api/users_pending user getPendingUsers
//
// Get all pending users.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
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

// swagger:route POST /api/users_pending/user/{username} user approvePendingUser
//
// approve pending user.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
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
				UserName: user.UserName,
				Password: newPass,
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

// swagger:route DELETE /api/users_pending/user/{username} user deletePendingUser
//
// delete pending user.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
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

// swagger:route DELETE /api/users_pending/{username}/pending user deleteAllPendingUsers
//
// delete all pending users.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func deleteAllPendingUsers(w http.ResponseWriter, r *http.Request) {
	// set header.
	err := database.DeleteAllRecords(database.PENDING_USERS_TABLE_NAME)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to delete all pending users "+err.Error()), "internal"))
		return
	}
	logic.ReturnSuccessResponse(w, r, "cleared all pending users")
}
