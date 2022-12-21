package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/pro/netcache"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
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
	oidc_provider_name     = "oidc"
	verify_user            = "verifyuser"
	auth_key               = "netmaker_auth"
	user_signin_length     = 16
	node_signin_length     = 64
)

// OAuthUser - generic OAuth strategy user
type OAuthUser struct {
	Name              string `json:"name" bson:"name"`
	Email             string `json:"email" bson:"email"`
	Login             string `json:"login" bson:"login"`
	UserPrincipalName string `json:"userPrincipalName" bson:"userPrincipalName"`
	AccessToken       string `json:"accesstoken" bson:"accesstoken"`
}

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
	case oidc_provider_name:
		return oidc_functions
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
		logger.Log(0, err.Error())
		return ""
	}
	var currentFrontendURL = servercfg.GetFrontendURL()
	if currentFrontendURL == "" {
		return ""
	}
	var authInfo = servercfg.GetAuthProviderInfo()
	var serverConn = servercfg.GetAPIHost()
	if strings.Contains(serverConn, "localhost") || strings.Contains(serverConn, "127.0.0.1") {
		serverConn = "http://" + serverConn
		logger.Log(1, "localhost OAuth detected, proceeding with insecure http redirect: (", serverConn, ")")
	} else {
		serverConn = "https://" + serverConn
		logger.Log(1, "external OAuth detected, proceeding with https redirect: ("+serverConn+")")
	}

	if authInfo[0] == "oidc" {
		functions[init_provider].(func(string, string, string, string))(serverConn+"/api/oauth/callback", authInfo[1], authInfo[2], authInfo[3])
		return authInfo[0]
	}

	functions[init_provider].(func(string, string, string))(serverConn+"/api/oauth/callback", authInfo[1], authInfo[2])
	return authInfo[0]
}

// HandleAuthCallback - handles oauth callback
// Note: not included in API reference as part of the OAuth process itself.
func HandleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if auth_provider == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintln(w, oauthNotConfigured)
		return
	}
	var functions = getCurrentAuthFunctions()
	if functions == nil {
		return
	}
	state, _ := getStateAndCode(r)
	_, err := netcache.Get(state) // if in netcache proceeed with node registration login
	if err == nil || len(state) == node_signin_length || errors.Is(err, netcache.ErrExpired) {
		logger.Log(0, "proceeding with node SSO callback")
		HandleNodeSSOCallback(w, r)
	} else { // handle normal login
		functions[handle_callback].(func(http.ResponseWriter, *http.Request))(w, r)
	}
}

// swagger:route GET /api/oauth/login nodes HandleAuthLogin
//
// Handles OAuth login.
//
//			Schemes: https
//
//			Security:
//	  		oauth
func HandleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if auth_provider == nil {
		var referer = r.Header.Get("referer")
		if referer != "" {
			http.Redirect(w, r, referer+"login?oauth=callback-error", http.StatusTemporaryRedirect)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintln(w, oauthNotConfigured)
		return
	}
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
		logger.Log(1, "error checking for existence of admin user during OAuth login for", email, "; user not added")
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
		if err = logic.CreateAdmin(&newUser); err != nil {
			logger.Log(1, "error creating admin from user,", email, "; user not added")
		} else {
			logger.Log(1, "admin created from user,", email, "; was first user added")
		}
	} else { // otherwise add to db as admin..?
		// TODO: add ability to add users with preemptive permissions
		newUser.IsAdmin = false
		if err = logic.CreateUser(&newUser); err != nil {
			logger.Log(1, "error creating user,", email, "; user not added")
		} else {
			logger.Log(0, "user created from ", email)
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
		logger.Log(0, "could not decode pass")
		return "", nil
	}
	return string(b64CurrentValue), nil
}

func getStateAndCode(r *http.Request) (string, string) {
	var state, code string
	if r.FormValue("state") != "" && r.FormValue("code") != "" {
		state = r.FormValue("state")
		code = r.FormValue("code")
	} else if r.URL.Query().Get("state") != "" && r.URL.Query().Get("code") != "" {
		state = r.URL.Query().Get("state")
		code = r.URL.Query().Get("code")
	}

	return state, code
}

func (user *OAuthUser) getUserName() string {
	var userName string
	if user.Email != "" {
		userName = user.Email
	} else if user.Login != "" {
		userName = user.Login
	} else if user.UserPrincipalName != "" {
		userName = user.UserPrincipalName
	} else if user.Name != "" {
		userName = user.Name
	}
	return userName
}

func isStateCached(state string) bool {
	_, err := netcache.Get(state)
	return err == nil || strings.Contains(err.Error(), "expired")
}
