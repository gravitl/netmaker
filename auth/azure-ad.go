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
	"golang.org/x/oauth2/microsoft"
)

var azure_ad_functions = map[string]interface{}{
	init_provider:   initAzureAD,
	get_user_info:   getAzureUserInfo,
	handle_callback: handleAzureCallback,
	handle_login:    handleAzureLogin,
	verify_user:     verifyAzureUser,
}

type azureOauthUser struct {
	UserPrincipalName string `json:"userPrincipalName" bson:"userPrincipalName"`
	AccessToken       string `json:"accesstoken" bson:"accesstoken"`
}

// == handle azure ad authentication here ==

func initAzureAD(redirectURL string, clientID string, clientSecret string) {
	auth_provider = &oauth2.Config{
		RedirectURL:  redirectURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"User.Read"},
		Endpoint:     microsoft.AzureADEndpoint(servercfg.GetAzureTenant()),
	}
}

func handleAzureLogin(w http.ResponseWriter, r *http.Request) {
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

func handleAzureCallback(w http.ResponseWriter, r *http.Request) {

	var content, err = getAzureUserInfo(r.FormValue("state"), r.FormValue("code"))
	if err != nil {
		logger.Log(1, "error when getting user info from azure:", err.Error())
		http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?oauth=callback-error", http.StatusTemporaryRedirect)
		return
	}
	_, err = logic.GetUser(content.UserPrincipalName)
	if err != nil { // user must not exists, so try to make one
		if err = addUser(content.UserPrincipalName); err != nil {
			return
		}
	}
	var newPass, fetchErr = fetchPassValue("")
	if fetchErr != nil {
		return
	}
	// send a netmaker jwt token
	var authRequest = models.UserAuthParams{
		UserName: content.UserPrincipalName,
		Password: newPass,
	}

	var jwt, jwtErr = logic.VerifyAuthRequest(authRequest)
	if jwtErr != nil {
		logger.Log(1, "could not parse jwt for user", authRequest.UserName)
		return
	}

	logger.Log(1, "completed azure OAuth sigin in for", content.UserPrincipalName)
	http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?login="+jwt+"&user="+content.UserPrincipalName, http.StatusPermanentRedirect)
}

func getAzureUserInfo(state string, code string) (*azureOauthUser, error) {
	oauth_state_string, isValid := logic.IsStateValid(state)
	if !isValid || state != oauth_state_string {
		return nil, fmt.Errorf("invalid oauth state")
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
	var httpReq, reqErr = http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me", nil)
	if reqErr != nil {
		return nil, fmt.Errorf("failed to create request to GitHub")
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
	var userInfo = &azureOauthUser{}
	if err = json.Unmarshal(contents, userInfo); err != nil {
		return nil, fmt.Errorf("failed parsing email from response data: %s", err.Error())
	}
	userInfo.AccessToken = string(data)
	return userInfo, nil
}

func verifyAzureUser(token *oauth2.Token) bool {
	return token.Valid()
}
