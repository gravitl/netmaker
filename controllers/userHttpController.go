package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/auth"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func userHandlers(r *mux.Router) {

	r.HandleFunc("/api/users/adm/hasadmin", hasAdmin).Methods("GET")
	r.HandleFunc("/api/users/adm/createadmin", createAdmin).Methods("POST")
	r.HandleFunc("/api/users/adm/authenticate", authenticateUser).Methods("POST")
	r.HandleFunc("/api/users/{username}", authorizeUser(http.HandlerFunc(updateUser))).Methods("PUT")
	r.HandleFunc("/api/users/{username}/adm", authorizeUserAdm(http.HandlerFunc(updateUserAdm))).Methods("PUT")
	r.HandleFunc("/api/users/{username}", authorizeUserAdm(http.HandlerFunc(createUser))).Methods("POST")
	r.HandleFunc("/api/users/{username}", authorizeUser(http.HandlerFunc(deleteUser))).Methods("DELETE")
	r.HandleFunc("/api/users/{username}", authorizeUser(http.HandlerFunc(getUser))).Methods("GET")
	r.HandleFunc("/api/users", authorizeUserAdm(http.HandlerFunc(getUsers))).Methods("GET")
	r.HandleFunc("/api/oauth/login", auth.HandleAuthLogin).Methods("GET")
	r.HandleFunc("/api/oauth/callback", auth.HandleAuthCallback).Methods("GET")
	r.HandleFunc("/api/oauth/error", throwOauthError).Methods("GET")
}

func throwOauthError(response http.ResponseWriter, request *http.Request) {
	returnErrorResponse(response, request, formatError(errors.New("No token returned"), "unauthorized"))
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
		returnErrorResponse(response, request, formatError(errors.New("No token returned"), "internal"))
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
	functions.PrintUserLog(username, "was authenticated", 2)
	response.Header().Set("Content-Type", "application/json")
	response.Write(successJSONResponse)
}

// The middleware for most requests to the API
// They all pass  through here first
// This will validate the JWT (or check for master token)
// This will also check against the authNetwork and make sure the node should be accessing that endpoint,
// even if it's technically ok
// This is kind of a poor man's RBAC. There's probably a better/smarter way.
// TODO: Consider better RBAC implementations
func authorizeUser(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var params = mux.Vars(r)

		// get the auth token
		bearerToken := r.Header.Get("Authorization")
		username := params["username"]
		err := ValidateUserToken(bearerToken, username, false)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "unauthorized"))
			return
		}
		r.Header.Set("user", username)
		next.ServeHTTP(w, r)
	}
}

func authorizeUserAdm(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var params = mux.Vars(r)

		//get the auth token
		bearerToken := r.Header.Get("Authorization")
		username := params["username"]
		err := ValidateUserToken(bearerToken, username, true)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "unauthorized"))
			return
		}
		r.Header.Set("user", username)
		next.ServeHTTP(w, r)
	}
}

// ValidateUserToken - self explained
func ValidateUserToken(token string, user string, adminonly bool) error {
	var tokenSplit = strings.Split(token, " ")
	//I put this in in case the user doesn't put in a token at all (in which case it's empty)
	//There's probably a smarter way of handling this.
	var authToken = "928rt238tghgwe@TY@$Y@#WQAEGB2FC#@HG#@$Hddd"

	if len(tokenSplit) > 1 {
		authToken = tokenSplit[1]
	} else {
		return errors.New("Missing Auth Token.")
	}

	username, _, isadmin, err := functions.VerifyUserToken(authToken)
	if err != nil {
		return errors.New("Error Verifying Auth Token")
	}
	isAuthorized := false
	if adminonly {
		isAuthorized = isadmin
	} else {
		isAuthorized = username == user || isadmin
	}
	if !isAuthorized {
		return errors.New("You are unauthorized to access this endpoint.")
	}

	return nil
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
	functions.PrintUserLog(r.Header.Get("user"), "fetched user "+usernameFetched, 2)
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

	functions.PrintUserLog(r.Header.Get("user"), "fetched users", 2)
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
	functions.PrintUserLog(admin.UserName, "was made a new admin", 1)
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
	functions.PrintUserLog(user.UserName, "was created", 1)
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
	functions.PrintUserLog(username, "was updated", 1)
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
	functions.PrintUserLog(username, "was updated (admin)", 1)
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
		returnErrorResponse(w, r, formatError(errors.New("delete unsuccessful."), "badrequest"))
		return
	}

	functions.PrintUserLog(username, "was deleted", 1)
	json.NewEncoder(w).Encode(params["username"] + " deleted.")
}
