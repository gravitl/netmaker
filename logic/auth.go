package logic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
)

const (
	auth_key = "netmaker_auth"
)

var (
	superUser = models.User{}
)

func ClearSuperUserCache() {
	superUser = models.User{}
}

var ResetAuthProvider = func() {}
var ResetIDPSyncHook = func() {}

// HasSuperAdmin - checks if server has an superadmin/owner
func HasSuperAdmin() (bool, error) {

	if superUser.IsSuperAdmin {
		return true, nil
	}

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
		if user.PlatformRoleID == models.SuperAdminRole || user.IsSuperAdmin {
			return true, nil
		}
	}

	return false, err
}

// GetUsersDB - gets users
func GetUsersDB() ([]models.User, error) {

	var users []models.User

	collection, err := database.FetchRecords(database.USERS_TABLE_NAME)

	if err != nil {
		return users, err
	}

	for _, value := range collection {

		var user models.User
		err = json.Unmarshal([]byte(value), &user)
		if err != nil {
			continue // get users
		}
		users = append(users, user)
	}

	return users, err
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

// IsOauthUser - returns
func IsOauthUser(user *models.User) error {
	var currentValue, err = FetchPassValue("")
	if err != nil {
		return err
	}
	var bCryptErr = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentValue))
	return bCryptErr
}

func FetchPassValue(newValue string) (string, error) {

	type valueHolder struct {
		Value string `json:"value" bson:"value"`
	}
	newValueHolder := valueHolder{}
	var currentValue, err = FetchAuthSecret()
	if err != nil {
		return "", err
	}
	var unmarshErr = json.Unmarshal([]byte(currentValue), &newValueHolder)
	if unmarshErr != nil {
		return "", unmarshErr
	}

	var b64CurrentValue, b64Err = base64.StdEncoding.DecodeString(newValueHolder.Value)
	if b64Err != nil {
		logger.Log(0, "could not decode pass")
		return "", nil
	}
	return string(b64CurrentValue), nil
}

// CreateUser - creates a user
func CreateUser(user *models.User) error {
	// check if user exists
	if _, err := GetUser(user.UserName); err == nil {
		return errors.New("user exists")
	}
	SetUserDefaults(user)
	if err := IsGroupsValid(user.UserGroups); err != nil {
		return errors.New("invalid groups: " + err.Error())
	}
	if err := IsNetworkRolesValid(user.NetworkRoles); err != nil {
		return errors.New("invalid network roles: " + err.Error())
	}

	var err = ValidateUser(user)
	if err != nil {
		logger.Log(0, "failed to validate user", err.Error())
		return err
	}
	// encrypt that password so we never see it again
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), 5)
	if err != nil {
		logger.Log(0, "error encrypting pass", err.Error())
		return err
	}
	// set password to encrypted password
	user.Password = string(hash)
	user.AuthType = models.BasicAuth
	if IsOauthUser(user) == nil {
		user.AuthType = models.OAuth
	}
	AddGlobalNetRolesToAdmins(user)
	_, err = CreateUserJWT(user.UserName, user.PlatformRoleID)
	if err != nil {
		logger.Log(0, "failed to generate token", err.Error())
		return err
	}

	// connect db
	data, err := json.Marshal(user)
	if err != nil {
		logger.Log(0, "failed to marshal", err.Error())
		return err
	}
	err = database.Insert(user.UserName, string(data), database.USERS_TABLE_NAME)
	if err != nil {
		logger.Log(0, "failed to insert user", err.Error())
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
	u.PlatformRoleID = models.SuperAdminRole
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
		return "", errors.New("incorrect credentials")
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
	tokenString, err := CreateUserJWT(authRequest.UserName, result.PlatformRoleID)
	if err != nil {
		slog.Error("error creating jwt", "error", err)
		return "", err
	}

	// update last login time
	result.LastLoginTime = time.Now().UTC()
	err = UpsertUser(result)
	if err != nil {
		slog.Error("error upserting user", "error", err)
		return "", err
	}

	return tokenString, nil
}

// UpsertUser - updates user in the db
func UpsertUser(user models.User) error {
	data, err := json.Marshal(&user)
	if err != nil {
		slog.Error("error marshalling user", "user", user.UserName, "error", err.Error())
		return err
	}
	if err = database.Insert(user.UserName, string(data), database.USERS_TABLE_NAME); err != nil {
		slog.Error("error inserting user", "user", user.UserName, "error", err.Error())
		return err
	}
	if user.IsSuperAdmin {
		superUser = user
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
	if userchange.UserName != "" && user.UserName != userchange.UserName {
		// check if username is available
		if _, err := GetUser(userchange.UserName); err == nil {
			return &models.User{}, errors.New("username exists already")
		}
		if userchange.UserName == MasterUser {
			return &models.User{}, errors.New("username not allowed")
		}

		user.UserName = userchange.UserName
	}
	if userchange.Password != "" {
		if len(userchange.Password) < 5 {
			return &models.User{}, errors.New("password requires min 5 characters")
		}
		// encrypt that password so we never see it again
		hash, err := bcrypt.GenerateFromPassword([]byte(userchange.Password), 5)

		if err != nil {
			return userchange, err
		}
		// set password to encrypted password
		userchange.Password = string(hash)

		user.Password = userchange.Password
	}
	if err := IsGroupsValid(userchange.UserGroups); err != nil {
		return userchange, errors.New("invalid groups: " + err.Error())
	}
	if err := IsNetworkRolesValid(userchange.NetworkRoles); err != nil {
		return userchange, errors.New("invalid network roles: " + err.Error())
	}

	if userchange.DisplayName != "" {
		if user.ExternalIdentityProviderID != "" &&
			user.DisplayName != userchange.DisplayName {
			return userchange, errors.New("display name cannot be updated for external user")
		}

		user.DisplayName = userchange.DisplayName
	}

	if user.ExternalIdentityProviderID != "" &&
		userchange.AccountDisabled != user.AccountDisabled {
		return userchange, errors.New("account status cannot be updated for external user")
	}

	// Reset Gw Access for service users
	go UpdateUserGwAccess(*user, *userchange)
	if userchange.PlatformRoleID != "" {
		user.PlatformRoleID = userchange.PlatformRoleID
	}

	for groupID := range userchange.UserGroups {
		_, ok := user.UserGroups[groupID]
		if !ok {
			group, err := GetUserGroup(groupID)
			if err != nil {
				return userchange, err
			}

			if group.ExternalIdentityProviderID != "" {
				return userchange, errors.New("cannot modify membership of external groups")
			}
		}
	}

	for groupID := range user.UserGroups {
		_, ok := userchange.UserGroups[groupID]
		if !ok {
			group, err := GetUserGroup(groupID)
			if err != nil {
				return userchange, err
			}

			if group.ExternalIdentityProviderID != "" {
				return userchange, errors.New("cannot modify membership of external groups")
			}
		}
	}

	user.UserGroups = userchange.UserGroups
	user.NetworkRoles = userchange.NetworkRoles
	AddGlobalNetRolesToAdmins(user)
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

	// check if role is valid
	_, err := GetRole(user.PlatformRoleID)
	if err != nil {
		return errors.New("failed to fetch platform role " + user.PlatformRoleID.String())
	}
	v := validator.New()
	_ = v.RegisterValidation("in_charset", func(fl validator.FieldLevel) bool {
		isgood := user.NameInCharSet()
		return isgood
	})
	err = v.Struct(user)

	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			logger.Log(2, e.Error())
		}
	}

	return err
}

// DeleteUser - deletes a given user
func DeleteUser(user string) error {

	if userRecord, err := database.FetchRecord(database.USERS_TABLE_NAME, user); err != nil || len(userRecord) == 0 {
		return errors.New("user does not exist")
	}

	err := database.DeleteRecord(database.USERS_TABLE_NAME, user)
	if err != nil {
		return err
	}
	go RemoveUserFromAclPolicy(user)
	return (&schema.UserAccessToken{UserName: user}).DeleteAllUserTokens(db.WithContext(context.TODO()))
}

func SetAuthSecret(secret string) error {
	type valueHolder struct {
		Value string `json:"value" bson:"value"`
	}
	record, err := FetchAuthSecret()
	if err == nil {
		v := valueHolder{}
		json.Unmarshal([]byte(record), &v)
		if v.Value != "" {
			return nil
		}
	}
	var b64NewValue = base64.StdEncoding.EncodeToString([]byte(secret))
	newValueHolder := valueHolder{
		Value: b64NewValue,
	}
	d, _ := json.Marshal(newValueHolder)
	return database.Insert(auth_key, string(d), database.GENERATED_TABLE_NAME)
}

// FetchAuthSecret - manages secrets for oauth
func FetchAuthSecret() (string, error) {
	var record, err = database.FetchRecord(database.GENERATED_TABLE_NAME, auth_key)
	if err != nil {
		return "", err
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
