package logic

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/gravitl/netmaker/scope"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	DashboardApp       = "dashboard"
	NetclientApp       = "netclient"
	NetmakerDesktopApp = "netmaker-desktop"
)

var IsOAuthConfigured = func(context.Context) bool { return false }
var ResetAuthProvider = func(context.Context) {}
var ResetIDPSyncHook = func(context.Context) {}

type CleanupUserRefsFunc func(ctx context.Context, username string, forceDeleteConfigs bool)

func ResolveInheritedAuth(ctx context.Context, user *schema.User) error {
	if scope.Level(ctx) != scope.TenantScope || user.AuthType != schema.Inherited {
		return nil
	}

	tenant := &schema.Tenant{ID: scope.ID(ctx)}
	err := tenant.Get(ctx)
	if err != nil {
		return err
	}

	orgMembership := &schema.OrgMembership{
		OrganizationID: tenant.OrganizationID,
		UserID:         user.ID,
	}
	err = orgMembership.Get(ctx)
	if err != nil {
		return err
	}

	user.AuthType = orgMembership.AuthType
	user.Password = orgMembership.Password
	user.ExternalIdentityProviderID = orgMembership.ExternalIdentityProviderID
	user.IsMFAEnabled = orgMembership.IsMFAEnabled
	user.TOTPSecret = orgMembership.TOTPSecret
	return nil
}

// IsOauthUser - returns
func IsOauthUser(ctx context.Context, user *schema.User) error {
	var currentValue, err = FetchOAuthSecret(ctx)
	if err != nil {
		return err
	}
	var bCryptErr = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentValue))
	return bCryptErr
}

// VerifyAuthRequest - verifies an auth request
func VerifyAuthRequest(ctx context.Context, authRequest models.UserAuthParams, appName string) (string, error) {
	if authRequest.UserName == "" {
		return "", errors.New("username can't be empty")
	} else if authRequest.Password == "" {
		return "", errors.New("password can't be empty")
	}
	// Search DB for node with Mac Address. Ignore pending nodes (they should not be able to authenticate with API until approved).
	_user := &schema.User{
		Username: authRequest.UserName,
	}
	err := _user.GetWithMembership(ctx)
	if err != nil {
		return "", errors.New("incorrect credentials")
	}

	err = ResolveInheritedAuth(ctx, _user)
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
		tokenString, err := CreatePreAuthToken(ctx, authRequest.UserName)
		if err != nil {
			slog.Error("error creating jwt", "error", err)
			return "", err
		}

		return tokenString, nil
	} else {
		// Create a new JWT for the node
		tokenString, err := CreateUserJWT(ctx, authRequest.UserName, appName)
		if err != nil {
			slog.Error("error creating jwt", "error", err)
			return "", err
		}

		// update last login time
		_user.LastLoginAt = time.Now().UTC()
		err = _user.Update(ctx)
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
func preserveExternalUserGroups(ctx context.Context, existing, change *schema.User) {
	for groupID := range existing.UserGroups.Data() {
		group, err := GetUserGroup(ctx, groupID)
		if err != nil || group.ExternalIdentityProviderID == "" {
			continue
		}
		change.UserGroups.Data()[groupID] = struct{}{}
	}
}

// UpdateUser - updates a given user
func UpdateUser(ctx context.Context, userchange, _user *schema.User) (*schema.User, error) {
	// check if user exists
	userCheck := &schema.User{Username: _user.Username}
	if err := userCheck.Get(ctx); err != nil {
		return &schema.User{}, err
	}

	queryUser := _user.Username
	if userchange.Username != "" && _user.Username != userchange.Username {
		// check if username is available
		userCheck := &schema.User{Username: userchange.Username}
		if err := userCheck.Get(ctx); err == nil {
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
		_, err := GetUserGroup(ctx, userGroupID)
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
	preserveExternalUserGroups(ctx, _user, userchange)
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
	detachedCtx := scope.WithContext(db.WithContext(context.Background()), scope.Level(ctx), scope.ID(ctx))
	go UpdateUserGwAccess(detachedCtx, _user, userchange)
	if userchange.PlatformRoleID != "" {
		_user.PlatformRoleID = userchange.PlatformRoleID
	}

	for groupID := range userchange.UserGroups.Data() {
		_, ok := _user.UserGroups.Data()[groupID]
		if !ok {
			group, err := GetUserGroup(ctx, groupID)
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
			group, err := GetUserGroup(ctx, groupID)
			if err != nil {
				return userchange, err
			}

			if group.ExternalIdentityProviderID != "" {
				return userchange, errors.New("cannot modify membership of external groups")
			}
		}
	}

	_user.IsMFAEnabled = userchange.IsMFAEnabled
	if !_user.IsMFAEnabled {
		_user.TOTPSecret = ""
	}

	groupsChanged := !CompareMaps(_user.UserGroups.Data(), userchange.UserGroups.Data())

	_user.AccountDisabled = userchange.AccountDisabled
	_user.UserGroups = userchange.UserGroups
	err := ValidateUser(_user)
	if err != nil {
		return &schema.User{}, err
	}

	// Fetch existing user to get ID
	_schemaUser := schema.User{Username: queryUser}
	err = _schemaUser.Get(ctx)
	if err != nil {
		return &schema.User{}, err
	}

	_user.ID = _schemaUser.ID

	err = _user.Update(ctx)
	if err != nil {
		return &schema.User{}, err
	}

	err = _user.UpsertMembership(ctx)
	if err != nil {
		return &schema.User{}, err
	}

	if groupsChanged {
		go RunPostureChecksForTenant(detachedCtx)
	}

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
	err := roleCheck.GetPlatformRole(db.WithContext(context.TODO()))
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

	return validationErr
}

func IsIDPUser(ctx context.Context, user *schema.User) bool {
	if scope.Level(ctx) == scope.TenantScope {
		if user.AuthType == schema.OAuth && IsSyncEnabled(ctx) {
			return true
		}
	}

	return false
}

func DeleteTenantUser(ctx context.Context, user *schema.User, forceDeleteConfigs bool, cleanup CleanupUserRefsFunc) error {
	err := (&schema.TenantMembership{TenantID: scope.ID(ctx), UserID: user.ID}).Delete(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user does not exist")
		}

		return err
	}

	RemoveUserFromAclPolicy(ctx, user.Username)

	if err := (&schema.UserAccessToken{UserName: user.Username}).DeleteAllUserTokens(ctx); err != nil {
		return err
	}

	if cleanup != nil {
		cleanupCtx := scope.WithContext(db.WithContext(context.Background()), scope.TenantScope, scope.ID(ctx))
		go cleanup(cleanupCtx, user.Username, forceDeleteConfigs)
	}

	return nil
}

func DeleteOrgUser(ctx context.Context, user *schema.User, forceDeleteConfigs bool, cleanup CleanupUserRefsFunc) error {
	err := (&schema.OrgMembership{OrganizationID: scope.ID(ctx), UserID: user.ID}).Delete(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user does not exist")
		}

		return err
	}

	memberships, err := (&schema.TenantMembership{UserID: user.ID}).ListByUserID(ctx)
	if err != nil {
		return err
	}

	for _, membership := range memberships {
		if membership.AuthType != schema.Inherited {
			continue
		}
		tenantCtx := scope.WithContext(ctx, scope.TenantScope, membership.TenantID)
		if err := DeleteTenantUser(tenantCtx, user, forceDeleteConfigs, cleanup); err != nil {
			return err
		}
	}

	return nil
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
	return oauthSecret.Set(db.WithContext(context.TODO()))
}

// FetchOAuthSecret fetches secrets for oauth
func FetchOAuthSecret(ctx context.Context) (string, error) {
	oauthSecret := &schema.Internal{
		Key: schema.InternalKey_OAuthSecret,
	}
	err := oauthSecret.Get(ctx)
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
func SetState(scope scope.Scope, scopeID, appName, state string) error {
	s := models.SsoState{
		Scope:      scope,
		ScopeID:    scopeID,
		AppName:    appName,
		Value:      state,
		Expiration: time.Now().Add(models.DefaultExpDuration),
	}
	r := &schema.SsoStateRecord{Key: state, Value: datatypes.NewJSONType(s)}
	return r.Upsert(db.WithContext(context.TODO()))
}

// GetLoginMethodsForUser returns available login options for the given username.
// Returns an empty slice (not an error) when the user is not found.
func GetLoginMethodsForUser(ctx context.Context, username string) ([]models.LoginOption, error) {
	user := &schema.User{Username: username}
	err := user.Get(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []models.LoginOption{}, nil
		}
		return []models.LoginOption{}, err
	}

	var options []models.LoginOption
	tenantMemberships, err := (&schema.TenantMembership{
		UserID: user.ID,
	}).ListByUserID(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing tenant memberships: %w", err)
	}

	for _, membership := range tenantMemberships {
		tenant := &schema.Tenant{ID: membership.TenantID}
		err = tenant.Get(ctx)
		if err != nil {
			continue
		}

		settings := &schema.TenantSettingsRecord{Key: membership.TenantID}
		err = settings.Get(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				settings.Value = datatypes.NewJSONType(schema.TenantSettings{})
			} else {
				continue
			}
		}

		var methodsAvailable models.LoginMethodsAvailable
		switch membership.AuthType {
		case schema.BasicAuth:
			methodsAvailable.BasicAuth = true
		case schema.OAuth:
			methodsAvailable.SSO = true
			methodsAvailable.SSOProvider = settings.Value.Data().AuthProvider
		case schema.Inherited:
			orgMembership := &schema.OrgMembership{
				OrganizationID: tenant.OrganizationID,
				UserID:         user.ID,
			}
			err = orgMembership.Get(ctx)
			if err != nil {
				continue
			}

			methodsAvailable.OrgAuth = true
			methodsAvailable.OrganizationID = tenant.OrganizationID
			if orgMembership.AuthType == schema.BasicAuth {
				methodsAvailable.BasicAuth = true
			} else if orgMembership.AuthType == schema.OAuth {
				orgSettings := &schema.OrganizationSettings{
					ID: tenant.OrganizationID,
				}
				err = orgSettings.Get(ctx)
				if err != nil {
					continue
				}

				methodsAvailable.SSO = true
				methodsAvailable.SSOProvider = orgSettings.Settings.Data().AuthProvider
			} else {
				continue
			}
		}

		options = append(options, models.LoginOption{
			Scope:         scope.TenantScope,
			ScopeID:       tenant.ID,
			ScopeName:     tenant.Name,
			ScopeSlug:     tenant.Slug,
			ScopeMetadata: tenant.Metadata,
			Methods:       methodsAvailable,
		})
	}

	var orgMemberships []schema.OrgMembership
	if IsMSP(ctx) {
		orgMemberships, err = (&schema.OrgMembership{
			UserID: user.ID,
		}).ListByUserID(ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing org memberships: %w", err)
		}
	}

	for _, membership := range orgMemberships {
		org := &schema.Organization{ID: membership.OrganizationID}
		err = org.Get(ctx)
		if err != nil {
			continue
		}

		settings := &schema.OrganizationSettings{ID: membership.OrganizationID}
		err = settings.Get(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				settings.Settings = datatypes.NewJSONType(schema.OrganizationSettingsData{})
			} else {
				continue
			}
		}

		var methodsAvailable models.LoginMethodsAvailable
		switch membership.AuthType {
		case schema.BasicAuth:
			methodsAvailable.BasicAuth = true
		case schema.OAuth:
			methodsAvailable.SSO = true
			methodsAvailable.SSOProvider = settings.Settings.Data().AuthProvider
		default:
			continue
		}

		options = append(options, models.LoginOption{
			Scope:         scope.OrgScope,
			ScopeID:       org.ID,
			ScopeName:     org.Name,
			ScopeSlug:     org.Slug,
			ScopeMetadata: org.Metadata,
			Methods:       methodsAvailable,
		})
	}

	if options == nil {
		options = []models.LoginOption{}
	}
	return options, nil
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
