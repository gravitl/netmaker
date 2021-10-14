package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/crypto/bcrypt"
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
}

//Node authenticates using its password and retrieves a JWT for authorization.
func authenticateUser(response http.ResponseWriter, request *http.Request) {

	//Auth request consists of Mac Address and Password (from node that is authorizing
	//in case of Master, auth is ignored and mac is set to "mastermac"
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

	jwt, err := VerifyAuthRequest(authRequest)
	if err != nil {
		returnErrorResponse(response, request, formatError(err, "badrequest"))
		return
	}

	if jwt == "" {
		//very unlikely that err is !nil and no jwt returned, but handle it anyways.
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
	//Send back the JWT
	successJSONResponse, jsonError := json.Marshal(successResponse)

	if jsonError != nil {
		returnErrorResponse(response, request, errorResponse)
		return
	}
	functions.PrintUserLog(username, "was authenticated", 2)
	response.Header().Set("Content-Type", "application/json")
	response.Write(successJSONResponse)
}

func VerifyAuthRequest(authRequest models.UserAuthParams) (string, error) {
	var result models.User
	if authRequest.UserName == "" {
		return "", errors.New("username can't be empty")
	} else if authRequest.Password == "" {
		return "", errors.New("password can't be empty")
	}
	//Search DB for node with Mac Address. Ignore pending nodes (they should not be able to authenticate with API untill approved).
	record, err := database.FetchRecord(database.USERS_TABLE_NAME, authRequest.UserName)
	if err != nil {
		return "", errors.New("incorrect credentials")
	}
	if err = json.Unmarshal([]byte(record), &result); err != nil {
		return "", errors.New("incorrect credentials")
	}

	//compare password from request to stored password in database
	//might be able to have a common hash (certificates?) and compare those so that a password isn't passed in in plain text...
	//TODO: Consider a way of hashing the password client side before sending, or using certificates
	if err = bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(authRequest.Password)); err != nil {
		return "", errors.New("incorrect credentials")
	}

	//Create a new JWT for the node
	tokenString, _ := functions.CreateUserJWT(authRequest.UserName, result.Networks, result.IsAdmin)
	return tokenString, nil
}

//The middleware for most requests to the API
//They all pass  through here first
//This will validate the JWT (or check for master token)
//This will also check against the authNetwork and make sure the node should be accessing that endpoint,
//even if it's technically ok
//This is kind of a poor man's RBAC. There's probably a better/smarter way.
//TODO: Consider better RBAC implementations
func authorizeUser(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var params = mux.Vars(r)

		//get the auth token
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

func HasAdmin() (bool, error) {

	collection, err := database.FetchRecords(database.USERS_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return false, nil
		} else {
			return true, err

		}
	}
	for _, value := range collection { // filter for isadmin true
		var user models.User
		err = json.Unmarshal([]byte(value), &user)
		if err != nil {
			continue
		}
		if user.IsAdmin {
			return true, nil
		}
	}

	return false, err
}

func hasAdmin(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	hasadmin, err := HasAdmin()
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	json.NewEncoder(w).Encode(hasadmin)

}

func GetUser(username string) (models.ReturnUser, error) {

	var user models.ReturnUser
	record, err := database.FetchRecord(database.USERS_TABLE_NAME, username)
	if err != nil {
		return user, err
	}
	if err = json.Unmarshal([]byte(record), &user); err != nil {
		return models.ReturnUser{}, err
	}
	return user, err
}

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

func GetUsers() ([]models.ReturnUser, error) {

	var users []models.ReturnUser

	collection, err := database.FetchRecords(database.USERS_TABLE_NAME)

	if err != nil {
		return users, err
	}

	for _, value := range collection {

		var user models.ReturnUser
		err = json.Unmarshal([]byte(value), &user)
		if err != nil {
			continue // get users
		}
		users = append(users, user)
	}

	return users, err
}

//Get an individual node. Nothin fancy here folks.
func getUser(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	usernameFetched := params["username"]
	user, err := GetUser(usernameFetched)

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "fetched user "+usernameFetched, 2)
	json.NewEncoder(w).Encode(user)
}

//Get an individual node. Nothin fancy here folks.
func getUsers(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	users, err := GetUsers()

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	functions.PrintUserLog(r.Header.Get("user"), "fetched users", 2)
	json.NewEncoder(w).Encode(users)
}

func CreateUser(user models.User) (models.User, error) {
	//check if user exists
	if _, err := GetUser(user.UserName); err == nil {
		return models.User{}, errors.New("user exists")
	}
	err := ValidateUser("create", user)
	if err != nil {
		return models.User{}, err
	}

	//encrypt that password so we never see it again
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), 5)
	if err != nil {
		return user, err
	}
	//set password to encrypted password
	user.Password = string(hash)

	tokenString, _ := functions.CreateUserJWT(user.UserName, user.Networks, user.IsAdmin)

	if tokenString == "" {
		//returnErrorResponse(w, r, errorResponse)
		return user, err
	}

	// connect db
	data, err := json.Marshal(&user)
	if err != nil {
		return user, err
	}
	err = database.Insert(user.UserName, string(data), database.USERS_TABLE_NAME)

	return user, err
}

func createAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var admin models.User
	//get node from body of request
	_ = json.NewDecoder(r.Body).Decode(&admin)

	admin, err := CreateAdmin(admin)

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	functions.PrintUserLog(admin.UserName, "was made a new admin", 1)
	json.NewEncoder(w).Encode(admin)
}

func CreateAdmin(admin models.User) (models.User, error) {
	hasadmin, err := HasAdmin()
	if err != nil {
		return models.User{}, err
	}
	if hasadmin {
		return models.User{}, errors.New("admin user already exists")
	}
	admin.IsAdmin = true
	return CreateUser(admin)
}

func createUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var user models.User
	//get node from body of request
	_ = json.NewDecoder(r.Body).Decode(&user)

	user, err := CreateUser(user)

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	functions.PrintUserLog(user.UserName, "was created", 1)
	json.NewEncoder(w).Encode(user)
}

func UpdateUser(userchange models.User, user models.User) (models.User, error) {
	//check if user exists
	if _, err := GetUser(user.UserName); err != nil {
		return models.User{}, err
	}

	err := ValidateUser("update", userchange)
	if err != nil {
		return models.User{}, err
	}

	queryUser := user.UserName

	if userchange.UserName != "" {
		user.UserName = userchange.UserName
	}
	if len(userchange.Networks) > 0 {
		user.Networks = userchange.Networks
	}
	if userchange.Password != "" {
		//encrypt that password so we never see it again
		hash, err := bcrypt.GenerateFromPassword([]byte(userchange.Password), 5)

		if err != nil {
			return userchange, err
		}
		//set password to encrypted password
		userchange.Password = string(hash)

		user.Password = userchange.Password
	}
	if err = database.DeleteRecord(database.USERS_TABLE_NAME, queryUser); err != nil {
		return models.User{}, err
	}
	data, err := json.Marshal(&user)
	if err != nil {
		return models.User{}, err
	}
	if err = database.Insert(user.UserName, string(data), database.USERS_TABLE_NAME); err != nil {
		return models.User{}, err
	}
	functions.PrintUserLog(models.NODE_SERVER_NAME, "updated user "+queryUser, 1)
	return user, nil
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	var user models.User
	//start here
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
	user, err = UpdateUser(userchange, user)
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
	//start here
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
	user, err = UpdateUser(userchange, user)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	functions.PrintUserLog(username, "was updated (admin)", 1)
	json.NewEncoder(w).Encode(user)
}

func DeleteUser(user string) (bool, error) {

	if userRecord, err := database.FetchRecord(database.USERS_TABLE_NAME, user); err != nil || len(userRecord) == 0 {
		return false, errors.New("user does not exist")
	}

	err := database.DeleteRecord(database.USERS_TABLE_NAME, user)
	if err != nil {
		return false, err
	}
	return true, nil
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)

	username := params["username"]
	success, err := DeleteUser(username)

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

func ValidateUser(operation string, user models.User) error {

	v := validator.New()
	err := v.Struct(user)

	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			fmt.Println(e)
		}
	}
	return err
}
