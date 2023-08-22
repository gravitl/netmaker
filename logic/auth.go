package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
)

// HasSuperAdmin - checks if server has an superadmin/owner
func HasSuperAdmin() (bool, error) {

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
		if user.IsSuperAdmin {
			return true, nil
		}
	}

	return false, err
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
func CreateUser(user *models.User) error {
	// check if user exists
	if _, err := GetUser(user.UserName); err == nil {
		return errors.New("user exists")
	}
	var err = ValidateUser(user)
	if err != nil {
		return err
	}

	// encrypt that password so we never see it again
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), 5)
	if err != nil {
		return err
	}
	// set password to encrypted password
	user.Password = string(hash)

	tokenString, _ := CreateUserJWT(user.UserName, user.IsSuperAdmin, user.IsAdmin)
	if tokenString == "" {
		return err
	}

	SetUserDefaults(user)

	// connect db
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	err = database.Insert(user.UserName, string(data), database.USERS_TABLE_NAME)
	if err != nil {
		return err
	}

	return nil
}

// CreateSuperAdmin - creates an super admin user
func CreateSuperAdmin(u *models.User) error {
	hassuperadmin, err := HasSuperAdmin()
	if err != nil {
		return err
	}
	if hassuperadmin {
		return errors.New("superadmin user already exists")
	}
	u.IsSuperAdmin = true
	return CreateUser(u)
}

// VerifyAuthRequest - verifies an auth request
func VerifyAuthRequest(authRequest models.UserAuthParams) (string, error) {
	var result models.User
	if authRequest.UserName == "" {
		return "", errors.New("username can't be empty")
	} else if authRequest.Password == "" {
		return "", errors.New("password can't be empty")
	}
	// Search DB for node with Mac Address. Ignore pending nodes (they should not be able to authenticate with API until approved).
	record, err := database.FetchRecord(database.USERS_TABLE_NAME, authRequest.UserName)
	if err != nil {
		return "", errors.New("error retrieving user from db: " + err.Error())
	}
	if err = json.Unmarshal([]byte(record), &result); err != nil {
		return "", errors.New("error unmarshalling user json: " + err.Error())
	}

	// compare password from request to stored password in database
	// might be able to have a common hash (certificates?) and compare those so that a password isn't passed in in plain text...
	// TODO: Consider a way of hashing the password client side before sending, or using certificates
	if err = bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(authRequest.Password)); err != nil {
		return "", errors.New("incorrect credentials")
	}

	// Create a new JWT for the node
	tokenString, _ := CreateUserJWT(authRequest.UserName, result.IsSuperAdmin, result.IsAdmin)
	return tokenString, nil
}

// UpsertUser - updates user in the db
func UpsertUser(user models.User) error {
	data, err := json.Marshal(&user)
	if err != nil {
		return err
	}
	if err = database.Insert(user.UserName, string(data), database.USERS_TABLE_NAME); err != nil {
		return err
	}
	return nil
}

// UpdateUser - updates a given user
func UpdateUser(userchange, user *models.User) (*models.User, error) {
	// check if user exists
	if _, err := GetUser(user.UserName); err != nil {
		return &models.User{}, err
	}

	queryUser := user.UserName

	if userchange.UserName != "" {
		// check if username is available
		if _, err := GetUser(userchange.UserName); err == nil {
			return &models.User{}, errors.New("username exists already")
		}
		user.UserName = userchange.UserName
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
	user.IsAdmin = userchange.IsAdmin

	err := ValidateUser(user)
	if err != nil {
		return &models.User{}, err
	}

	if err = database.DeleteRecord(database.USERS_TABLE_NAME, queryUser); err != nil {
		return &models.User{}, err
	}
	data, err := json.Marshal(&user)
	if err != nil {
		return &models.User{}, err
	}
	if err = database.Insert(user.UserName, string(data), database.USERS_TABLE_NAME); err != nil {
		return &models.User{}, err
	}
	logger.Log(1, "updated user", queryUser)
	return user, nil
}

// ValidateUser - validates a user model
func ValidateUser(user *models.User) error {

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
	if err != nil {
		logger.Log(2, "error retrieving oauth state:", err.Error())
		return "", false
	}
	if s.Value != "" {
		if err = delState(state); err != nil {
			logger.Log(2, "error deleting oauth state:", err.Error())
			return "", false
		}
	}
	return s.Value, true
}

// delState - removes a state from cache/db
func delState(state string) error {
	return database.DeleteRecord(database.SSO_STATE_CACHE, state)
}
