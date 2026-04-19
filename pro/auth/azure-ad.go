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

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
	"gorm.io/gorm"
)

var azure_ad_functions = map[string]interface{}{
	init_provider:   initAzureAD,
	get_user_info:   getAzureUserInfo,
	handle_callback: handleAzureCallback,
	handle_login:    handleAzureLogin,
	verify_user:     verifyAzureUser,
}

// == handle azure ad authentication here ==

func initAzureAD(redirectURL string, clientID string, clientSecret string) {
	auth_provider = &oauth2.Config{
		RedirectURL:  redirectURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"User.Read", "email", "profile", "openid"},
		Endpoint:     microsoft.AzureADEndpoint(logic.GetAzureTenant()),
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

	content, err := getAzureUserInfo(rState, rCode)
	if err != nil {
		logger.Log(1, "error when getting user info from azure:", err.Error())
		if strings.Contains(err.Error(), "invalid oauth state") || strings.Contains(err.Error(), "failed to fetch user email from SSO state") {
			handleOauthNotValid(w)
			return
		}
		handleOauthNotConfigured(w)
		return
	}
	var inviteExists bool
	// check if invite exists for User
	in, err := logic.GetUserInvite(content.Email)
	if err == nil {
		inviteExists = true
	}
	// check if user approval is already pending
	if !inviteExists && (logic.IsPendingUser(content.Email) || logic.IsPendingUser(content.UserPrincipalName)) {
		handleOauthUserSignUpApprovalPending(w)
		return
	}

	user, err := GetMatchingUser(content)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { // user must not exist, so try to make one
			if inviteExists {
				// create user
				user, err := proLogic.PrepareOauthUserFromInvite(in)
				if err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
					return
				}
				user.Username = content.UserPrincipalName
				user.ExternalIdentityProviderID = string(content.ID)
				if err = logic.CreateUser(&user); err != nil {
					handleSomethingWentWrong(w)
					return
				}
				logic.DeleteUserInvite(content.Email)
				logic.DeletePendingUser(content.UserPrincipalName)
				logic.DeletePendingUser(content.Email)
			} else {
				if !isEmailAllowed(content.Email) {
					handleOauthUserNotAllowedToSignUp(w)
					return
				}
				err = logic.InsertPendingUser(&models.User{
					UserName:                   content.UserPrincipalName,
					ExternalIdentityProviderID: string(content.ID),
					AuthType:                   schema.OAuth,
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
		if user.AuthType == schema.BasicAuth {
			logger.Log(0, "invalid auth type: basic_auth")
			handleAuthTypeMismatch(w)
			return
		}
	}

	user, err = GetMatchingUser(content)
	if err != nil {
		handleOauthUserNotFound(w)
		return
	}

	if user.AccountDisabled {
		handleUserAccountDisabled(w)
		return
	}

	userRole := &schema.UserRole{ID: user.PlatformRoleID}
	err = userRole.Get(r.Context())
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
		UserName: content.UserPrincipalName,
		Password: newPass,
	}

	var jwt, jwtErr = logic.VerifyAuthRequest(authRequest, state.AppName)
	if jwtErr != nil {
		logger.Log(1, "could not parse jwt for user", authRequest.UserName)
		return
	}
	logic.LogEvent(&models.Event{
		Action: schema.Login,
		Source: models.Subject{
			ID:   user.Username,
			Name: user.Username,
			Type: schema.UserSub,
		},
		TriggeredBy: user.Username,
		Target: models.Subject{
			ID:   schema.DashboardSub.String(),
			Name: schema.DashboardSub.String(),
			Type: schema.DashboardSub,
			Info: logic.ToReturnUser(user),
		},
		Origin: schema.Dashboard,
	})
	logger.Log(1, "completed azure OAuth sigin in for", user.Username)
	http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?login="+jwt+"&user="+user.Username, http.StatusPermanentRedirect)
}

func GetMatchingUser(oauthUser *OAuthUser) (*schema.User, error) {
	user := &schema.User{
		Username: oauthUser.UserPrincipalName,
	}
	err := user.Get(db.WithContext(context.TODO()))
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else {
		return user, nil
	}

	user = &schema.User{
		ExternalIdentityProviderID: string(oauthUser.ID),
	}
	err = user.GetByExternalID(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	return user, nil
}

func getAzureUserInfo(state string, code string) (*OAuthUser, error) {
	oauth_state_string, isValid := logic.IsStateValid(state)
	if (!isValid || state != oauth_state_string) && !isStateCached(state) {
		return nil, fmt.Errorf("invalid oauth state")
	}
	var token, err = auth_provider.Exchange(context.Background(), code, oauth2.SetAuthURLParam("prompt", "login"))
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
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
	graphClient := &http.Client{Timeout: 30 * time.Second}
	response, err := graphClient.Do(httpReq)
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
		return userInfo, err
	}
	return userInfo, nil
}

func verifyAzureUser(token *oauth2.Token) bool {
	return token.Valid()
}
