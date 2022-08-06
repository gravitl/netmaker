package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

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

type googleOauthUser struct {
	Email       string `json:"email" bson:"email"`
	AccessToken string `json:"accesstoken" bson:"accesstoken"`
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
	var oauth_state_string = logic.RandomString(16)
	if auth_provider == nil && servercfg.GetFrontendURL() != "" {
		http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?oauth=callback-error", http.StatusTemporaryRedirect)
		return
	} else if auth_provider == nil {
		fmt.Fprintf(w, "%s", []byte("no frontend URL was provided and an OAuth login was attempted\nplease reconfigure server to use OAuth or use basic credentials"))
		return
	}

	if err := logic.SetState(oauth_state_string); err != nil {
		http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?oauth=callback-error", http.StatusTemporaryRedirect)
		return
	}

	var url = auth_provider.AuthCodeURL(oauth_state_string)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {

	var content, err = getGoogleUserInfo(r.FormValue("state"), r.FormValue("code"))
	if err != nil {
		logger.Log(1, "error when getting user info from google:", err.Error())
		http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?oauth=callback-error", http.StatusTemporaryRedirect)
		return
	}
	_, err = logic.GetUser(content.Email)
	if err != nil { // user must not exists, so try to make one
		if err = addUser(content.Email); err != nil {
			return
		}
	}
	var newPass, fetchErr = fetchPassValue("")
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
	http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?login="+jwt+"&user="+content.Email, http.StatusPermanentRedirect)
}

func getGoogleUserInfo(state string, code string) (*googleOauthUser, error) {
	oauth_state_string, isValid := logic.IsStateValid(state)
	if !isValid || state != oauth_state_string {
		return nil, fmt.Errorf("invalid OAuth state")
	}
	var token, err = auth_provider.Exchange(context.Background(), code)
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
	var userInfo = &googleOauthUser{}
	if err = json.Unmarshal(contents, userInfo); err != nil {
		return nil, fmt.Errorf("failed parsing email from response data: %s", err.Error())
	}
	userInfo.AccessToken = string(data)
	return userInfo, nil
}

func verifyGoogleUser(token *oauth2.Token) bool {
	return token.Valid()
}
