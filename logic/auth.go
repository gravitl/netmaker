package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/crypto/bcrypt"
)

// HasAdmin - checks if server has an admin
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

// GetReturnUser - gets a user
func GetReturnUser(username string) (models.ReturnUser, error) {

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

// GetUsers - gets users
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

// CreateUser - creates a user
func CreateUser(user models.User) (models.User, error) {
	// check if user exists
	if _, err := GetUser(user.UserName); err == nil {
		return models.User{}, errors.New("user exists")
	}
	var err = ValidateUser(user)
	if err != nil {
		return models.User{}, err
	}

	// encrypt that password so we never see it again
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), 5)
	if err != nil {
		return user, err
	}
	// set password to encrypted password
	user.Password = string(hash)

	tokenString, _ := CreateUserJWT(user.UserName, user.Networks, user.IsAdmin)

	if tokenString == "" {
		// returnErrorResponse(w, r, errorResponse)
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

// CreateAdmin - creates an admin user
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

// VerifyAuthRequest - verifies an auth request
func VerifyAuthRequest(authRequest models.UserAuthParams) (string, error) {
	var result models.User
	if authRequest.UserName == "" {
		return "", errors.New("username can't be empty")
	} else if authRequest.Password == "" {
		return "", errors.New("password can't be empty")
	}
	//Search DB for node with Mac Address. Ignore pending nodes (they should not be able to authenticate with API until approved).
	record, err := database.FetchRecord(database.USERS_TABLE_NAME, authRequest.UserName)
	if err != nil {
		return "", errors.New("incorrect credentials")
	}
	if err = json.Unmarshal([]byte(record), &result); err != nil {
		return "", errors.New("incorrect credentials")
	}

	// compare password from request to stored password in database
	// might be able to have a common hash (certificates?) and compare those so that a password isn't passed in in plain text...
	// TODO: Consider a way of hashing the password client side before sending, or using certificates
	if err = bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(authRequest.Password)); err != nil {
		return "", errors.New("incorrect credentials")
	}

	//Create a new JWT for the node
	tokenString, _ := CreateUserJWT(authRequest.UserName, result.Networks, result.IsAdmin)
	return tokenString, nil
}

// UpdateUserNetworks - updates the networks of a given user
func UpdateUserNetworks(newNetworks []string, isadmin bool, currentUser *models.User) error {
	// check if user exists
	if returnedUser, err := GetUser(currentUser.UserName); err != nil {
		return err
	} else if returnedUser.IsAdmin {
		return fmt.Errorf("can not make changes to an admin user, attempted to change %s", returnedUser.UserName)
	}
	if isadmin {
		currentUser.IsAdmin = true
		currentUser.Networks = nil
	} else {
		currentUser.Networks = newNetworks
	}

	data, err := json.Marshal(currentUser)
	if err != nil {
		return err
	}
	if err = database.Insert(currentUser.UserName, string(data), database.USERS_TABLE_NAME); err != nil {
		return err
	}

	return nil
}

// UpdateUser - updates a given user
func UpdateUser(userchange models.User, user models.User) (models.User, error) {
	//check if user exists
	if _, err := GetUser(user.UserName); err != nil {
		return models.User{}, err
	}

	err := ValidateUser(userchange)
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
		// encrypt that password so we never see it again
		hash, err := bcrypt.GenerateFromPassword([]byte(userchange.Password), 5)

		if err != nil {
			return userchange, err
		}
		// set password to encrypted password
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
	logger.Log(1, "updated user", queryUser)
	return user, nil
}

// ValidateUser - validates a user model
func ValidateUser(user models.User) error {

	v := validator.New()
	_ = v.RegisterValidation("in_charset", func(fl validator.FieldLevel) bool {
		isgood := user.NameInCharSet()
		return isgood
	})
	err := v.Struct(user)

	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			logger.Log(2, e.Error())
		}
	}

	return err
}

// DeleteUser - deletes a given user
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

// FetchAuthSecret - manages secrets for oauth
func FetchAuthSecret(key string, secret string) (string, error) {
	var record, err = database.FetchRecord(database.GENERATED_TABLE_NAME, key)
	if err != nil {
		if err = database.Insert(key, secret, database.GENERATED_TABLE_NAME); err != nil {
			return "", err
		} else {
			return secret, nil
		}
	}
	return record, nil
}

// GetState - gets an SsoState from DB, if expired returns error
func GetState(state string) (*models.SsoState, error) {
	var s models.SsoState
	record, err := database.FetchRecord(database.SSO_STATE_CACHE, state)
	if err != nil {
		return &s, err
	}

	if err = json.Unmarshal([]byte(record), &s); err != nil {
		return &s, err
	}

	if s.IsExpired() {
		return &s, fmt.Errorf("state expired")
	}

	return &s, nil
}

// SetState - sets a state with new expiration
func SetState(state string) error {
	s := models.SsoState{
		Value:      state,
		Expiration: time.Now().Add(models.DefaultExpDuration),
	}

	data, err := json.Marshal(&s)
	if err != nil {
		return err
	}

	return database.Insert(state, string(data), database.SSO_STATE_CACHE)
}

// IsStateValid - checks if given state is valid or not
// deletes state after call is made to clean up, should only be called once per sign-in
func IsStateValid(state string) (string, bool) {
	s, err := GetState(state)
	if s.Value != "" {
		delState(state)
	}
	return s.Value, err == nil
}

// delState - removes a state from cache/db
func delState(state string) error {
	return database.DeleteRecord(database.SSO_STATE_CACHE, state)
}
