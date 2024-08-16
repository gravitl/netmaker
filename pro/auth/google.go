package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var google_functions = map[string]interface{}{
	init_provider:   initGoogle,
	get_user_info:   getGoogleUserInfo,
	handle_callback: handleGoogleCallback,
	handle_login:    handleGoogleLogin,
	verify_user:     verifyGoogleUser,
}

// == handle google authentication here ==

func initGoogle(redirectURL string, clientID string, clientSecret string) {
	auth_provider = &oauth2.Config{
		RedirectURL:  redirectURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
}

func handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	var oauth_state_string = logic.RandomString(user_signin_length)
	if auth_provider == nil {
		handleOauthNotConfigured(w)
		return
	}
	logger.Log(0, "Setting OAuth State ", oauth_state_string)
	if err := logic.SetState(oauth_state_string); err != nil {
		handleOauthNotConfigured(w)
		return
	}

	var url = auth_provider.AuthCodeURL(oauth_state_string)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {

	var rState, rCode = getStateAndCode(r)
	logger.Log(0, "Fetched OAuth State ", rState)
	var content, err = getGoogleUserInfo(rState, rCode)
	if err != nil {
		logger.Log(1, "error when getting user info from google:", err.Error())
		if strings.Contains(err.Error(), "invalid oauth state") {
			handleOauthNotValid(w)
			return
		}
		handleOauthNotConfigured(w)
		return
	}
	logger.Log(0, "CALLBACK ----> 1")

	logger.Log(0, "CALLBACK ----> 2")
	var inviteExists bool
	// check if invite exists for User
	in, err := logic.GetUserInvite(content.Email)
	if err == nil {
		inviteExists = true
	}
	logger.Log(0, fmt.Sprintf("CALLBACK ----> 3  %v", inviteExists))
	// check if user approval is already pending
	if !inviteExists && logic.IsPendingUser(content.Email) {
		handleOauthUserSignUpApprovalPending(w)
		return
	}
	logger.Log(0, "CALLBACK ----> 4")
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
				logger.Log(0, "CALLBACK ----> 4.0")

				if err = logic.CreateUser(&user); err != nil {
					handleSomethingWentWrong(w)
					return
				}
				logic.DeleteUserInvite(user.UserName)
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
	logger.Log(0, "CALLBACK ----> 6")
	user, err := logic.GetUser(content.Email)
	if err != nil {
		logger.Log(0, "error fetching user: ", err.Error())
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

	logger.Log(1, "completed google OAuth sigin in for", content.Email)
	http.Redirect(w, r, fmt.Sprintf("%s/login?login=%s&user=%s", servercfg.GetFrontendURL(), jwt, content.Email), http.StatusPermanentRedirect)
}

func getGoogleUserInfo(state string, code string) (*OAuthUser, error) {
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
	client := &http.Client{
		Timeout: time.Second * 30,
	}
	response, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body: %s", err.Error())
	}
	logger.Log(0, fmt.Sprintf("---------------> USERINFO: %v, token: %s", string(contents), token.AccessToken))
	var userInfo = &OAuthUser{}
	if err = json.Unmarshal(contents, userInfo); err != nil {
		return nil, fmt.Errorf("failed parsing email from response data: %s", err.Error())
	}
	userInfo.AccessToken = string(data)
	return userInfo, nil
}

func verifyGoogleUser(token *oauth2.Token) bool {
	return token.Valid()
}
