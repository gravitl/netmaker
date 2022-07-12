package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/auth"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func userHandlers(r *mux.Router) {

	r.HandleFunc("/api/users/adm/hasadmin", hasAdmin).Methods("GET")
	r.HandleFunc("/api/users/adm/createadmin", createAdmin).Methods("POST")
	r.HandleFunc("/api/users/adm/authenticate", authenticateUser).Methods("POST")
	r.HandleFunc("/api/users/{username}", securityCheck(false, continueIfUserMatch(http.HandlerFunc(updateUser)))).Methods("PUT")
	r.HandleFunc("/api/users/networks/{username}", securityCheck(true, http.HandlerFunc(updateUserNetworks))).Methods("PUT")
	r.HandleFunc("/api/users/{username}/adm", securityCheck(true, http.HandlerFunc(updateUserAdm))).Methods("PUT")
	r.HandleFunc("/api/users/{username}", securityCheck(true, http.HandlerFunc(createUser))).Methods("POST")
	r.HandleFunc("/api/users/{username}", securityCheck(true, http.HandlerFunc(deleteUser))).Methods("DELETE")
	r.HandleFunc("/api/users/{username}", securityCheck(false, continueIfUserMatch(http.HandlerFunc(getUser)))).Methods("GET")
	r.HandleFunc("/api/users", securityCheck(true, http.HandlerFunc(getUsers))).Methods("GET")
	r.HandleFunc("/api/oauth/login", auth.HandleAuthLogin).Methods("GET")
	r.HandleFunc("/api/oauth/callback", auth.HandleAuthCallback).Methods("GET")
}

// Node authenticates using its password and retrieves a JWT for authorization.
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
		returnErrorResponse(response, request, errorResponse)
		return
	}
	username := authRequest.UserName
	jwt, err := logic.VerifyAuthRequest(authRequest)
	if err != nil {
		logger.Log(0, username, "user validation failed: ",
			err.Error())
		returnErrorResponse(response, request, formatError(err, "badrequest"))
		return
	}

	if jwt == "" {
		// very unlikely that err is !nil and no jwt returned, but handle it anyways.
		logger.Log(0, username, "jwt token is empty")
		returnErrorResponse(response, request, formatError(errors.New("no token returned"), "internal"))
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
		returnErrorResponse(response, request, errorResponse)
		return
	}
	logger.Log(2, username, "was authenticated")
	response.Header().Set("Content-Type", "application/json")
	response.Write(successJSONResponse)
}

func hasAdmin(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	hasadmin, err := logic.HasAdmin()
	if err != nil {
		logger.Log(0, "failed to check for admin: ", err.Error())
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	json.NewEncoder(w).Encode(hasadmin)

}

// GetUserInternal - gets an internal user
func GetUserInternal(username string) (models.User, error) {

	var user models.User
	record, err := database.FetchRecord(database.USERS_TABLE_NAME, username)
	if err != nil {
		return user, err
	}
	if err = json.Unmarshal([]byte(record), &user); err != nil {
		return models.User{}, err
	}
	return user, err
}

// Get an individual user. Nothin fancy here folks.
func getUser(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	usernameFetched := params["username"]
	user, err := logic.GetUser(usernameFetched)

	if err != nil {
		logger.Log(0, usernameFetched, "failed to fetch user: ", err.Error())
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "fetched user", usernameFetched)
	json.NewEncoder(w).Encode(user)
}

// Get all users. Nothin fancy here folks.
func getUsers(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	users, err := logic.GetUsers()

	if err != nil {
		logger.Log(0, "failed to fetch users: ", err.Error())
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	logger.Log(2, r.Header.Get("user"), "fetched users")
	json.NewEncoder(w).Encode(users)
}

func createAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var admin models.User

	err := json.NewDecoder(r.Body).Decode(&admin)
	if err != nil {

		logger.Log(0, admin.UserName, "error decoding request body: ",
			err.Error())
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	admin, err = logic.CreateAdmin(admin)

	if err != nil {
		logger.Log(0, admin.UserName, "failed to create admin: ",
			err.Error())
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	logger.Log(1, admin.UserName, "was made a new admin")
	json.NewEncoder(w).Encode(admin)
}

func createUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var user models.User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	user, err = logic.CreateUser(user)
	if err != nil {
		logger.Log(0, user.UserName, "error creating new user: ",
			err.Error())
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	logger.Log(1, user.UserName, "was created")
	json.NewEncoder(w).Encode(user)
}

func updateUserNetworks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	var user models.User
	// start here
	username := params["username"]
	user, err := GetUserInternal(username)
	if err != nil {
		logger.Log(0, username,
			"failed to update user networks: ", err.Error())
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	var userchange models.User
	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&userchange)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	err = logic.UpdateUserNetworks(userchange.Networks, userchange.IsAdmin, &user)
	if err != nil {
		logger.Log(0, username,
			"failed to update user networks: ", err.Error())
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	logger.Log(1, username, "status was updated")
	json.NewEncoder(w).Encode(user)
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	var user models.User
	// start here
	username := params["username"]
	user, err := GetUserInternal(username)
	if err != nil {
		logger.Log(0, username,
			"failed to update user info: ", err.Error())
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	if auth.IsOauthUser(&user) == nil {
		err := fmt.Errorf("cannot update user info for oauth user %s", username)
		logger.Log(0, err.Error())
		returnErrorResponse(w, r, formatError(err, "forbidden"))
		return
	}
	var userchange models.User
	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&userchange)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	userchange.Networks = nil
	user, err = logic.UpdateUser(userchange, user)
	if err != nil {
		logger.Log(0, username,
			"failed to update user info: ", err.Error())
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	logger.Log(1, username, "was updated")
	json.NewEncoder(w).Encode(user)
}

func updateUserAdm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	var user models.User
	// start here
	username := params["username"]
	user, err := GetUserInternal(username)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	if auth.IsOauthUser(&user) != nil {
		err := fmt.Errorf("cannot update user info for oauth user %s", username)
		logger.Log(0, err.Error())
		returnErrorResponse(w, r, formatError(err, "forbidden"))
		return
	}
	var userchange models.User
	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&userchange)
	if err != nil {
		logger.Log(0, "error decoding request body: ",
			err.Error())
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	if !user.IsAdmin {
		logger.Log(0, username, "not a admin user")
		returnErrorResponse(w, r, formatError(errors.New("not a admin user"), "badrequest"))
	}
	user, err = logic.UpdateUser(userchange, user)
	if err != nil {
		logger.Log(0, username,
			"failed to update user (admin) info: ", err.Error())
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	logger.Log(1, username, "was updated (admin)")
	json.NewEncoder(w).Encode(user)
}

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
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	} else if !success {
		err := errors.New("delete unsuccessful")
		logger.Log(0, username, err.Error())
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	logger.Log(1, username, "was deleted")
	json.NewEncoder(w).Encode(params["username"] + " deleted.")
}
