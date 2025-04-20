package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gravitl/netmaker/schema"
	"net/http"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/auth"
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

var ListRoles = listRoles

func userHandlers(r *mux.Router) {
	r.HandleFunc("/api/users/adm/hassuperadmin", hasSuperAdmin).Methods(http.MethodGet)
	r.HandleFunc("/api/users/adm/createsuperadmin", createSuperAdmin).Methods(http.MethodPost)
	r.HandleFunc("/api/users/adm/transfersuperadmin/{username}", logic.SecurityCheck(true, http.HandlerFunc(transferSuperAdmin))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/users/adm/authenticate", authenticateUser).Methods(http.MethodPost)
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(true, http.HandlerFunc(updateUser))).Methods(http.MethodPut)
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(true, checkFreeTierLimits(limitChoiceUsers, http.HandlerFunc(createUser)))).Methods(http.MethodPost)
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(true, http.HandlerFunc(deleteUser))).Methods(http.MethodDelete)
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(false, logic.ContinueIfUserMatch(http.HandlerFunc(getUser)))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users", logic.SecurityCheck(false, logic.ContinueIfUserMatch(http.HandlerFunc(getUserV1)))).Methods(http.MethodGet)
	r.HandleFunc("/api/users", logic.SecurityCheck(true, http.HandlerFunc(getUsers))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/roles", logic.SecurityCheck(true, http.HandlerFunc(ListRoles))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/access_token", logic.SecurityCheck(true, http.HandlerFunc(createUserAccessToken))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/users/access_token", logic.SecurityCheck(true, http.HandlerFunc(getUserAccessTokens))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/access_token", logic.SecurityCheck(true, http.HandlerFunc(deleteUserAccessTokens))).Methods(http.MethodDelete)
}

// @Summary     Authenticate a user to retrieve an authorization token
// @Router      /api/v1/users/{username}/access_token [post]
// @Tags        Auth
// @Accept      json
// @Param       body body models.UserAuthParams true "Authentication parameters"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func createUserAccessToken(w http.ResponseWriter, r *http.Request) {

	// Auth request consists of Mac Address and Password (from node that is authorizing
	// in case of Master, auth is ignored and mac is set to "mastermac"
	var req schema.UserAccessToken

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if req.Name == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("name is required"), "badrequest"))
		return
	}
	if req.UserName == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("username is required"), "badrequest"))
		return
	}

	user, err := logic.GetUser(req.UserName)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "unauthorized"))
		return
	}
	if logic.IsOauthUser(user) == nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user is registered via SSO"), "badrequest"))
		return
	}
	req.ID = uuid.New().String()
	req.CreatedBy = r.Header.Get("user")
	req.CreatedAt = time.Now()
	jwt, err := logic.CreateUserAccessJwtToken(user.UserName, user.PlatformRoleID, req.ExpiresAt, req.ID)
	if jwt == "" {
		// very unlikely that err is !nil and no jwt returned, but handle it anyways.
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error creating access token "+err.Error()), "internal"),
		)
		return
	}
	err = req.Create(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error creating access token "+err.Error()), "internal"),
		)
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, models.SuccessfulUserLoginResponse{
		AuthToken: jwt,
		UserName:  req.UserName,
	}, "api access token has generated for user "+req.UserName)
}

// @Summary     Authenticate a user to retrieve an authorization token
// @Router      /api/v1/users/{username}/access_token [post]
// @Tags        Auth
// @Accept      json
// @Param       body body models.UserAuthParams true "Authentication parameters"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func getUserAccessTokens(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("username is required"), "badrequest"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, (&schema.UserAccessToken{UserName: username}).ListByUser(r.Context()), "fetched api access tokens for user "+username)
}

// @Summary     Authenticate a user to retrieve an authorization token
// @Router      /api/v1/users/{username}/access_token [post]
// @Tags        Auth
// @Accept      json
// @Param       body body models.UserAuthParams true "Authentication parameters"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func deleteUserAccessTokens(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("id is required"), "badrequest"))
		return
	}

	err := (&schema.UserAccessToken{ID: id}).Delete(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("error deleting access token "+err.Error()), "internal"),
		)
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, nil, "revoked access token")
}

// @Summary     Authenticate a user to retrieve an authorization token
// @Router      /api/users/adm/authenticate [post]
// @Tags        Auth
// @Accept      json
// @Param       body body models.UserAuthParams true "Authentication parameters"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func authenticateUser(response http.ResponseWriter, request *http.Request) {

	// Auth request consists of Mac Address and Password (from node that is authorizing
	// in case of Master, auth is ignored and mac is set to "mastermac"
	var authRequest models.UserAuthParams
	var errorResponse = models.ErrorResponse{
		Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
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
	user, err := logic.GetUser(authRequest.UserName)
	if err != nil {
		logger.Log(0, authRequest.UserName, "user validation failed: ",
			err.Error())
		logic.ReturnErrorResponse(response, request, logic.FormatError(err, "unauthorized"))
		return
	}
	if logic.IsOauthUser(user) == nil {
		logic.ReturnErrorResponse(response, request, logic.FormatError(errors.New("user is registered via SSO"), "badrequest"))
		return
	}
	if !user.IsSuperAdmin && !logic.IsBasicAuthEnabled() {
		logic.ReturnErrorResponse(
			response,
			request,
			logic.FormatError(fmt.Errorf("basic auth is disabled"), "badrequest"),
		)
		return
	}

	if val := request.Header.Get("From-Ui"); val == "true" {
		// request came from UI, if normal user block Login

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
		logic.ReturnErrorResponse(
			response,
			request,
			logic.FormatError(errors.New("no token returned"), "internal"),
		)
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
		if servercfg.IsPro && logic.GetRacAutoDisable() {
			// enable all associeated clients for the user
			clients, err := logic.GetAllExtClients()
			if err != nil {
				slog.Error("error getting clients: ", "error", err)
				return
			}
			for _, client := range clients {
				if client.OwnerID == username && !client.Enabled {
					slog.Info(
						fmt.Sprintf(
							"enabling ext client %s for user %s due to RAC autodisabling feature",
							client.ClientID,
							client.OwnerID,
						),
					)
					if newClient, err := logic.ToggleExtClientConnectivity(&client, true); err != nil {
						slog.Error(
							"error enabling ext client in RAC autodisable hook",
							"error",
							err,
						)
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

// @Summary     Check if the server has a super admin
// @Router      /api/users/adm/hassuperadmin [get]
// @Tags        Users
// @Success     200 {object} bool
// @Failure     500 {object} models.ErrorResponse
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

// @Summary     Get an individual user
// @Router      /api/users/{username} [get]
// @Tags        Users
// @Param       username path string true "Username of the user to fetch"
// @Success     200 {object} models.User
// @Failure     500 {object} models.ErrorResponse
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

// swagger:route GET /api/v1/users user getUserV1
//
// Get an individual user with role info.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: ReturnUserWithRolesAndGroups
func getUserV1(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	usernameFetched := r.URL.Query().Get("username")
	if usernameFetched == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("username is required"), "badrequest"))
		return
	}
	user, err := logic.GetReturnUser(usernameFetched)
	if err != nil {
		logger.Log(0, usernameFetched, "failed to fetch user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	userRoleTemplate, err := logic.GetRole(user.PlatformRoleID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	resp := models.ReturnUserWithRolesAndGroups{
		ReturnUser:   user,
		PlatformRole: userRoleTemplate,
		UserGroups:   map[models.UserGroupID]models.UserGroup{},
	}
	for gId := range user.UserGroups {
		grp, err := logic.GetUserGroup(gId)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		resp.UserGroups[gId] = grp
	}
	logger.Log(2, r.Header.Get("user"), "fetched user", usernameFetched)
	logic.ReturnSuccessResponseWithJson(w, r, resp, "fetched user with role info")
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

// @Summary     Create a super admin
// @Router      /api/users/adm/createsuperadmin [post]
// @Tags        Users
// @Param       body body models.User true "User details"
// @Success     200 {object} models.User
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func createSuperAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var u models.User

	err := json.NewDecoder(r.Body).Decode(&u)
	if err != nil {
		slog.Error("error decoding request body", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if !logic.IsBasicAuthEnabled() {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("basic auth is disabled"), "badrequest"),
		)
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

// @Summary     Transfer super admin role to another admin user
// @Router      /api/users/adm/transfersuperadmin/{username} [post]
// @Tags        Users
// @Param       username path string true "Username of the user to transfer super admin role"
// @Success     200 {object} models.User
// @Failure     403 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
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
	if !logic.IsBasicAuthEnabled() {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("basic auth is disabled"), "badrequest"),
		)
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

// @Summary     Create a user
// @Router      /api/users/{username} [post]
// @Tags        Users
// @Param       username path string true "Username of the user to create"
// @Param       body body models.User true "User details"
// @Success     200 {object} models.User
// @Failure     400 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
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
	if !servercfg.IsPro {
		user.PlatformRoleID = models.AdminRole
	}

	if user.PlatformRoleID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("platform role is missing"), "badrequest"))
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
	go mq.PublishPeerUpdate(false)
	slog.Info("user was created", "username", user.UserName)
	json.NewEncoder(w).Encode(logic.ToReturnUser(user))
}

// @Summary     Update a user
// @Router      /api/users/{username} [put]
// @Tags        Users
// @Param       username path string true "Username of the user to update"
// @Param       body body models.User true "User details"
// @Success     200 {object} models.User
// @Failure     400 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
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
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("user in param and request body not matching"),
				"badrequest",
			),
		)
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
			slog.Error(
				"failed to update user",
				"caller",
				caller.UserName,
				"attempted to update user",
				username,
				"error",
				err,
			)
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
		if servercfg.IsPro {
			// user cannot update his own roles and groups
			if len(user.NetworkRoles) != len(userchange.NetworkRoles) || !reflect.DeepEqual(user.NetworkRoles, userchange.NetworkRoles) {
				err = errors.New("user cannot update self update their network roles")
				slog.Error("failed to update user", "caller", caller.UserName, "attempted to update user", username, "error", err)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "forbidden"))
				return
			}
			// user cannot update his own roles and groups
			if len(user.UserGroups) != len(userchange.UserGroups) || !reflect.DeepEqual(user.UserGroups, userchange.UserGroups) {
				err = errors.New("user cannot update self update their groups")
				slog.Error("failed to update user", "caller", caller.UserName, "attempted to update user", username, "error", err)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "forbidden"))
				return
			}
		}

	}
	if ismaster {
		if user.PlatformRoleID != models.SuperAdminRole && userchange.PlatformRoleID == models.SuperAdminRole {
			slog.Error("operation not allowed", "caller", logic.MasterUser, "attempted to update user role to superadmin", username)
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("attempted to update user role to superadmin"), "forbidden"))
			return
		}
	}

	if logic.IsOauthUser(user) == nil && userchange.Password != "" {
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
	go mq.PublishPeerUpdate(false)
	logger.Log(1, username, "was updated")
	json.NewEncoder(w).Encode(logic.ToReturnUser(*user))
}

// @Summary     Delete a user
// @Router      /api/users/{username} [delete]
// @Tags        Users
// @Param       username path string true "Username of the user to delete"
// @Success     200 {string} string
// @Failure     500 {object} models.ErrorResponse
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
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("superadmin cannot be deleted"), "internal"),
		)
		return
	}
	if callerUserRole.ID != models.SuperAdminRole {
		if callerUserRole.ID == models.AdminRole && userRole.ID == models.AdminRole {
			slog.Error(
				"failed to delete user: ",
				"user",
				username,
				"error",
				"admin cannot delete another admin user, including oneself",
			)
			logic.ReturnErrorResponse(
				w,
				r,
				logic.FormatError(
					fmt.Errorf("admin cannot delete another admin user, including oneself"),
					"internal",
				),
			)
			return
		}
	}
	err = logic.DeleteUser(username)
	if err != nil {
		logger.Log(0, username,
			"failed to delete user: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
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
				err = logic.DeleteExtClientAndCleanup(extclient)
				if err != nil {
					slog.Error("failed to delete extclient",
						"id", extclient.ClientID, "owner", username, "error", err)
				} else {
					if err := mq.PublishDeletedClientPeerUpdate(&extclient); err != nil {
						slog.Error("error setting ext peers: " + err.Error())
					}
				}
			}
		}
		mq.PublishPeerUpdate(false)
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

// @Summary     lists all user roles.
// @Router      /api/v1/user/roles [get]
// @Tags        Users
// @Param       role_id query string true "roleid required to get the role details"
// @Success     200 {object}  []models.UserRolePermissionTemplate
// @Failure     500 {object} models.ErrorResponse
func listRoles(w http.ResponseWriter, r *http.Request) {
	var roles []models.UserRolePermissionTemplate
	var err error
	roles, err = logic.ListPlatformRoles()
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, roles, "successfully fetched user roles permission templates")
}
