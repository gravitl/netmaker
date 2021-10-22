package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var google_functions = map[string]interface{}{
	init_provider:   initGoogle,
	get_user_info:   getUserInfo,
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
	oauth_state_string = logic.RandomString(16)
	var url = auth_provider.AuthCodeURL(oauth_state_string)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {

	var content, err = getUserInfo(r.FormValue("state"), r.FormValue("code"))
	if err != nil {
		fmt.Println(err.Error())
		http.Redirect(w, r, servercfg.GetFrontendURL()+"?oauth=callback-error", http.StatusTemporaryRedirect)
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
		logic.Log("could not parse jwt for user "+authRequest.UserName, 1)
		return
	}

	logic.Log("completed google oauth sigin in for "+content.Email, 0)
	http.Redirect(w, r, servercfg.GetFrontendURL()+"?login="+jwt+"&email="+content.Email, http.StatusPermanentRedirect)
}

func getUserInfo(state string, code string) (*OauthUser, error) {
	if state != oauth_state_string {
		return nil, fmt.Errorf("invalid oauth state")
	}
	var token, err = auth_provider.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	var data []byte
	data, err = json.Marshal(token)
	if err != nil {
		return nil, fmt.Errorf("failed to convert token to json: %s", err.Error())
	}
	response, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body: %s", err.Error())
	}
	var userInfo = &OauthUser{}
	if err = json.Unmarshal(contents, userInfo); err != nil {
		return nil, fmt.Errorf("failed parsing email from response data: %s", err.Error())
	}
	userInfo.AccessToken = string(data)
	return userInfo, nil
}

func verifyGoogleUser(token *oauth2.Token) bool {
	if token.Valid() {
		var err error
		_, err = http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
		return err == nil
	}
	return false
}
