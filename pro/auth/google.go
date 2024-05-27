package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gravitl/netmaker/auth"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
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
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/groups"},
		Endpoint:     google.Endpoint,
	}
}

func handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
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

func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {

	var rState, rCode = getStateAndCode(r)

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
	if !isEmailAllowed(content.Email) {
		handleOauthUserNotAllowedToSignUp(w)
		return
	}
	// check if user approval is already pending
	if logic.IsPendingUser(content.Email) {
		handleOauthUserSignUpApprovalPending(w)
		return
	}
	_, err = logic.GetUser(content.Email)
	if err != nil {
		if database.IsEmptyRecord(err) { // user must not exist, so try to make one
			err = logic.InsertPendingUser(&models.User{
				UserName: content.Email,
			})
			if err != nil {
				handleSomethingWentWrong(w)
				return
			}
			handleFirstTimeOauthUserSignUp(w)
			return
		} else {
			handleSomethingWentWrong(w)
			return
		}
	}
	user, err := logic.GetUser(content.Email)
	if err != nil {
		logger.Log(0, "error fetching user: ", err.Error())
		handleOauthUserNotFound(w)
		return
	}
	if !(user.IsSuperAdmin || user.IsAdmin) {
		handleOauthUserNotAllowed(w)
		return
	}
	var newPass, fetchErr = auth.FetchPassValue("")
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
