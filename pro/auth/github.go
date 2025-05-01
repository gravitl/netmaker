package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

var github_functions = map[string]interface{}{
	init_provider:   initGithub,
	get_user_info:   getGithubUserInfo,
	handle_callback: handleGithubCallback,
	handle_login:    handleGithubLogin,
	verify_user:     verifyGithubUser,
}

// == handle github authentication here ==

func initGithub(redirectURL string, clientID string, clientSecret string) {
	auth_provider = &oauth2.Config{
		RedirectURL:  redirectURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"read:user", "user:email"},
		Endpoint:     github.Endpoint,
	}
}

func handleGithubLogin(w http.ResponseWriter, r *http.Request) {
	var oauth_state_string = logic.RandomString(user_signin_length)
	if auth_provider == nil {
		handleOauthNotConfigured(w)
		return
	}

	if err := logic.SetState(oauth_state_string); err != nil {
		handleOauthNotConfigured(w)
		return
	}

	var url = auth_provider.AuthCodeURL(oauth_state_string)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleGithubCallback(w http.ResponseWriter, r *http.Request) {

	var rState, rCode = getStateAndCode(r)
	var content, err = getGithubUserInfo(rState, rCode)
	if err != nil {
		logger.Log(1, "error when getting user info from github:", err.Error())
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
	if !inviteExists && logic.IsPendingUser(content.Email) {
		handleOauthUserSignUpApprovalPending(w)
		return
	}
	// if user exists with provider ID, convert them into email ID
	user, err := logic.GetUser(content.Login)
	if err == nil {
		// if user exists, then ensure user's auth type is
		// oauth before proceeding.
		if user.AuthType == models.BasicAuth {
			logger.Log(0, "invalid auth type: basic_auth")
			handleAuthTypeMismatch(w)
			return
		}

		// checks if user exists with email
		_, err := logic.GetUser(content.Email)
		if err != nil {
			user.UserName = content.Email
			user.ExternalIdentityProviderID = content.Login
			database.DeleteRecord(database.USERS_TABLE_NAME, content.Login)
			d, _ := json.Marshal(user)
			database.Insert(user.UserName, string(d), database.USERS_TABLE_NAME)
		}

	}
	_, err = logic.GetUser(content.Email)
	if err != nil {
		if database.IsEmptyRecord(err) { // user must not exist, so try to make one
			if inviteExists {
				// create user
				user, err := proLogic.PrepareOauthUserFromInvite(in)
				if err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
					return
				}
				user.ExternalIdentityProviderID = content.Login
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
					UserName: content.Email,
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
	}
	user, err = logic.GetUser(content.Email)
	if err != nil {
		handleOauthUserNotFound(w)
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

	var jwt, jwtErr = logic.VerifyAuthRequest(authRequest)
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
		Target: models.Subject{
			ID:   models.DashboardSub.String(),
			Name: models.DashboardSub.String(),
			Type: models.DashboardSub,
			Info: user,
		},
		Origin: models.Dashboard,
	})
	logger.Log(1, "completed github OAuth sigin in for", content.Email)
	http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?login="+jwt+"&user="+content.Email, http.StatusPermanentRedirect)
}

func getGithubUserInfo(state, code string) (*OAuthUser, error) {
	oauth_state_string, isValid := logic.IsStateValid(state)
	if (!isValid || state != oauth_state_string) && !isStateCached(state) {
		return nil, fmt.Errorf("invalid oauth state")
	}
	var token, err = auth_provider.Exchange(context.Background(), code, oauth2.SetAuthURLParam("prompt", "login"))
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	if !token.Valid() {
		return nil, fmt.Errorf("GitHub code exchange yielded invalid token")
	}
	var data []byte
	data, err = json.Marshal(token)
	if err != nil {
		return nil, fmt.Errorf("failed to convert token to json: %s", err.Error())
	}
	var httpClient = &http.Client{}
	var httpReq, reqErr = http.NewRequest("GET", "https://api.github.com/user", nil)
	if reqErr != nil {
		return nil, fmt.Errorf("failed to create request to GitHub")
	}
	httpReq.Header.Set("Authorization", "token "+token.AccessToken)
	response, err := httpClient.Do(httpReq)
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
		// if user's email is not made public, get the info from the github emails api
		logger.Log(2, "fetching user email from github api")
		userInfo.Email, err = getGithubEmailsInfo(token.AccessToken)
		if err != nil {
			logger.Log(0, "failed to fetch user's email from github: ", err.Error())
		}
	}
	if userInfo.Email == "" {
		err = errors.New("failed to fetch user email from SSO state")
		return userInfo, err
	}
	return userInfo, nil
}

func verifyGithubUser(token *oauth2.Token) bool {
	return token.Valid()
}

func getGithubEmailsInfo(accessToken string) (string, error) {

	var httpClient = &http.Client{}
	var httpReq, reqErr = http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if reqErr != nil {
		return "", fmt.Errorf("failed to create request to GitHub")
	}
	httpReq.Header.Add("Accept", "application/vnd.github.v3+json")
	httpReq.Header.Set("Authorization", "token "+accessToken)
	response, err := httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed getting user info: %s", err.Error())
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("failed reading response body: %s", err.Error())
	}

	emailsInfo := []interface{}{}
	err = json.Unmarshal(contents, &emailsInfo)
	if err != nil {
		return "", err
	}
	for _, info := range emailsInfo {
		emailInfoMap := info.(map[string]interface{})
		if emailInfoMap["primary"].(bool) {
			return emailInfoMap["email"].(string), nil
		}

	}
	return "", errors.New("email not found")
}
