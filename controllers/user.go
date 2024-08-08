package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/auth"
	"github.com/gravitl/netmaker/database"
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
	r.HandleFunc("/api/users/adm/transfersuperadmin/{username}", logic.SecurityCheck(true, http.HandlerFunc(transferSuperAdmin))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/users/adm/authenticate", authenticateUser).Methods(http.MethodPost)
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(true, http.HandlerFunc(updateUser))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(true, checkFreeTierLimits(limitChoiceUsers, http.HandlerFunc(createUser)))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(true, http.HandlerFunc(deleteUser))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(false, logic.ContinueIfUserMatch(http.HandlerFunc(getUser)))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/users", logic.SecurityCheck(true, http.HandlerFunc(getUsers))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/users_pending", logic.SecurityCheck(true, http.HandlerFunc(getPendingUsers))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/users_pending", logic.SecurityCheck(true, http.HandlerFunc(deleteAllPendingUsers))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/users_pending/user/{username}", logic.SecurityCheck(true, http.HandlerFunc(deletePendingUser))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/users_pending/user/{username}", logic.SecurityCheck(true, http.HandlerFunc(approvePendingUser))).
		Methods(http.MethodPost)

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

	if !servercfg.IsBasicAuthEnabled() {
		logic.ReturnErrorResponse(
			response,
			request,
			logic.FormatError(fmt.Errorf("basic auth is disabled"), "badrequest"),
		)
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
		if !(user.IsAdmin || user.IsSuperAdmin) {
			logic.ReturnErrorResponse(
				response,
				request,
				logic.FormatError(errors.New("only admins can access dashboard"), "unauthorized"),
			)
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
		if servercfg.IsPro && servercfg.GetRacAutoDisable() {
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

// @Summary     Get all users
// @Router      /api/users [get]
// @Tags        Users
// @Success     200 {array} models.User
// @Failure     500 {object} models.ErrorResponse
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

	if !servercfg.IsBasicAuthEnabled() {
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
	if !caller.IsSuperAdmin {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("only superadmin can assign the superadmin role to another user"),
				"forbidden",
			),
		)
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
	if !u.IsAdmin {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("only admins can be promoted to superadmin role"),
				"forbidden",
			),
		)
		return
	}
	if !servercfg.IsBasicAuthEnabled() {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("basic auth is disabled"), "badrequest"),
		)
		return
	}

	u.IsSuperAdmin = true
	u.IsAdmin = false
	err = logic.UpsertUser(*u)
	if err != nil {
		slog.Error("error updating user to superadmin: ", "user", u.UserName, "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	caller.IsSuperAdmin = false
	caller.IsAdmin = true
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
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
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
	if !caller.IsSuperAdmin && user.IsAdmin {
		err = errors.New("only superadmin can create admin users")
		slog.Error("error creating new user: ", "user", user.UserName, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "forbidden"))
		return
	}
	if user.IsSuperAdmin {
		err = errors.New("additional superadmins cannot be created")
		slog.Error("error creating new user: ", "user", user.UserName, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "forbidden"))
		return
	}
	if !servercfg.IsPro && !user.IsAdmin {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("non-admins users can only be created on Pro version"),
				"forbidden",
			),
		)
		return
	}

	err = logic.CreateUser(&user)
	if err != nil {
		slog.Error("error creating new user: ", "user", user.UserName, "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
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
		if caller.IsAdmin && user.IsSuperAdmin {
			slog.Error(
				"non-superadmin user",
				"caller",
				caller.UserName,
				"attempted to update superadmin user",
				username,
			)
			logic.ReturnErrorResponse(
				w,
				r,
				logic.FormatError(errors.New("cannot update superadmin user"), "forbidden"),
			)
			return
		}
		if !caller.IsAdmin && !caller.IsSuperAdmin {
			slog.Error(
				"operation not allowed",
				"caller",
				caller.UserName,
				"attempted to update user",
				username,
			)
			logic.ReturnErrorResponse(
				w,
				r,
				logic.FormatError(errors.New("cannot update superadmin user"), "forbidden"),
			)
			return
		}
		if caller.IsAdmin && user.IsAdmin {
			slog.Error(
				"admin user cannot update another admin",
				"caller",
				caller.UserName,
				"attempted to update admin user",
				username,
			)
			logic.ReturnErrorResponse(
				w,
				r,
				logic.FormatError(
					errors.New("admin user cannot update another admin"),
					"forbidden",
				),
			)
			return
		}
		if caller.IsAdmin && userchange.IsAdmin {
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
		if user.IsAdmin != userchange.IsAdmin || user.IsSuperAdmin != userchange.IsSuperAdmin {
			slog.Error(
				"user cannot change his own role",
				"caller",
				caller.UserName,
				"attempted to update user role",
				username,
			)
			logic.ReturnErrorResponse(
				w,
				r,
				logic.FormatError(errors.New("user not allowed to self assign role"), "forbidden"),
			)
			return

		}
	}
	if ismaster {
		if !user.IsSuperAdmin && userchange.IsSuperAdmin {
			slog.Error(
				"operation not allowed",
				"caller",
				logic.MasterUser,
				"attempted to update user role to superadmin",
				username,
			)
			logic.ReturnErrorResponse(
				w,
				r,
				logic.FormatError(
					errors.New("attempted to update user role to superadmin"),
					"forbidden",
				),
			)
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
	username := params["username"]
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username,
			"failed to update user info: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if user.IsSuperAdmin {
		slog.Error(
			"failed to delete user: ", "user", username, "error", "superadmin cannot be deleted")
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("superadmin cannot be deleted"), "internal"),
		)
		return
	}
	if !caller.IsSuperAdmin {
		if caller.IsAdmin && user.IsAdmin {
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
			var newPass, fetchErr = auth.FetchPassValue("")
			if fetchErr != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(fetchErr, "internal"))
				return
			}
			if err = logic.CreateUser(&models.User{
				UserName: user.UserName,
				Password: newPass,
			}); err != nil {
				logic.ReturnErrorResponse(
					w,
					r,
					logic.FormatError(fmt.Errorf("failed to create user: %s", err), "internal"),
				)
				return
			}
			err = logic.DeletePendingUser(username)
			if err != nil {
				logic.ReturnErrorResponse(
					w,
					r,
					logic.FormatError(
						fmt.Errorf("failed to delete pending user: %s", err),
						"internal",
					),
				)
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
				logic.ReturnErrorResponse(
					w,
					r,
					logic.FormatError(
						fmt.Errorf("failed to delete pending user: %s", err),
						"internal",
					),
				)
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
	w.Header().Set("Content-Type", "application/json")
	err := database.DeleteAllRecords(database.PENDING_USERS_TABLE_NAME)
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("failed to delete all pending users "+err.Error()),
				"internal",
			),
		)
		return
	}
	logic.ReturnSuccessResponse(w, r, "cleared all pending users")
}
