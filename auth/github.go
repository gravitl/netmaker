package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
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

type githubOauthUser struct {
	Login       string `json:"login" bson:"login"`
	AccessToken string `json:"accesstoken" bson:"accesstoken"`
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

func handleGithubCallback(w http.ResponseWriter, r *http.Request) {

	var content, err = getGithubUserInfo(r.URL.Query().Get("state"), r.URL.Query().Get("code"))
	if err != nil {
		logger.Log(1, "error when getting user info from github:", err.Error())
		http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?oauth=callback-error", http.StatusTemporaryRedirect)
		return
	}
	_, err = logic.GetUser(content.Login)
	if err != nil { // user must not exist, so try to make one
		if err = addUser(content.Login); err != nil {
			return
		}
	}
	var newPass, fetchErr = fetchPassValue("")
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

func getGithubUserInfo(state string, code string) (*githubOauthUser, error) {
	oauth_state_string, isValid := logic.IsStateValid(state)
	if !isValid || state != oauth_state_string {
		return nil, fmt.Errorf("invalid OAuth state")
	}
	var token, err = auth_provider.Exchange(context.Background(), code)
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
	var userInfo = &githubOauthUser{}
	if err = json.Unmarshal(contents, userInfo); err != nil {
		return nil, fmt.Errorf("failed parsing email from response data: %s", err.Error())
	}
	userInfo.AccessToken = string(data)
	return userInfo, nil
}

func verifyGithubUser(token *oauth2.Token) bool {
	return token.Valid()
}
