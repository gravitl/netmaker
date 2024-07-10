package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/auth"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/email"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

var (
	upgrader = websocket.Upgrader{}
)

func userHandlers(r *mux.Router) {
	r.HandleFunc("/api/users/adm/hassuperadmin", hasSuperAdmin).Methods(http.MethodGet)
	r.HandleFunc("/api/users/adm/createsuperadmin", createSuperAdmin).Methods(http.MethodPost)
	r.HandleFunc("/api/users/adm/transfersuperadmin/{username}", logic.SecurityCheck(true, http.HandlerFunc(transferSuperAdmin))).Methods(http.MethodPost)
	r.HandleFunc("/api/users/adm/authenticate", authenticateUser).Methods(http.MethodPost)
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(true, http.HandlerFunc(updateUser))).Methods(http.MethodPut)
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(true, checkFreeTierLimits(limitChoiceUsers, http.HandlerFunc(createUser)))).Methods(http.MethodPost)
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(true, http.HandlerFunc(deleteUser))).Methods(http.MethodDelete)
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(false, logic.ContinueIfUserMatch(http.HandlerFunc(getUser)))).Methods(http.MethodGet)
	r.HandleFunc("/api/users", logic.SecurityCheck(true, http.HandlerFunc(getUsers))).Methods(http.MethodGet)
	r.HandleFunc("/api/users_pending", logic.SecurityCheck(true, http.HandlerFunc(getPendingUsers))).Methods(http.MethodGet)
	r.HandleFunc("/api/users_pending", logic.SecurityCheck(true, http.HandlerFunc(deleteAllPendingUsers))).Methods(http.MethodDelete)
	r.HandleFunc("/api/users_pending/user/{username}", logic.SecurityCheck(true, http.HandlerFunc(deletePendingUser))).Methods(http.MethodDelete)
	r.HandleFunc("/api/users_pending/user/{username}", logic.SecurityCheck(true, http.HandlerFunc(approvePendingUser))).Methods(http.MethodPost)

	// User Role Handlers
	r.HandleFunc("/api/v1/users/roles", logic.SecurityCheck(true, http.HandlerFunc(listRoles))).Methods(http.MethodGet)
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
	groups, err := logic.ListUserGroups()
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
	var params = mux.Vars(r)
	gid := params["group_id"]
	if gid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("group id is required"), "badrequest"))
		return
	}
	group, err := logic.GetUserGroup(models.UserGroupID(gid))
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
	var userGroup models.UserGroup
	err := json.NewDecoder(r.Body).Decode(&userGroup)
	if err != nil {
		slog.Error("error decoding request body", "error",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = logic.ValidateCreateGroupReq(userGroup)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = logic.CreateUserGroup(userGroup)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, userGroup, "created user group")
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
	err = logic.ValidateUpdateGroupReq(userGroup)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = logic.UpdateUserGroup(userGroup)
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
	var params = mux.Vars(r)
	gid := params["group_id"]
	if gid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("role is required"), "badrequest"))
		return
	}
	err := logic.DeleteUserGroup(models.UserGroupID(gid))
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
	roles, err := logic.ListRoles()
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
	var params = mux.Vars(r)
	rid := params["role_id"]
	if rid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("role is required"), "badrequest"))
		return
	}
	role, err := logic.GetRole(models.UserRole(rid))
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
	err = logic.ValidateCreateRoleReq(userRole)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	userRole.Default = false
	userRole.GlobalLevelAccess = make(map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope)
	err = logic.CreateRole(userRole)
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
	err = logic.ValidateUpdateRoleReq(userRole)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	userRole.GlobalLevelAccess = make(map[models.RsrcType]map[models.RsrcID]models.RsrcPermissionScope)
	err = logic.UpdateRole(userRole)
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
	var params = mux.Vars(r)
	rid := params["role_id"]
	if rid == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("role is required"), "badrequest"))
		return
	}
	err := logic.DeleteRole(models.UserRole(rid))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, nil, "created user role")
}

// swagger:route POST /api/users/adm/authenticate authenticate authenticateUser
//
// User authenticates using its password and retrieves a JWT for authorization.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: successResponse
func authenticateUser(response http.ResponseWriter, request *http.Request) {

	// Auth request consists of Mac Address and Password (from node that is authorizing
	// in case of Master, auth is ignored and mac is set to "mastermac"
	var authRequest models.UserAuthParams
	var errorResponse = models.ErrorResponse{
		Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
	}

	if !servercfg.IsBasicAuthEnabled() {
		logic.ReturnErrorResponse(response, request, logic.FormatError(fmt.Errorf("basic auth is disabled"), "badrequest"))
		return
	}

	decoder := json.NewDecoder(request.Body)
	decoderErr := decoder.Decode(&authRequest)
	defer request.Body.Close()
	if decoderErr != nil {
		logger.Log(0, "error decoding request body: ",
			decoderErr.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	if val := request.Header.Get("From-Ui"); val == "true" {
		// request came from UI, if normal user block Login
		user, err := logic.GetUser(authRequest.UserName)
		if err != nil {
			logger.Log(0, authRequest.UserName, "user validation failed: ",
				err.Error())
			logic.ReturnErrorResponse(response, request, logic.FormatError(err, "unauthorized"))
			return
		}
		role, err := logic.GetRole(user.PlatformRoleID)
		if err != nil {
			logic.ReturnErrorResponse(response, request, logic.FormatError(errors.New("access denied to dashboard"), "unauthorized"))
			return
		}
		if role.DenyDashboardAccess {
			logic.ReturnErrorResponse(response, request, logic.FormatError(errors.New("access denied to dashboard"), "unauthorized"))
			return
		}
	}
	username := authRequest.UserName
	jwt, err := logic.VerifyAuthRequest(authRequest)
	if err != nil {
		logger.Log(0, username, "user validation failed: ",
			err.Error())
		logic.ReturnErrorResponse(response, request, logic.FormatError(err, "badrequest"))
		return
	}

	if jwt == "" {
		// very unlikely that err is !nil and no jwt returned, but handle it anyways.
		logger.Log(0, username, "jwt token is empty")
		logic.ReturnErrorResponse(response, request, logic.FormatError(errors.New("no token returned"), "internal"))
		return
	}

	var successResponse = models.SuccessResponse{
		Code:    http.StatusOK,
		Message: "W1R3: Device " + username + " Authorized",
		Response: models.SuccessfulUserLoginResponse{
			AuthToken: jwt,
			UserName:  username,
		},
	}
	// Send back the JWT
	successJSONResponse, jsonError := json.Marshal(successResponse)
	if jsonError != nil {
		logger.Log(0, username,
			"error marshalling resp: ", jsonError.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	logger.Log(2, username, "was authenticated")
	response.Header().Set("Content-Type", "application/json")
	response.Write(successJSONResponse)

	go func() {
		if servercfg.IsPro && servercfg.GetRacAutoDisable() {
			// enable all associeated clients for the user
			clients, err := logic.GetAllExtClients()
			if err != nil {
				slog.Error("error getting clients: ", "error", err)
				return
			}
			for _, client := range clients {
				if client.OwnerID == username && !client.Enabled {
					slog.Info(fmt.Sprintf("enabling ext client %s for user %s due to RAC autodisabling feature", client.ClientID, client.OwnerID))
					if newClient, err := logic.ToggleExtClientConnectivity(&client, true); err != nil {
						slog.Error("error enabling ext client in RAC autodisable hook", "error", err)
						continue // dont return but try for other clients
					} else {
						// publish peer update to ingress gateway
						if ingressNode, err := logic.GetNodeByID(newClient.IngressGatewayID); err == nil {
							if err = mq.PublishPeerUpdate(false); err != nil {
								slog.Error("error updating ext clients on", "ingress", ingressNode.ID.String(), "err", err.Error())
							}
						}
					}
				}
			}
		}
	}()
}

// swagger:route GET /api/users/adm/hassuperadmin user hasSuperAdmin
//
// Checks whether the server has an admin.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: hasAdmin
func hasSuperAdmin(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	hasSuperAdmin, err := logic.HasSuperAdmin()
	if err != nil {
		logger.Log(0, "failed to check for admin: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	json.NewEncoder(w).Encode(hasSuperAdmin)

}

// swagger:route GET /api/users/{username} user getUser
//
// Get an individual user.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func getUser(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	usernameFetched := params["username"]
	user, err := logic.GetReturnUser(usernameFetched)

	if err != nil {
		logger.Log(0, usernameFetched, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "fetched user", usernameFetched)
	json.NewEncoder(w).Encode(user)
}

// swagger:route GET /api/users user getUsers
//
// Get all users.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func getUsers(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	users, err := logic.GetUsers()

	if err != nil {
		logger.Log(0, "failed to fetch users: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logic.SortUsers(users[:])
	logger.Log(2, r.Header.Get("user"), "fetched users")
	json.NewEncoder(w).Encode(users)
}

// swagger:route POST /api/users/adm/createsuperadmin user createAdmin
//
// Make a user an admin.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func createSuperAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var u models.User

	err := json.NewDecoder(r.Body).Decode(&u)
	if err != nil {
		slog.Error("error decoding request body", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if !servercfg.IsBasicAuthEnabled() {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("basic auth is disabled"), "badrequest"))
		return
	}

	err = logic.CreateSuperAdmin(&u)
	if err != nil {
		slog.Error("failed to create admin", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, u.UserName, "was made a super admin")
	json.NewEncoder(w).Encode(logic.ToReturnUser(u))
}

// swagger:route POST /api/users/adm/transfersuperadmin user transferSuperAdmin
//
// Transfers superadmin role to an admin user.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func transferSuperAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	caller, err := logic.GetUser(r.Header.Get("user"))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
	}
	if caller.PlatformRoleID != models.SuperAdminRole {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("only superadmin can assign the superadmin role to another user"), "forbidden"))
		return
	}
	var params = mux.Vars(r)
	username := params["username"]
	u, err := logic.GetUser(username)
	if err != nil {
		slog.Error("error getting user", "user", u.UserName, "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if u.PlatformRoleID != models.AdminRole {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("only admins can be promoted to superadmin role"), "forbidden"))
		return
	}
	if !servercfg.IsBasicAuthEnabled() {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("basic auth is disabled"), "badrequest"))
		return
	}

	u.PlatformRoleID = models.SuperAdminRole
	err = logic.UpsertUser(*u)
	if err != nil {
		slog.Error("error updating user to superadmin: ", "user", u.UserName, "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	caller.PlatformRoleID = models.AdminRole
	err = logic.UpsertUser(*caller)
	if err != nil {
		slog.Error("error demoting user to admin: ", "user", caller.UserName, "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	slog.Info("user was made a super admin", "user", u.UserName)
	json.NewEncoder(w).Encode(logic.ToReturnUser(*u))
}

// swagger:route POST /api/users/{username} user createUser
//
// Create a user.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func createUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	caller, err := logic.GetUser(r.Header.Get("user"))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
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
	uniqueGroupsPlatformRole := make(map[models.UserRole]struct{})
	for groupID := range user.UserGroups {
		userG, err := logic.GetUserGroup(groupID)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
		uniqueGroupsPlatformRole[userG.PlatformRole] = struct{}{}
	}
	if len(uniqueGroupsPlatformRole) > 1 {
		err = errors.New("only groups with same platform role can be assigned to an user")
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	userRole, err := logic.GetRole(user.PlatformRoleID)
	if err != nil {
		err = errors.New("error fetching role " + user.PlatformRoleID.String() + " " + err.Error())
		slog.Error("error creating new user: ", "user", user.UserName, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if userRole.ID == models.SuperAdminRole {
		err = errors.New("additional superadmins cannot be created")
		slog.Error("error creating new user: ", "user", user.UserName, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "forbidden"))
		return
	}

	if caller.PlatformRoleID != models.SuperAdminRole && user.PlatformRoleID == models.AdminRole {
		err = errors.New("only superadmin can create admin users")
		slog.Error("error creating new user: ", "user", user.UserName, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "forbidden"))
		return
	}

	if !servercfg.IsPro && user.PlatformRoleID != models.AdminRole {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("non-admins users can only be created on Pro version"), "forbidden"))
		return
	}
	err = logic.CreateUser(&user)
	if err != nil {
		slog.Error("error creating new user: ", "user", user.UserName, "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logic.DeleteUserInvite(user.UserName)
	logic.DeletePendingUser(user.UserName)
	slog.Info("user was created", "username", user.UserName)
	json.NewEncoder(w).Encode(logic.ToReturnUser(user))
}

// swagger:route PUT /api/users/{username} user updateUser
//
// Update a user.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func updateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	// start here
	var caller *models.User
	var err error
	var ismaster bool
	if r.Header.Get("user") == logic.MasterUser {
		ismaster = true
	} else {
		caller, err = logic.GetUser(r.Header.Get("user"))
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		}
	}

	username := params["username"]
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username,
			"failed to update user info: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	var userchange models.User
	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&userchange)
	if err != nil {
		slog.Error("failed to decode body", "error ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if user.UserName != userchange.UserName {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user in param and request body not matching"), "badrequest"))
		return
	}
	selfUpdate := false
	if !ismaster && caller.UserName == user.UserName {
		selfUpdate = true
	}

	if !ismaster && !selfUpdate {
		if caller.PlatformRoleID == models.AdminRole && user.PlatformRoleID == models.SuperAdminRole {
			slog.Error("non-superadmin user", "caller", caller.UserName, "attempted to update superadmin user", username)
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("cannot update superadmin user"), "forbidden"))
			return
		}
		if caller.PlatformRoleID != models.AdminRole && caller.PlatformRoleID != models.SuperAdminRole {
			slog.Error("operation not allowed", "caller", caller.UserName, "attempted to update user", username)
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("cannot update superadmin user"), "forbidden"))
			return
		}
		if caller.PlatformRoleID == models.AdminRole && user.PlatformRoleID == models.AdminRole {
			slog.Error("admin user cannot update another admin", "caller", caller.UserName, "attempted to update admin user", username)
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("admin user cannot update another admin"), "forbidden"))
			return
		}
		if caller.PlatformRoleID == models.AdminRole && userchange.PlatformRoleID == models.AdminRole {
			err = errors.New("admin user cannot update role of an another user to admin")
			slog.Error("failed to update user", "caller", caller.UserName, "attempted to update user", username, "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "forbidden"))
			return
		}

	}
	if !ismaster && selfUpdate {
		if user.PlatformRoleID != userchange.PlatformRoleID {
			slog.Error("user cannot change his own role", "caller", caller.UserName, "attempted to update user role", username)
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user not allowed to self assign role"), "forbidden"))
			return

		}
	}
	if ismaster {
		if user.PlatformRoleID != models.SuperAdminRole && userchange.PlatformRoleID == models.SuperAdminRole {
			slog.Error("operation not allowed", "caller", logic.MasterUser, "attempted to update user role to superadmin", username)
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("attempted to update user role to superadmin"), "forbidden"))
			return
		}
	}

	if auth.IsOauthUser(user) == nil && userchange.Password != "" {
		err := fmt.Errorf("cannot update user's password for an oauth user %s", username)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "forbidden"))
		return
	}

	user, err = logic.UpdateUser(&userchange, user)
	if err != nil {
		logger.Log(0, username,
			"failed to update user info: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, username, "was updated")
	json.NewEncoder(w).Encode(logic.ToReturnUser(*user))
}

// swagger:route DELETE /api/users/{username} user deleteUser
//
// Delete a user.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: userBodyResponse
func deleteUser(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)
	caller, err := logic.GetUser(r.Header.Get("user"))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
	}
	callerUserRole, err := logic.GetRole(caller.PlatformRoleID)
	if err != nil {
		slog.Error("failed to get role ", "role", callerUserRole.ID, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	username := params["username"]
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username,
			"failed to update user info: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	userRole, err := logic.GetRole(user.PlatformRoleID)
	if err != nil {
		slog.Error("failed to get role ", "role", userRole.ID, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if userRole.ID == models.SuperAdminRole {
		slog.Error(
			"failed to delete user: ", "user", username, "error", "superadmin cannot be deleted")
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("superadmin cannot be deleted"), "internal"))
		return
	}
	if callerUserRole.ID != models.SuperAdminRole {
		if callerUserRole.ID == models.AdminRole && userRole.ID == models.AdminRole {
			slog.Error(
				"failed to delete user: ", "user", username, "error", "admin cannot delete another admin user, including oneself")
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("admin cannot delete another admin user, including oneself"), "internal"))
			return
		}
	}
	success, err := logic.DeleteUser(username)
	if err != nil {
		logger.Log(0, username,
			"failed to delete user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	} else if !success {
		err := errors.New("delete unsuccessful")
		logger.Log(0, username, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	// check and delete extclient with this ownerID
	go func() {
		extclients, err := logic.GetAllExtClients()
		if err != nil {
			slog.Error("failed to get extclients", "error", err)
			return
		}
		for _, extclient := range extclients {
			if extclient.OwnerID == user.UserName {
				err = logic.DeleteExtClient(extclient.Network, extclient.ClientID)
				if err != nil {
					slog.Error("failed to delete extclient",
						"id", extclient.ClientID, "owner", user.UserName, "error", err)
				}
			}
		}
		if servercfg.IsDNSMode() {
			logic.SetDNS()
		}
	}()
	logger.Log(1, username, "was deleted")
	json.NewEncoder(w).Encode(params["username"] + " deleted.")
}

// Called when vpn client dials in to start the auth flow and first stage is to get register URL itself
func socketHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade our raw HTTP connection to a websocket based one
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log(0, "error during connection upgrade for node sign-in:", err.Error())
		return
	}
	if conn == nil {
		logger.Log(0, "failed to establish web-socket connection during node sign-in")
		return
	}
	// Start handling the session
	go auth.SessionHandler(conn)
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
			var newPass, fetchErr = auth.FetchPassValue("")
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
	code, _ := url.QueryUnescape(r.URL.Query().Get("code"))
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

	for _, inviteGroupID := range in.Groups {
		userG, err := logic.GetUserGroup(inviteGroupID)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("error fetching group id "+inviteGroupID.String()), "badrequest"))
			return
		}
		user.PlatformRoleID = userG.PlatformRole
		user.UserGroups[inviteGroupID] = struct{}{}
	}
	if user.PlatformRoleID == "" {
		user.PlatformRoleID = models.ServiceUser
	}
	user.NetworkRoles = make(map[models.NetworkID]map[models.UserRole]struct{})
	err = logic.CreateUser(&user)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	// delete invite
	logic.DeleteUserInvite(email)
	logic.DeletePendingUser(email)
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
	code, _ := url.QueryUnescape(r.URL.Query().Get("code"))
	err := logic.ValidateAndApproveUserInvite(email, code)
	if err != nil {
		logger.Log(0, "failed to fetch users: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	http.Redirect(w, r, fmt.Sprintf("%s/login?email=%s&code=%s", servercfg.GetFrontendURL(), email, code), http.StatusTemporaryRedirect)
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
	uniqueGroupsPlatformRole := make(map[models.UserRole]struct{})
	for _, groupID := range inviteReq.Groups {
		userG, err := logic.GetUserGroup(groupID)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
		uniqueGroupsPlatformRole[userG.PlatformRole] = struct{}{}
	}
	if len(uniqueGroupsPlatformRole) > 1 {
		err = errors.New("only groups with same platform role can be assigned to an user")
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
			Email:      inviteeEmail,
			Groups:     inviteReq.Groups,
			InviteCode: logic.RandomString(8),
		}
		err = logic.InsertUserInvite(invite)
		if err != nil {
			slog.Error("failed to insert invite for user", "email", invite.Email, "error", err)
		}
		// notify user with magic link
		go func(invite models.UserInvite) {
			// Set E-Mail body. You can set plain text or html with text/html
			u, err := url.Parse(fmt.Sprintf("https://api.%s/api/v1/users/invite?email=%s&code=%s",
				servercfg.GetServer(), url.QueryEscape(invite.Email), url.QueryEscape(invite.InviteCode)))
			if err != nil {
				slog.Error("failed to parse to invite url", "error", err)
				return
			}
			e := email.UserInvitedMail{
				BodyBuilder: &email.EmailBodyBuilderWithH1HeadlineAndImage{},
				InviteURL:   u.String(),
			}
			n := email.Notification{
				RecipientMail: invite.Email,
			}
			err = email.Send(n.NewEmailSender(e))
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
	var params = mux.Vars(r)
	username := params["invitee_email"]
	err := logic.DeleteUserInvite(username)
	if err != nil {
		logger.Log(0, "failed to delete user invite: ", username, err.Error())
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
