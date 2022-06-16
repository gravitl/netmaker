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
		returnErrorResponse(response, request, errorResponse)
		return
	}

	jwt, err := logic.VerifyAuthRequest(authRequest)
	if err != nil {
		returnErrorResponse(response, request, formatError(err, "badrequest"))
		return
	}

	if jwt == "" {
		// very unlikely that err is !nil and no jwt returned, but handle it anyways.
		returnErrorResponse(response, request, formatError(errors.New("no token returned"), "internal"))
		return
	}

	username := authRequest.UserName
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

// Get an individual node. Nothin fancy here folks.
func getUser(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	usernameFetched := params["username"]
	user, err := logic.GetUser(usernameFetched)

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "fetched user", usernameFetched)
	json.NewEncoder(w).Encode(user)
}

// Get an individual node. Nothin fancy here folks.
func getUsers(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	users, err := logic.GetUsers()

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	logger.Log(2, r.Header.Get("user"), "fetched users")
	json.NewEncoder(w).Encode(users)
}

func createAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var admin models.User
	// get node from body of request
	_ = json.NewDecoder(r.Body).Decode(&admin)

	admin, err := logic.CreateAdmin(admin)

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	logger.Log(1, admin.UserName, "was made a new admin")
	json.NewEncoder(w).Encode(admin)
}

func createUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var user models.User
	// get node from body of request
	_ = json.NewDecoder(r.Body).Decode(&user)

	user, err := logic.CreateUser(user)

	if err != nil {
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
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	var userchange models.User
	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&userchange)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	err = logic.UpdateUserNetworks(userchange.Networks, userchange.IsAdmin, &user)
	if err != nil {
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
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	if auth.IsOauthUser(&user) == nil {
		returnErrorResponse(w, r, formatError(fmt.Errorf("can not update user info for oauth user %s", username), "forbidden"))
		return
	}
	var userchange models.User
	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&userchange)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	userchange.Networks = nil
	user, err = logic.UpdateUser(userchange, user)
	if err != nil {
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
		returnErrorResponse(w, r, formatError(fmt.Errorf("can not update user info for oauth user"), "forbidden"))
		return
	}
	var userchange models.User
	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&userchange)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	user, err = logic.UpdateUser(userchange, user)
	if err != nil {
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
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	} else if !success {
		returnErrorResponse(w, r, formatError(errors.New("delete unsuccessful"), "badrequest"))
		return
	}

	logger.Log(1, username, "was deleted")
	json.NewEncoder(w).Encode(params["username"] + " deleted.")
}
