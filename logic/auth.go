package logic

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	DashboardApp       = "dashboard"
	NetclientApp       = "netclient"
	NetmakerDesktopApp = "netmaker-desktop"
)

var IsOAuthConfigured = func() bool { return false }
var ResetAuthProvider = func() {}
var ResetIDPSyncHook = func() {}

// HasSuperAdmin - checks if server has an superadmin/owner
func HasSuperAdmin() (bool, error) {
	return (&schema.User{}).SuperAdminExists(db.WithContext(context.TODO()))
}

// GetUsers - gets users
func GetUsers() ([]models.ReturnUser, error) {
	_users, err := (&schema.User{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	users := make([]models.ReturnUser, len(_users))
	for i, _user := range _users {
		users[i] = ToReturnUser(&_user)
	}
	return users, nil
}

// IsOauthUser - returns
func IsOauthUser(user *schema.User) error {
	var currentValue, err = FetchOAuthSecret()
	if err != nil {
		return err
	}
	var bCryptErr = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentValue))
	return bCryptErr
}

// CreateUser - creates a user
func CreateUser(_user *schema.User) error {
	// check if user exists
	userCheck := &schema.User{Username: _user.Username}
	if err := userCheck.Get(db.WithContext(context.TODO())); err == nil {
		return errors.New("user exists")
	}
	SetUserDefaults(_user)
	if err := IsGroupsValid(_user.UserGroups.Data()); err != nil {
		return errors.New("invalid groups: " + err.Error())
	}

	var err = ValidateUser(_user)
	if err != nil {
		logger.Log(0, "failed to validate user", err.Error())
		return err
	}
	// encrypt that password so we never see it again
	hash, err := bcrypt.GenerateFromPassword([]byte(_user.Password), 5)
	if err != nil {
		logger.Log(0, "error encrypting pass", err.Error())
		return err
	}
	// set password to encrypted password
	_user.Password = string(hash)
	_user.AuthType = schema.BasicAuth
	if IsOauthUser(_user) == nil {
		_user.AuthType = schema.OAuth
	}
	AddGlobalNetRolesToAdmins(_user)
	// create user will always be called either from API or Dashboard.
	_, err = CreateUserJWT(_user.Username, _user.PlatformRoleID, DashboardApp)
	if err != nil {
		logger.Log(0, "failed to generate token", err.Error())
		return err
	}

	dbctx := db.BeginTx(context.TODO())
	commit := false
	defer func() {
		if commit {
			db.FromContext(dbctx).Commit()
		} else {
			db.FromContext(dbctx).Rollback()
		}
	}()

	err = _user.Create(dbctx)
	if err != nil {
		return fmt.Errorf("failed to create user %s: %v", _user.Username, err)
	}

	commit = true
	return nil
}

// CreateSuperAdmin - creates an super admin user
func CreateSuperAdmin(u *schema.User) error {
	hassuperadmin, err := HasSuperAdmin()
	if err != nil {
		return err
	}
	if hassuperadmin {
		return errors.New("superadmin user already exists")
	}
	u.PlatformRoleID = schema.SuperAdminRole
	return CreateUser(u)
}

// VerifyAuthRequest - verifies an auth request
func VerifyAuthRequest(authRequest models.UserAuthParams, appName string) (string, error) {
	if authRequest.UserName == "" {
		return "", errors.New("username can't be empty")
	} else if authRequest.Password == "" {
		return "", errors.New("password can't be empty")
	}
	// Search DB for node with Mac Address. Ignore pending nodes (they should not be able to authenticate with API until approved).
	_user := &schema.User{
		Username: authRequest.UserName,
	}
	err := _user.Get(DefaultScope(db.WithContext(context.TODO())))
	if err != nil {
		return "", errors.New("incorrect credentials")
	}

	// compare password from request to stored password in database
	// might be able to have a common hash (certificates?) and compare those so that a password isn't passed in in plain text...
	// TODO: Consider a way of hashing the password client side before sending, or using certificates
	if err = bcrypt.CompareHashAndPassword([]byte(_user.Password), []byte(authRequest.Password)); err != nil {
		return "", errors.New("incorrect credentials")
	}

	if _user.IsMFAEnabled {
		tokenString, err := CreatePreAuthToken(authRequest.UserName)
		if err != nil {
			slog.Error("error creating jwt", "error", err)
			return "", err
		}

		return tokenString, nil
	} else {
		// Create a new JWT for the node
		tokenString, err := CreateUserJWT(authRequest.UserName, schema.UserRoleID(_user.PlatformRoleID), appName)
		if err != nil {
			slog.Error("error creating jwt", "error", err)
			return "", err
		}

		// update last login time
		_user.LastLoginAt = time.Now().UTC()
		err = _user.Update(db.WithContext(context.TODO()))
		if err != nil {
			slog.Error("error upserting user", "error", err)
			return "", err
		}

		return tokenString, nil
	}
}

// UpsertUser - updates user in the db
func UpsertUser(_user schema.User) error {
	_existingUser := schema.User{Username: _user.Username}
	// Check if user exists to preserve ID
	err := _existingUser.Get(db.WithContext(context.TODO()))
	if err == nil {
		_user.ID = _existingUser.ID
		return _user.Update(db.WithContext(context.TODO()))
	}

	return _user.Create(db.WithContext(context.TODO()))
}

// preserveExternalUserGroups copies IdP-managed group membership from the existing
// user onto the update payload so external groups are not dropped when the UI
// omits them (e.g. role-only updates).
func preserveExternalUserGroups(existing, change *schema.User) {
	for groupID := range existing.UserGroups.Data() {
		group, err := GetUserGroup(groupID)
		if err != nil || group.ExternalIdentityProviderID == "" {
			continue
		}
		change.UserGroups.Data()[groupID] = struct{}{}
	}
}

// UpdateUser - updates a given user
func UpdateUser(userchange, _user *schema.User) (*schema.User, error) {
	// check if user exists
	userCheck := &schema.User{Username: _user.Username}
	if err := userCheck.Get(db.WithContext(context.TODO())); err != nil {
		return &schema.User{}, err
	}

	queryUser := _user.Username
	if userchange.Username != "" && _user.Username != userchange.Username {
		// check if username is available
		userCheck := &schema.User{Username: userchange.Username}
		if err := userCheck.Get(db.WithContext(context.TODO())); err == nil {
			return &schema.User{}, errors.New("username exists already")
		}
		if userchange.Username == MasterUser {
			return &schema.User{}, errors.New("username not allowed")
		}

		_user.Username = userchange.Username
	}
	if userchange.Password != "" {
		if len(userchange.Password) < 5 {
			return &schema.User{}, errors.New("password requires min 5 characters")
		}
		// encrypt that password so we never see it again
		hash, err := bcrypt.GenerateFromPassword([]byte(userchange.Password), 5)

		if err != nil {
			return userchange, err
		}
		// set password to encrypted password
		userchange.Password = string(hash)

		_user.Password = userchange.Password
	}

	validUserGroups := make(map[schema.UserGroupID]struct{})
	for userGroupID := range userchange.UserGroups.Data() {
		_, err := GetUserGroup(userGroupID)
		if err == nil {
			validUserGroups[userGroupID] = struct{}{}
		}
	}

	userchange.UserGroups = datatypes.NewJSONType(validUserGroups)

	oldRole := _user.PlatformRoleID
	newRole := userchange.PlatformRoleID
	if newRole == "" {
		newRole = oldRole
	}
	AddGlobalGroupOnRoleUpgrade(oldRole, newRole, userchange.UserGroups.Data())
	preserveExternalUserGroups(_user, userchange)
	if oldRole != newRole {
		for groupID := range _user.UserGroups.Data() {
			userchange.UserGroups.Data()[groupID] = struct{}{}
		}
	}

	if userchange.DisplayName != "" {
		if _user.ExternalIdentityProviderID != "" &&
			_user.DisplayName != userchange.DisplayName {
			return userchange, errors.New("display name cannot be updated for external user")
		}

		_user.DisplayName = userchange.DisplayName
	}

	if _user.ExternalIdentityProviderID != "" &&
		userchange.AccountDisabled != _user.AccountDisabled {
		return userchange, errors.New("account status cannot be updated for external user")
	}

	// Reset Gw Access for service users
	go UpdateUserGwAccess(_user, userchange)
	if userchange.PlatformRoleID != "" {
		_user.PlatformRoleID = userchange.PlatformRoleID
	}

	for groupID := range userchange.UserGroups.Data() {
		_, ok := _user.UserGroups.Data()[groupID]
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

	for groupID := range _user.UserGroups.Data() {
		_, ok := userchange.UserGroups.Data()[groupID]
		if !ok {
			if newRole == schema.Auditor {
				continue
			}
			group, err := GetUserGroup(groupID)
			if err != nil {
				return userchange, err
			}

			if group.ExternalIdentityProviderID != "" {
				return userchange, errors.New("cannot modify membership of external groups")
			}
		}
	}

	var updateMFA bool
	if _user.IsMFAEnabled != userchange.IsMFAEnabled {
		updateMFA = true
	}

	_user.IsMFAEnabled = userchange.IsMFAEnabled

	var updateAccountStatus bool
	if _user.AccountDisabled != userchange.AccountDisabled {
		updateAccountStatus = true
	}

	_user.IsMFAEnabled = userchange.IsMFAEnabled
	if !_user.IsMFAEnabled {
		_user.TOTPSecret = ""
	}

	_user.UserGroups = userchange.UserGroups
	err := ValidateUser(_user)
	if err != nil {
		return &schema.User{}, err
	}

	dbctx := db.BeginTx(context.TODO())
	commit := false
	defer func() {
		if commit {
			db.FromContext(dbctx).Commit()
			logger.Log(1, "updated user", queryUser)
		} else {
			db.FromContext(dbctx).Rollback()
		}
	}()

	// Fetch existing user to get ID
	_schemaUser := schema.User{Username: queryUser}
	err = _schemaUser.Get(dbctx)
	if err != nil {
		return &schema.User{}, err
	}

	_user.ID = _schemaUser.ID

	err = _user.Update(dbctx)
	if err != nil {
		return &schema.User{}, err
	}

	if updateAccountStatus {
		err = _user.UpdateAccountStatus(dbctx)
		if err != nil {
			return &schema.User{}, err
		}
	}

	if updateMFA {
		err = _user.UpdateMFA(dbctx)
		if err != nil {
			return &schema.User{}, err
		}
	}

	commit = true
	return _user, nil
}

func validateUserName(user *schema.User) error {
	var validationErr error

	if len(user.Username) == 0 {
		validationErr = errors.Join(validationErr, errors.New("username cannot be empty"))
	} else if len(user.Username) <= 3 {
		validationErr = errors.Join(validationErr, errors.New("username must have more than 3 characters"))
	}

	var isValidEmail bool
	_, err := mail.ParseAddress(user.Username)
	if err == nil {
		isValidEmail = true
	}

	if !isValidEmail {
		charset := "abcdefghijklmnopqrstuvwxyz1234567890-."
		for _, char := range user.Username {
			if !strings.Contains(charset, strings.ToLower(string(char))) {
				validationErr = errors.Join(validationErr, errors.New("invalid character(s) in username"))
				break
			}
		}
	}
	return validationErr
}

// ValidateUser - validates a user model
func ValidateUser(user *schema.User) error {
	var validationErr error
	// check if role is valid
	roleCheck := &schema.UserRole{ID: user.PlatformRoleID}
	err := roleCheck.Get(db.WithContext(context.TODO()))
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		validationErr = errors.Join(validationErr, fmt.Errorf("invalid user role %s", user.PlatformRoleID))
	}

	err = validateUserName(user)
	if err != nil {
		validationErr = errors.Join(validationErr, err)
	}

	if len(user.Password) < 5 {
		validationErr = errors.Join(validationErr, errors.New("password must have a minimum of 5 characters"))
	}

	return validationErr
}

// DeleteUser - deletes a given user
func DeleteUser(user string) error {
	_user := schema.User{
		Username: user,
	}
	err := _user.Delete(db.WithContext(context.TODO()))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user does not exist")
		}

		return err
	}

	RemoveUserFromAclPolicy(user)
	return (&schema.UserAccessToken{UserName: user}).DeleteAllUserTokens(db.WithContext(context.TODO()))
}

func SetOAuthSecret(secret string) error {
	oauthSecret := &schema.Internal{
		Key: schema.InternalKey_OAuthSecret,
	}
	err := oauthSecret.Get(db.WithContext(context.TODO()))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if oauthSecret.Value != "" {
		return nil
	}

	oauthSecret.Value = base64.StdEncoding.EncodeToString([]byte(secret))
	ctx := db.WithContext(context.TODO())
	if oauthSecret.TenantID == "" {
		oauthSecret.TenantID = scope.ID(DefaultScope(ctx))
	}
	return oauthSecret.Set(ctx)
}

// FetchOAuthSecret fetches secrets for oauth
func FetchOAuthSecret() (string, error) {
	oauthSecret := &schema.Internal{
		Key: schema.InternalKey_OAuthSecret,
	}
	err := oauthSecret.Get(db.WithContext(context.TODO()))
	if err != nil {
		return "", err
	}

	oauthSecretValue, err := base64.StdEncoding.DecodeString(oauthSecret.Value)
	if err != nil {
		return "", err
	}

	return string(oauthSecretValue), nil
}

// GetState - gets an SsoState from DB, if expired returns error
func GetState(state string) (*models.SsoState, error) {
	r := &schema.SsoStateRecord{Key: state}
	if err := r.Get(db.WithContext(context.TODO())); err != nil {
		return nil, err
	}
	s := r.Value.Data()
	if s.IsExpired() {
		return &s, fmt.Errorf("state expired")
	}
	return &s, nil
}

// SetState - sets a state with new expiration
func SetState(appName, state string) error {
	s := models.SsoState{
		AppName:    appName,
		Value:      state,
		Expiration: time.Now().Add(models.DefaultExpDuration),
	}
	r := &schema.SsoStateRecord{Key: state, Value: datatypes.NewJSONType(s)}
	ctx := db.WithContext(context.TODO())
	if r.TenantID == "" {
		r.TenantID = scope.ID(DefaultScope(ctx))
	}
	return r.Upsert(ctx)
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
	return (&schema.SsoStateRecord{Key: state}).Delete(db.WithContext(context.TODO()))
}

// CleanExpiredSSOStates removes expired SSO state entries from the database
// to prevent unbounded table growth that degrades FetchRecord performance.
func CleanExpiredSSOStates() error {
	records, err := (&schema.SsoStateRecord{}).List(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}
	for _, r := range records {
		s := r.Value.Data()
		if s.IsExpired() {
			_ = (&schema.SsoStateRecord{Key: r.Key}).Delete(db.WithContext(context.TODO()))
		}
	}
	return nil
}

// AddSSOStateCleanupHook registers a periodic cleanup of expired SSO states
func AddSSOStateCleanupHook() {
	HookManagerCh <- models.HookDetails{
		ID:       "sso-state-cleanup",
		Hook:     WrapHook(CleanExpiredSSOStates),
		Interval: 15 * time.Minute,
	}
}
