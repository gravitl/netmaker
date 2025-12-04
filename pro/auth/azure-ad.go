package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

const AzureAD_TIMEOUT = 10 * time.Second

var azure_ad_functions = map[string]interface{}{
	init_provider:   initAzureAD,
	get_user_info:   getAzureUserInfo,
	handle_callback: handleAzureCallback,
	handle_login:    handleAzureLogin,
	verify_user:     verifyAzureUser,
}

var azure_ad_verifier *oidc.IDTokenVerifier

// == handle azure ad authentication here ==

func initAzureAD(redirectURL string, clientID string, clientSecret string) {
	tenantID := logic.GetAzureTenant()
	if tenantID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), AzureAD_TIMEOUT)
		defer cancel()

		issuer := fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", tenantID)
		provider, err := oidc.NewProvider(ctx, issuer)
		if err != nil {
			logger.Log(1, "error when initializing Azure AD OIDC provider:", err.Error())
		} else {
			azure_ad_verifier = provider.Verifier(&oidc.Config{ClientID: clientID})
		}
	}

	auth_provider = &oauth2.Config{
		RedirectURL:  redirectURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"User.Read", "email", "profile", "openid"},
		Endpoint:     microsoft.AzureADEndpoint(tenantID),
	}
}

func handleAzureLogin(w http.ResponseWriter, r *http.Request) {
	appName := r.Header.Get("X-Application-Name")
	if appName == "" {
		appName = logic.NetmakerDesktopApp
	}

	var oauth_state_string = logic.RandomString(user_signin_length)
	if auth_provider == nil {
		handleOauthNotConfigured(w)
		return
	}

	if err := logic.SetState(appName, oauth_state_string); err != nil {
		handleOauthNotConfigured(w)
		return
	}

	var url = auth_provider.AuthCodeURL(oauth_state_string)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleAzureCallback(w http.ResponseWriter, r *http.Request) {
	var rState, rCode = getStateAndCode(r)
	state, err := logic.GetState(rState)
	if err != nil {
		handleOauthNotValid(w)
		return
	}

	azureInfo, err := getAzureUserInfoWithToken(rState, rCode)
	if err != nil {
		logger.Log(1, "error when getting user info from azure:", err.Error())
		if strings.Contains(err.Error(), "invalid oauth state") || strings.Contains(err.Error(), "failed to fetch user email from SSO state") {
			handleOauthNotValid(w)
			return
		}
		handleOauthNotConfigured(w)
		return
	}
	content := azureInfo.OAuthUser

	var inviteExists bool
	// check if invite exists for User
	in, err := logic.GetUserInvite(content.Email)
	if err == nil {
		inviteExists = true
	}
	// check if user approval is already pending
	if !inviteExists && logic.IsPendingUser(content.Email) {
		handleOauthUserSignUpApprovalPending(w)
		return
	}

	user, err := logic.GetUser(content.UserPrincipalName)
	if err == nil {
		// if user exists, then ensure user's auth type is
		// oauth before proceeding.
		if user.AuthType == models.BasicAuth {
			logger.Log(0, "invalid auth type: basic_auth")
			handleAuthTypeMismatch(w)
			return
		}

		// if user exists with provider ID, convert them into email ID
		_, err := logic.GetUser(content.Email)
		if err != nil {
			user.UserName = content.Email
			user.ExternalIdentityProviderID = content.UserPrincipalName
			database.DeleteRecord(database.USERS_TABLE_NAME, content.UserPrincipalName)
			d, _ := json.Marshal(user)
			database.Insert(user.UserName, string(d), database.USERS_TABLE_NAME)
		}
	}

	user, err = logic.GetUser(content.Email)
	if err != nil {
		if database.IsEmptyRecord(err) { // user must not exist, so try to make one
			if inviteExists {
				// create user
				user, err := proLogic.PrepareOauthUserFromInvite(in)
				if err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
					return
				}
				user.ExternalIdentityProviderID = string(content.ID)
				if err = logic.CreateUser(&user); err != nil {
					handleSomethingWentWrong(w)
					return
				}
				logic.DeleteUserInvite(content.Email)
				logic.DeletePendingUser(content.Email)
			} else {
				if !isEmailAllowed(content.Email) {
					handleOauthUserNotAllowedToSignUp(w)
					return
				}
				err = logic.InsertPendingUser(&models.User{
					UserName:                   content.Email,
					ExternalIdentityProviderID: string(content.ID),
					AuthType:                   models.OAuth,
				})
				if err != nil {
					handleSomethingWentWrong(w)
					return
				}
				handleFirstTimeOauthUserSignUp(w)
				return
			}
		} else {
			handleSomethingWentWrong(w)
			return
		}
	} else {
		// if user exists, then ensure user's auth type is
		// oauth before proceeding.
		if user.AuthType == models.BasicAuth {
			logger.Log(0, "invalid auth type: basic_auth")
			handleAuthTypeMismatch(w)
			return
		}
	}

	user, err = logic.GetUser(content.Email)
	if err != nil {
		handleOauthUserNotFound(w)
		return
	}

	// Check device claims if user has ExternalIdentityProviderID set (synced from IDP)
	// Validate device authorization - allow if device is registered/compliant, block if not
	if user.ExternalIdentityProviderID != "" {
		if err := checkDeviceClaims(azureInfo.RawIDToken); err != nil {
			logger.Log(1, "Device authorization check failed for user with ExternalIdentityProviderID:", err.Error())
			handleDeviceClaimsMissing(w)
			return
		}
	}

	if user.AccountDisabled {
		handleUserAccountDisabled(w)
		return
	}

	userRole, err := logic.GetRole(user.PlatformRoleID)
	if err != nil {
		handleSomethingWentWrong(w)
		return
	}
	if userRole.DenyDashboardAccess {
		handleOauthUserNotAllowed(w)
		return
	}
	var newPass, fetchErr = logic.FetchPassValue("")
	if fetchErr != nil {
		return
	}
	// send a netmaker jwt token
	var authRequest = models.UserAuthParams{
		UserName: content.Email,
		Password: newPass,
	}

	var jwt, jwtErr = logic.VerifyAuthRequest(authRequest, state.AppName)
	if jwtErr != nil {
		logger.Log(1, "could not parse jwt for user", authRequest.UserName)
		return
	}
	logic.LogEvent(&models.Event{
		Action: models.Login,
		Source: models.Subject{
			ID:   user.UserName,
			Name: user.UserName,
			Type: models.UserSub,
		},
		TriggeredBy: user.UserName,
		Target: models.Subject{
			ID:   models.DashboardSub.String(),
			Name: models.DashboardSub.String(),
			Type: models.DashboardSub,
			Info: user,
		},
		Origin: models.Dashboard,
	})
	logger.Log(1, "completed azure OAuth sigin in for", content.Email)
	http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?login="+jwt+"&user="+content.Email, http.StatusPermanentRedirect)
}

// AzureUserInfo extends OAuthUser with ID token for device claims verification
type AzureUserInfo struct {
	*OAuthUser
	RawIDToken string
}

func getAzureUserInfo(state string, code string) (*OAuthUser, error) {
	azureInfo, err := getAzureUserInfoWithToken(state, code)
	if err != nil {
		return nil, err
	}
	return azureInfo.OAuthUser, nil
}

func getAzureUserInfoWithToken(state string, code string) (*AzureUserInfo, error) {
	oauth_state_string, isValid := logic.IsStateValid(state)
	if (!isValid || state != oauth_state_string) && !isStateCached(state) {
		return nil, fmt.Errorf("invalid oauth state")
	}

	ctx, cancel := context.WithTimeout(context.Background(), AzureAD_TIMEOUT)
	defer cancel()

	var token, err = auth_provider.Exchange(ctx, code, oauth2.SetAuthURLParam("prompt", "login"))
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}

	// Extract raw ID token for later device claims verification
	rawIDToken, _ := token.Extra("id_token").(string)

	var data []byte
	data, err = json.Marshal(token)
	if err != nil {
		return nil, fmt.Errorf("failed to convert token to json: %s", err.Error())
	}
	var httpReq, reqErr = http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me", nil)
	if reqErr != nil {
		return nil, fmt.Errorf("failed to create request to microsoft")
	}

	httpReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	response, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body: %s", err.Error())
	}
	var userInfo = &OAuthUser{}
	if err = json.Unmarshal(contents, userInfo); err != nil {
		return nil, fmt.Errorf("failed parsing email from response data: %s", err.Error())
	}
	userInfo.AccessToken = string(data)
	if userInfo.Email == "" {
		userInfo.Email = getUserEmailFromClaims(token.AccessToken)
	}
	if userInfo.Email == "" && userInfo.UserPrincipalName != "" {
		userInfo.Email = userInfo.UserPrincipalName
	}
	if userInfo.Email == "" {
		err = errors.New("failed to fetch user email from SSO state")
		return &AzureUserInfo{OAuthUser: userInfo, RawIDToken: rawIDToken}, err
	}
	return &AzureUserInfo{OAuthUser: userInfo, RawIDToken: rawIDToken}, nil
}

// checkDeviceClaims validates device authorization from device claims in the ID token
// Returns an error if device claims are present but device is NOT authorized (to block authentication)
// Returns nil if device claims are NOT present OR device IS authorized (to allow authentication)
func checkDeviceClaims(rawIDToken string) error {
	if azure_ad_verifier == nil {
		// If verifier not available, allow authentication
		return nil
	}

	if rawIDToken == "" {
		// If ID token not available, allow authentication
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), AzureAD_TIMEOUT)
	defer cancel()

	idToken, err := azure_ad_verifier.Verify(ctx, rawIDToken)
	if err != nil {
		// If token verification fails, allow authentication
		logger.Log(1, "Failed to verify Azure AD ID token for device claims check:", err.Error())
		return nil
	}

	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		// If claims extraction fails, allow authentication
		logger.Log(1, "Failed to extract claims from ID token:", err.Error())
		return nil
	}

	// Check for device claims in the ID token
	hasDeviceClaims := false
	deviceID := ""
	deviceRegStatus := ""
	fmt.Printf("==> DEVICE CLAIMS: %+v\n", claims)
	// Extract device information from claims
	if val, exists := claims["deviceid"]; exists && val != nil {
		deviceID = fmt.Sprintf("%v", val)
		hasDeviceClaims = true
		logger.Log(3, fmt.Sprintf("Found deviceid claim: %s", deviceID))
	}

	if val, exists := claims["deviceregstatus"]; exists && val != nil {
		deviceRegStatus = fmt.Sprintf("%v", val)
		logger.Log(3, fmt.Sprintf("Found deviceregstatus claim: %s", deviceRegStatus))
	}

	// If device claims ARE present, validate device authorization
	if hasDeviceClaims {
		// Check device registration status - allow if device is registered/compliant
		// Azure AD sets deviceregstatus to values like "Registered", "Compliant", etc.
		// If status indicates the device is registered/compliant, allow authentication
		if deviceRegStatus != "" {
			// Normalize status to lowercase for comparison
			statusLower := strings.ToLower(deviceRegStatus)
			// Allow if device is registered, compliant, or managed (authorized devices)
			if strings.Contains(statusLower, "registered") ||
				strings.Contains(statusLower, "compliant") ||
				strings.Contains(statusLower, "managed") {
				logger.Log(3, fmt.Sprintf("Device authorization check: Device %s has status '%s' - device is authorized, allowing authentication", deviceID, deviceRegStatus))
				return nil
			} else {
				// Device claims present but device is NOT registered/compliant - block authentication
				logger.Log(1, fmt.Sprintf("Device authorization check: Device %s has status '%s' - device is not authorized, blocking authentication", deviceID, deviceRegStatus))
				return fmt.Errorf("device claims found but device is not registered/compliant - authentication not allowed")
			}
		} else if compliant, ok := claims["xms_compliant"].(string); ok {
			logger.Log(3, "Azure Device: id=%s compliant=%s", deviceID, compliant)
			if compliant != "true" {
				return errors.New("access denied: device not compliant")
			}
		} else if deviceID != "" {
			// If device ID exists but no status, allow authentication (device is registered)
			logger.Log(3, fmt.Sprintf("Device authorization check: Device %s found but no registration status - allowing authentication", deviceID))
			return nil
		}
	}

	// If device claims are NOT present, allow authentication
	return nil
}

func verifyAzureUser(token *oauth2.Token) bool {
	return token.Valid()
}
