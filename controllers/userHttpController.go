package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mongoconn"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/go-playground/validator.v9"
)

func userHandlers(r *mux.Router) {

	r.HandleFunc("/api/users/adm/hasadmin", hasAdmin).Methods("GET")
	r.HandleFunc("/api/users/adm/createadmin", createAdmin).Methods("POST")
	r.HandleFunc("/api/users/adm/authenticate", authenticateUser).Methods("POST")
	r.HandleFunc("/api/users/{username}", authorizeUser(http.HandlerFunc(updateUser))).Methods("PUT")
	r.HandleFunc("/api/users/{username}", authorizeUser(http.HandlerFunc(deleteUser))).Methods("DELETE")
	r.HandleFunc("/api/users/{username}", authorizeUser(http.HandlerFunc(getUser))).Methods("GET")
}

//Node authenticates using its password and retrieves a JWT for authorization.
func authenticateUser(response http.ResponseWriter, request *http.Request) {

	//Auth request consists of Mac Address and Password (from node that is authorizing
	//in case of Master, auth is ignored and mac is set to "mastermac"
	var authRequest models.UserAuthParams
	var result models.User
	var errorResponse = models.ErrorResponse{
		Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
	}

	decoder := json.NewDecoder(request.Body)
	decoderErr := decoder.Decode(&authRequest)
	defer request.Body.Close()

	if decoderErr != nil {
		returnErrorResponse(response, request, errorResponse)
		return
	} else {
		errorResponse.Code = http.StatusBadRequest
		if authRequest.UserName == "" {
			errorResponse.Message = "W1R3: Username can't be empty"
			returnErrorResponse(response, request, errorResponse)
			return
		} else if authRequest.Password == "" {
			errorResponse.Message = "W1R3: Password can't be empty"
			returnErrorResponse(response, request, errorResponse)
			return
		} else {

			//Search DB for node with Mac Address. Ignore pending nodes (they should not be able to authenticate with API untill approved).
			collection := mongoconn.Client.Database("netmaker").Collection("users")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			var err = collection.FindOne(ctx, bson.M{"username": authRequest.UserName}).Decode(&result)

			defer cancel()

			if err != nil {
				errorResponse.Message = "W1R3: User " + authRequest.UserName + " not found."
				returnErrorResponse(response, request, errorResponse)
				return
			}

			//compare password from request to stored password in database
			//might be able to have a common hash (certificates?) and compare those so that a password isn't passed in in plain text...
			//TODO: Consider a way of hashing the password client side before sending, or using certificates
			err = bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(authRequest.Password))
			if err != nil {
				errorResponse = models.ErrorResponse{
					Code: http.StatusUnauthorized, Message: "W1R3: Wrong Password.",
				}
				returnErrorResponse(response, request, errorResponse)
				return
			} else {
				//Create a new JWT for the node
				tokenString, _ := functions.CreateUserJWT(authRequest.UserName, result.IsAdmin)

				if tokenString == "" {
					returnErrorResponse(response, request, errorResponse)
					return
				}

				var successResponse = models.SuccessResponse{
					Code:    http.StatusOK,
					Message: "W1R3: Device " + authRequest.UserName + " Authorized",
					Response: models.SuccessfulUserLoginResponse{
						AuthToken: tokenString,
						UserName:  authRequest.UserName,
					},
				}
				//Send back the JWT
				successJSONResponse, jsonError := json.Marshal(successResponse)

				if jsonError != nil {
					returnErrorResponse(response, request, errorResponse)
					return
				}
				response.Header().Set("Content-Type", "application/json")
				response.Write(successJSONResponse)
			}
		}
	}
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

		var errorResponse = models.ErrorResponse{
			Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
		}

		w.Header().Set("Content-Type", "application/json")

		//get the auth token
		bearerToken := r.Header.Get("Authorization")

		var tokenSplit = strings.Split(bearerToken, " ")

		//I put this in in case the user doesn't put in a token at all (in which case it's empty)
		//There's probably a smarter way of handling this.
		var authToken = "928rt238tghgwe@TY@$Y@#WQAEGB2FC#@HG#@$Hddd"

		if len(tokenSplit) > 1 {
			authToken = tokenSplit[1]
		} else {
			errorResponse = models.ErrorResponse{
				Code: http.StatusUnauthorized, Message: "W1R3: Missing Auth Token.",
			}
			returnErrorResponse(w, r, errorResponse)
			return
		}

		//This checks if
		//A: the token is the master password
		//B: the token corresponds to a mac address, and if so, which one
		//TODO: There's probably a better way of dealing with the "master token"/master password. Plz Halp.
		username, _, err := functions.VerifyUserToken(authToken)

		if err != nil {
			errorResponse = models.ErrorResponse{
				Code: http.StatusUnauthorized, Message: "W1R3: Error Verifying Auth Token.",
			}
			returnErrorResponse(w, r, errorResponse)
			return
		}

		isAuthorized := username != ""

		if !isAuthorized {
			errorResponse = models.ErrorResponse{
				Code: http.StatusUnauthorized, Message: "W1R3: You are unauthorized to access this endpoint.",
			}
			returnErrorResponse(w, r, errorResponse)
			return
		} else {
			//If authorized, this function passes along it's request and output to the appropriate route function.
			next.ServeHTTP(w, r)
		}
	}
}

func HasAdmin() (bool, error) {

	collection := mongoconn.Client.Database("netmaker").Collection("users")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	filter := bson.M{"isadmin": true}

	//Filtering out the ID field cuz Dillon doesn't like it. May want to filter out other fields in the future
	var result bson.M

	err := collection.FindOne(ctx, filter).Decode(&result)

	defer cancel()

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, err
		fmt.Println(err)
	}
	return true, err
}

func hasAdmin(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	hasadmin, _ := HasAdmin()

	//Returns all the nodes in JSON format

	json.NewEncoder(w).Encode(hasadmin)

}

func GetUser(username string) (models.User, error) {

	var user models.User

	collection := mongoconn.Client.Database("netmaker").Collection("users")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	filter := bson.M{"username": username}
	err := collection.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&user)

	defer cancel()

	return user, err
}

//Get an individual node. Nothin fancy here folks.
func getUser(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	user, err := GetUser(params["username"])

	if err != nil {
		mongoconn.GetError(err, w)
		return
	}

	json.NewEncoder(w).Encode(user)
}

func CreateUser(user models.User) (models.User, error) {

	//encrypt that password so we never see it again
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), 5)

	if err != nil {
		return user, err
	}
	//set password to encrypted password
	user.Password = string(hash)

	tokenString, _ := functions.CreateUserJWT(user.UserName, user.IsAdmin)

	if tokenString == "" {
		//returnErrorResponse(w, r, errorResponse)
		return user, err
	}

	// connect db
	collection := mongoconn.Client.Database("netmaker").Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	// insert our node to the node db.
	result, err := collection.InsertOne(ctx, user)
	_ = result

	defer cancel()

	return user, err
}

func createAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var errorResponse = models.ErrorResponse{
		Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
	}

	hasadmin, err := HasAdmin()

	if hasadmin {
		errorResponse = models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: "W1R3: Admin already exists! ",
		}
		returnErrorResponse(w, r, errorResponse)
		return
	}

	var admin models.User

	//get node from body of request
	_ = json.NewDecoder(r.Body).Decode(&admin)

	admin.IsAdmin = true

	err = ValidateUser("create", admin)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	admin, err = CreateUser(admin)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	json.NewEncoder(w).Encode(admin)
}

func UpdateUser(userchange models.User, user models.User) (models.User, error) {

	queryUser := user.UserName

	if userchange.UserName != "" {
		user.UserName = userchange.UserName
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
	//collection := mongoconn.ConnectDB()
	collection := mongoconn.Client.Database("netmaker").Collection("users")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	// Create filter
	filter := bson.M{"username": queryUser}

	fmt.Println("Updating User " + user.UserName)

	// prepare update model.
	update := bson.D{
		{"$set", bson.D{
			{"username", user.UserName},
			{"password", user.Password},
			{"isadmin", user.IsAdmin},
		}},
	}
	var userupdate models.User

	errN := collection.FindOneAndUpdate(ctx, filter, update).Decode(&userupdate)
	if errN != nil {
		fmt.Println("Could not update: ")
		fmt.Println(errN)
	} else {
		fmt.Println("User updated  successfully.")
	}

	defer cancel()

	return userupdate, errN
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	var user models.User

	//start here
	user, err := GetUser(params["username"])
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	var userchange models.User

	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&userchange)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	userchange.IsAdmin = true

	err = ValidateUser("update", userchange)

	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	user, err = UpdateUser(userchange, user)

	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	json.NewEncoder(w).Encode(user)
}

func DeleteUser(user string) (bool, error) {

	deleted := false

	collection := mongoconn.Client.Database("netmaker").Collection("users")

	filter := bson.M{"username": user}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	result, err := collection.DeleteOne(ctx, filter)

	deletecount := result.DeletedCount

	if deletecount > 0 {
		deleted = true
	}

	defer cancel()

	return deleted, err
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)

	success, err := DeleteUser(params["username"])

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	} else if !success {
		returnErrorResponse(w, r, formatError(errors.New("Delete unsuccessful."), "badrequest"))
		return
	}

	json.NewEncoder(w).Encode(params["username"] + " deleted.")
}

func ValidateUser(operation string, user models.User) error {

	v := validator.New()

	_ = v.RegisterValidation("username_unique", func(fl validator.FieldLevel) bool {
		_, err := GetUser(user.UserName)
		return err == nil || operation == "update"
	})

	_ = v.RegisterValidation("username_valid", func(fl validator.FieldLevel) bool {
		isvalid := functions.NameInNodeCharSet(user.UserName)
		return isvalid
	})

	_ = v.RegisterValidation("password_check", func(fl validator.FieldLevel) bool {
		notEmptyCheck := user.Password != ""
		goodLength := len(user.Password) > 5
		return (notEmptyCheck && goodLength) || operation == "update"
	})

	err := v.Struct(user)

	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			fmt.Println(e)
		}
	}
	return err
}
