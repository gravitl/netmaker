package auth

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/pro/netcache"
	"github.com/gravitl/netmaker/models"
)

var (
	redirectUrl string
)

// HandleHostSSOCallback handles the callback from the sso endpoint
// It is the analogue of auth.handleNodeSSOCallback but takes care of the end point flow
// Retrieves the mkey from the state cache and adds the machine to the users email namespace
// TODO: A confirmation page for new machines should be added to avoid phishing vulnerabilities
// TODO: Add groups information from OIDC tokens into machine HostInfo
// Listens in /oidc/callback.
func HandleHostSSOCallback(w http.ResponseWriter, r *http.Request) {

	var functions = getCurrentAuthFunctions()
	if functions == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad conf"))
		logger.Log(0, "Missing Oauth config in HandleNodeSSOCallback")
		return
	}

	state, code := getStateAndCode(r)

	var userClaims, err = functions[get_user_info].(func(string, string) (*OAuthUser, error))(state, code)
	if err != nil {
		logger.Log(0, "error when getting user info from callback:", err.Error())
		handleOauthNotConfigured(w)
		return
	}

	if code == "" || state == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Wrong params"))
		logger.Log(0, "Missing params in HandleSSOCallback")
		return
	}

	// all responses should be in html format from here on out
	w.Header().Add("content-type", "text/html; charset=utf-8")

	// retrieve machinekey from state cache
	reqKeyIf, machineKeyFoundErr := netcache.Get(state)
	if machineKeyFoundErr != nil {
		logger.Log(0, "requested machine state key expired before authorisation completed -", machineKeyFoundErr.Error())
		reqKeyIf = &netcache.CValue{
			Network:    "invalid",
			Value:      state,
			Pass:       "",
			User:       "netmaker",
			Expiration: time.Now(),
		}
		response := returnErrTemplate("", "requested machine state key expired before authorisation completed", state, reqKeyIf)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(response)
		return
	}
	// check if user exists
	user, err := logic.GetUser(userClaims.getUserName())
	if err != nil {
		handleOauthUserNotFound(w)
		return
	}
	if !user.IsAdmin || !user.IsSuperAdmin {
		handleOauthUserNotAllowed(w)
		return
	}
	logger.Log(1, "registering host for user:", userClaims.getUserName(), reqKeyIf.Host.Name, reqKeyIf.Host.ID.String())

	// Send OK to user in the browser
	var response bytes.Buffer
	if err := ssoCallbackTemplate.Execute(&response, ssoCallbackTemplateConfig{
		User: userClaims.getUserName(),
		Verb: "Authenticated",
	}); err != nil {
		logger.Log(0, "Could not render SSO callback template ", err.Error())
		response := returnErrTemplate(reqKeyIf.User, "Could not render SSO callback template", state, reqKeyIf)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(response)
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write(response.Bytes())
	}

	reqKeyIf.User = userClaims.getUserName() // set the cached registering hosts' user
	if err = netcache.Set(state, reqKeyIf); err != nil {
		logger.Log(0, "machine failed to complete join on network,", reqKeyIf.Network, "-", err.Error())
		return
	}
}

func setNetcache(ncache *netcache.CValue, state string) error {
	if ncache == nil {
		return fmt.Errorf("cache miss")
	}
	var err error
	if err = netcache.Set(state, ncache); err != nil {
		logger.Log(0, "machine failed to complete join on network,", ncache.Network, "-", err.Error())
	}
	return err
}

func returnErrTemplate(uname, message, state string, ncache *netcache.CValue) []byte {
	var response bytes.Buffer
	if ncache != nil {
		ncache.Pass = message
	}
	err := ssoErrCallbackTemplate.Execute(&response, ssoCallbackTemplateConfig{
		User: uname,
		Verb: message,
	})
	if err != nil {
		return []byte(err.Error())
	}
	err = setNetcache(ncache, state)
	if err != nil {
		return []byte(err.Error())
	}
	return response.Bytes()
}

// RegisterHostSSO redirects to the IDP for authentication
// Puts machine key in cache so the callback can retrieve it using the oidc state param
// Listens in /oidc/register/:regKey.
func RegisterHostSSO(w http.ResponseWriter, r *http.Request) {

	if auth_provider == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid login attempt"))
		return
	}
	vars := mux.Vars(r)

	// machineKeyStr this is not key but state
	machineKeyStr := vars["regKey"]
	if machineKeyStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid login attempt"))
		return
	}

	http.Redirect(w, r, auth_provider.AuthCodeURL(machineKeyStr), http.StatusSeeOther)
}

// == private ==

func isUserIsAllowed(username, network string, shouldAddUser bool) (*models.User, error) {

	user, err := logic.GetUser(username)
	if err != nil && shouldAddUser { // user must not exist, so try to make one
		if err = addUser(username); err != nil {
			logger.Log(0, "failed to add user", username, "during a node SSO network join on network", network)
			// response := returnErrTemplate(user.UserName, "failed to add user", state, reqKeyIf)
			// w.WriteHeader(http.StatusInternalServerError)
			// w.Write(response)
			return nil, fmt.Errorf("failed to add user to system")
		}
		logger.Log(0, "user", username, "was added during a node SSO network join on network", network)
		user, _ = logic.GetUser(username)
	}

	return user, nil
}
