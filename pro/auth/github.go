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
		Scopes:       []string{},
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
		if strings.Contains(err.Error(), "invalid oauth state") {
			handleOauthNotValid(w)
			return
		}
		handleOauthNotConfigured(w)
		return
	}
	if !isEmailAllowed(content.Login) {
		handleOauthUserNotAllowedToSignUp(w)
		return
	}
	var inviteExists bool
	// check if invite exists for User
	in, err := logic.GetUserInvite(content.Login)
	if err == nil {
		inviteExists = true
	}
	// check if user approval is already pending
	if !inviteExists && logic.IsPendingUser(content.Login) {
		handleOauthUserSignUpApprovalPending(w)
		return
	}
	_, err = logic.GetUser(content.Login)
	if err != nil {
		if database.IsEmptyRecord(err) { // user must not exist, so try to make one
			if inviteExists {
				// create user
				var newPass, fetchErr = logic.FetchPassValue("")
				if fetchErr != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(fetchErr, "internal"))
					return
				}
				user := &models.User{
					UserName: content.Login,
					Password: newPass,
				}

				for _, inviteGroupID := range in.Groups {
					userG, err := proLogic.GetUserGroup(inviteGroupID)
					if err != nil {
						logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("error fetching group id "+inviteGroupID.String()), "badrequest"))
						return
					}
					user.PlatformRoleID = userG.PlatformRole
					user.UserGroups[inviteGroupID] = struct{}{}
				}
				if user.PlatformRoleID == "" {
					user.PlatformRoleID = models.ServiceUser
				}
				if err = logic.CreateUser(user); err != nil {
					handleSomethingWentWrong(w)
					return
				}
				logic.DeleteUserInvite(user.UserName)
				logic.DeletePendingUser(content.Login)
			} else {
				err = logic.InsertPendingUser(&models.User{
					UserName: content.Login,
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
	user, err := logic.GetUser(content.Login)
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
		UserName: content.Login,
		Password: newPass,
	}

	var jwt, jwtErr = logic.VerifyAuthRequest(authRequest)
	if jwtErr != nil {
		logger.Log(1, "could not parse jwt for user", authRequest.UserName)
		return
	}

	logger.Log(1, "completed github OAuth sigin in for", content.Login)
	http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?login="+jwt+"&user="+content.Login, http.StatusPermanentRedirect)
}

func getGithubUserInfo(state string, code string) (*OAuthUser, error) {
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
	return userInfo, nil
}

func verifyGithubUser(token *oauth2.Token) bool {
	return token.Valid()
}
