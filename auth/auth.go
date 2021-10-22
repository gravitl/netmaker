package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
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
	auth_key               = "netmaker_auth"
)

var oauth_state_string = "netmaker-oauth-state" // should be set randomly each provider login
var auth_provider *oauth2.Config

func getCurrentAuthFunctions() map[string]interface{} {
	var authInfo = servercfg.GetAuthProviderInfo()
	var authProvider = authInfo[0]
	switch authProvider {
	case google_provider_name:
		return google_functions
	case azure_ad_provider_name:
		return azure_ad_functions
	case github_provider_name:
		return github_functions
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
	var _, err = fetchPassValue(logic.RandomString(64))
	if err != nil {
		logic.Log(err.Error(), 0)
		return ""
	}
	var currentFrontendURL = servercfg.GetFrontendURL()
	if currentFrontendURL == "" {
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

// IsOauthUser - returns
func IsOauthUser(user *models.User) error {
	var currentValue, err = fetchPassValue("")
	if err != nil {
		return err
	}
	var bCryptErr = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentValue))
	return bCryptErr
}

// == private methods ==

func addUser(email string) error {
	var hasAdmin, err = logic.HasAdmin()
	if err != nil {
		logic.Log("error checking for existence of admin user during OAuth login for "+email+", user not added", 1)
		return err
	} // generate random password to adapt to current model
	var newPass, fetchErr = fetchPassValue("")
	if fetchErr != nil {
		return fetchErr
	}
	var newUser = models.User{
		UserName: email,
		Password: newPass,
	}
	if !hasAdmin { // must be first attempt, create an admin
		if newUser, err = logic.CreateAdmin(newUser); err != nil {
			logic.Log("error creating admin from user, "+email+", user not added", 1)
		} else {
			logic.Log("admin created from user, "+email+", was first user added", 0)
		}
	} else { // otherwise add to db as admin..?
		// TODO: add ability to add users with preemptive permissions
		newUser.IsAdmin = false
		if newUser, err = logic.CreateUser(newUser); err != nil {
			logic.Log("error creating user, "+email+", user not added", 1)
		} else {
			logic.Log("user created from, "+email+"", 0)
		}
	}
	return nil
}

func fetchPassValue(newValue string) (string, error) {

	type valueHolder struct {
		Value string `json:"value" bson:"value"`
	}
	var b64NewValue = base64.StdEncoding.EncodeToString([]byte(newValue))
	var newValueHolder = &valueHolder{
		Value: b64NewValue,
	}
	var data, marshalErr = json.Marshal(newValueHolder)
	if marshalErr != nil {
		return "", marshalErr
	}

	var currentValue, err = logic.FetchAuthSecret(auth_key, string(data))
	if err != nil {
		return "", err
	}
	var unmarshErr = json.Unmarshal([]byte(currentValue), newValueHolder)
	if unmarshErr != nil {
		return "", unmarshErr
	}

	var b64CurrentValue, b64Err = base64.StdEncoding.DecodeString(newValueHolder.Value)
	if b64Err != nil {
		logic.Log("could not decode pass", 0)
		return "", nil
	}
	return string(b64CurrentValue), nil
}
