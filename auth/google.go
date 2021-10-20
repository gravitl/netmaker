package auth

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var google_functions = map[string]interface{}{
	init_provider:   initGoogle,
	get_user_info:   getUserInfo,
	handle_callback: handleGoogleCallback,
	handle_login:    handleGoogleLogin,
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
	url := auth_provider.AuthCodeURL(oauth_state_string)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {

	var content, err = getUserInfo(r.FormValue("state"), r.FormValue("code"))
	if err != nil {
		fmt.Println(err.Error())
		http.Redirect(w, r, "/api/oauth/error", http.StatusTemporaryRedirect)
		return
	}
	fmt.Fprintf(w, "Content: %s\n", content)
}

func getUserInfo(state string, code string) ([]byte, error) {
	if state != oauth_state_string {
		return nil, fmt.Errorf("invalid oauth state")
	}
	token, err := auth_provider.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
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
	return contents, nil
}
