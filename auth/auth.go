package auth

import (
	"encoding/json"
	"net/http"

	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/oauth2"
)

// == consts ==
const (
	init_provider          = "initprovider"
	get_user_info          = "getuserinfo"
	handle_callback        = "handlecallback"
	handle_login           = "handlelogin"
	google_provider_name   = "google"
	azure_ad_provider_name = "azure-ad"
	github_provider_name   = "github"
	verify_user            = "verifyuser"
)

var oauth_state_string = "netmaker-oauth-state" // should be set randomly each provider login
var auth_provider *oauth2.Config

type OauthUser struct {
	Email       string `json:"email" bson:"email"`
	AccessToken string `json:"accesstoken" bson:"accesstoken"`
}

func getCurrentAuthFunctions() map[string]interface{} {
	var authInfo = servercfg.GetAuthProviderInfo()
	var authProvider = authInfo[0]
	switch authProvider {
	case google_provider_name:
		return google_functions
	case azure_ad_provider_name:
		return google_functions
	case github_provider_name:
		return google_functions
	default:
		return nil
	}
}

// InitializeAuthProvider - initializes the auth provider if any is present
func InitializeAuthProvider() string {
	var functions = getCurrentAuthFunctions()
	if functions == nil {
		return ""
	}
	var authInfo = servercfg.GetAuthProviderInfo()
	functions[init_provider].(func(string, string, string))(servercfg.GetAPIConnString()+"/api/oauth/callback", authInfo[1], authInfo[2])
	return authInfo[0]
}

// HandleAuthCallback - handles oauth callback
func HandleAuthCallback(w http.ResponseWriter, r *http.Request) {
	var functions = getCurrentAuthFunctions()
	if functions == nil {
		return
	}
	functions[handle_callback].(func(http.ResponseWriter, *http.Request))(w, r)
}

// HandleAuthLogin - handles oauth login
func HandleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var functions = getCurrentAuthFunctions()
	if functions == nil {
		return
	}
	functions[handle_login].(func(http.ResponseWriter, *http.Request))(w, r)
}

// VerifyUserToken - checks if oauth2 token is valid
func VerifyUserToken(accessToken string) bool {
	var token = &oauth2.Token{}
	var err = json.Unmarshal([]byte(accessToken), token)
	if err != nil || !token.Valid() {
		return false
	}
	var functions = getCurrentAuthFunctions()
	if functions == nil {
		return false
	}
	return functions[verify_user].(func(*oauth2.Token) bool)(token)
}
