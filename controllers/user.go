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
	r.HandleFunc("/api/v1/users", logic.SecurityCheck(false, logic.ContinueIfUserMatch(http.HandlerFunc(getUserV1)))).Methods(http.MethodGet)
	r.HandleFunc("/api/users", logic.SecurityCheck(true, http.HandlerFunc(getUsers))).Methods(http.MethodGet)

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
	usernameFetched, _ := url.QueryUnescape(r.URL.Query().Get("username"))
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
	if err = logic.IsGroupsValid(user.UserGroups); err != nil {
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
