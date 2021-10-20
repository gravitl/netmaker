package auth

import (
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
	oauth_state_string     = "netmaker-oauth-state"
	google_provider_name   = "google"
	azure_ad_provider_name = "azure-ad"
	github_provider_name   = "github"
)

var auth_provider *oauth2.Config

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
func InitializeAuthProvider() bool {
	var functions = getCurrentAuthFunctions()
	if functions == nil {
		return false
	}
	var authInfo = servercfg.GetAuthProviderInfo()
	functions[init_provider].(func(string, string, string))(servercfg.GetAPIConnString(), authInfo[1], authInfo[2])
	return auth_provider != nil
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
