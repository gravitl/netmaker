package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/auth"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

var (
	upgrader = websocket.Upgrader{}
)

func userHandlers(r *mux.Router) {

	r.HandleFunc("/api/users/adm/hasadmin", hasAdmin).Methods("GET")
	r.HandleFunc("/api/users/adm/createadmin", createAdmin).Methods("POST")
	r.HandleFunc("/api/users/adm/authenticate", authenticateUser).Methods("POST")
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(false, logic.ContinueIfUserMatch(http.HandlerFunc(updateUser)))).Methods("PUT")
	r.HandleFunc("/api/users/networks/{username}", logic.SecurityCheck(true, http.HandlerFunc(updateUserNetworks))).Methods("PUT")
	r.HandleFunc("/api/users/{username}/adm", logic.SecurityCheck(true, http.HandlerFunc(updateUserAdm))).Methods("PUT")
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(true, checkFreeTierLimits(users_l, http.HandlerFunc(createUser)))).Methods("POST")
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(true, http.HandlerFunc(deleteUser))).Methods("DELETE")
	r.HandleFunc("/api/users/{username}", logic.SecurityCheck(false, logic.ContinueIfUserMatch(http.HandlerFunc(getUser)))).Methods("GET")
	r.HandleFunc("/api/users", logic.SecurityCheck(true, http.HandlerFunc(getUsers))).Methods("GET")
	r.HandleFunc("/api/oauth/login", auth.HandleAuthLogin).Methods("GET")
	r.HandleFunc("/api/oauth/callback", auth.HandleAuthCallback).Methods("GET")
	r.HandleFunc("/api/oauth/node-handler", socketHandler)
	r.HandleFunc("/api/oauth/register/{regKey}", auth.RegisterNodeSSO).Methods("GET")
}

// swagger:route POST /api/users/adm/authenticate user authenticateUser
//
// Node authenticates using its password and retrieves a JWT for authorization.
//
//		Schemes: https
//
// 		Security:
//   		oauth
//
//		Responses:
//			200: successResponse
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
			"error marshalling resp: ", err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	logger.Log(2, username, "was authenticated")
	response.Header().Set("Content-Type", "application/json")
	response.Write(successJSONResponse)
}

// swagger:route GET /api/users/adm/hasadmin user hasAdmin
//
// Checks whether the server has an admin.
//
//		Schemes: https
//
// 		Security:
//   		oauth
//
//		Responses:
//			200: successResponse
func hasAdmin(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	hasadmin, err := logic.HasAdmin()
	if err != nil {
		logger.Log(0, "failed to check for admin: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	json.NewEncoder(w).Encode(hasadmin)

}

// swagger:route GET /api/users/{username} user getUser
//
// Get an individual user.
//
//		Schemes: https
//
// 		Security:
//   		oauth
//
//		Responses:
//			200: userBodyResponse
func getUser(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	usernameFetched := params["username"]
	user, err := logic.GetUser(usernameFetched)

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
//		Schemes: https
//
// 		Security:
//   		oauth
//
//		Responses:
//			200: userBodyResponse
func getUsers(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	users, err := logic.GetUsers()

	if err != nil {
		logger.Log(0, "failed to fetch users: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logger.Log(2, r.Header.Get("user"), "fetched users")
	json.NewEncoder(w).Encode(users)
}

// swagger:route POST /api/users/adm/createadmin user createAdmin
//
// Make a user an admin.
//
//		Schemes: https
//
// 		Security:
//   		oauth
//
//		Responses:
//			200: userBodyResponse
func createAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var admin models.User

	err := json.NewDecoder(r.Body).Decode(&admin)
	if err != nil {

		logger.Log(0, admin.UserName, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if !servercfg.IsBasicAuthEnabled() {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("basic auth is disabled"), "badrequest"))
		return
	}

	err = logic.CreateAdmin(&admin)
	if err != nil {
		logger.Log(0, admin.UserName, "failed to create admin: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	logger.Log(1, admin.UserName, "was made a new admin")
	json.NewEncoder(w).Encode(admin)
}

// swagger:route POST /api/users/{username} user createUser
//
// Create a user.
//
//		Schemes: https
//
// 		Security:
//   		oauth
//
//		Responses:
//			200: userBodyResponse
func createUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var user models.User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		logger.Log(0, user.UserName, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	err = logic.CreateUser(&user)
	if err != nil {
		logger.Log(0, user.UserName, "error creating new user: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, user.UserName, "was created")
	json.NewEncoder(w).Encode(user)
}

// swagger:route PUT /api/users/networks/{username} user updateUserNetworks
//
// Updates the networks of the given user.
//
//		Schemes: https
//
// 		Security:
//   		oauth
//
//		Responses:
//			200: userBodyResponse
func updateUserNetworks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	// start here
	username := params["username"]
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username,
			"failed to update user networks: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	var userchange models.User
	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&userchange)
	if err != nil {
		logger.Log(0, username, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = logic.UpdateUserNetworks(userchange.Networks, userchange.Groups, userchange.IsAdmin, &models.ReturnUser{
		Groups:   user.Groups,
		IsAdmin:  user.IsAdmin,
		Networks: user.Networks,
		UserName: user.UserName,
	})

	if err != nil {
		logger.Log(0, username,
			"failed to update user networks: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, username, "status was updated")
	json.NewEncoder(w).Encode(user)
}

// swagger:route PUT /api/users/{username} user updateUser
//
// Update a user.
//
//		Schemes: https
//
// 		Security:
//   		oauth
//
//		Responses:
//			200: userBodyResponse
func updateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	// start here
	username := params["username"]
	user, err := logic.GetUser(username)
	if err != nil {
		logger.Log(0, username,
			"failed to update user info: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if auth.IsOauthUser(user) == nil {
		err := fmt.Errorf("cannot update user info for oauth user %s", username)
		logger.Log(0, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "forbidden"))
		return
	}
	var userchange models.User
	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&userchange)
	if err != nil {
		logger.Log(0, username, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	userchange.Networks = nil
	user, err = logic.UpdateUser(&userchange, user)
	if err != nil {
		logger.Log(0, username,
			"failed to update user info: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, username, "was updated")
	json.NewEncoder(w).Encode(user)
}

// swagger:route PUT /api/users/{username}/adm user updateUserAdm
//
// Updates the given admin user's info (as long as the user is an admin).
//
//		Schemes: https
//
// 		Security:
//   		oauth
//
//		Responses:
//			200: userBodyResponse
func updateUserAdm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	// start here
	username := params["username"]
	user, err := logic.GetUser(username)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if auth.IsOauthUser(user) != nil {
		err := fmt.Errorf("cannot update user info for oauth user %s", username)
		logger.Log(0, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "forbidden"))
		return
	}
	var userchange models.User
	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&userchange)
	if err != nil {
		logger.Log(0, username, "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if !user.IsAdmin {
		logger.Log(0, username, "not an admin user")
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("not a admin user"), "badrequest"))
	}
	user, err = logic.UpdateUser(&userchange, user)
	if err != nil {
		logger.Log(0, username,
			"failed to update user (admin) info: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, username, "was updated (admin)")
	json.NewEncoder(w).Encode(user)
}

// swagger:route DELETE /api/users/{username} user deleteUser
//
// Delete a user.
//
//		Schemes: https
//
// 		Security:
//   		oauth
//
//		Responses:
//			200: userBodyResponse
func deleteUser(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)

	username := params["username"]

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
