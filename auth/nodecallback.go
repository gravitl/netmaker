package auth

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/pro"
	"github.com/gravitl/netmaker/logic/pro/netcache"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/models/promodels"
	"github.com/gravitl/netmaker/servercfg"
)

var (
	redirectUrl string
)

// HandleNodeSSOCallback handles the callback from the sso endpoint
// It is the analogue of auth.handleNodeSSOCallback but takes care of the end point flow
// Retrieves the mkey from the state cache and adds the machine to the users email namespace
// TODO: A confirmation page for new machines should be added to avoid phishing vulnerabilities
// TODO: Add groups information from OIDC tokens into machine HostInfo
// Listens in /oidc/callback.
func HandleNodeSSOCallback(w http.ResponseWriter, r *http.Request) {

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
		http.Redirect(w, r, servercfg.GetFrontendURL()+"/login?oauth=callback-error", http.StatusTemporaryRedirect)
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

	user, err := isUserIsAllowed(userClaims.getUserName(), reqKeyIf.Network, true)
	if err != nil {
		logger.Log(0, "error occurred during SSO node join for user", userClaims.getUserName(), "on network", reqKeyIf.Network, "-", err.Error())
		response := returnErrTemplate(user.UserName, err.Error(), state, reqKeyIf)
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write(response)
		return
	}

	logger.Log(1, "registering new node for user:", user.UserName, "on network", reqKeyIf.Network)

	// Send OK to user in the browser
	var response bytes.Buffer
	if err := ssoCallbackTemplate.Execute(&response, ssoCallbackTemplateConfig{
		User: userClaims.getUserName(),
		Verb: "Authenticated",
	}); err != nil {
		logger.Log(0, "Could not render SSO callback template ", err.Error())
		response := returnErrTemplate(user.UserName, "Could not render SSO callback template", state, reqKeyIf)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(response)

	} else {
		w.WriteHeader(http.StatusOK)
		w.Write(response.Bytes())
	}

	// Need to send access key to the client
	logger.Log(1, "Handling new machine addition to network",
		reqKeyIf.Network, "with key",
		reqKeyIf.Value, " identity:", userClaims.getUserName(), "claims:", fmt.Sprintf("%+v", userClaims))

	var answer string
	// The registation logic is starting here:
	// we request access key with 1 use for the required network
	accessToken, err := requestAccessKey(reqKeyIf.Network, 1, userClaims.getUserName())
	if err != nil {
		answer = fmt.Sprintf("Error from the netmaker controller %s", err.Error())
	} else {
		answer = fmt.Sprintf("AccessToken: %s", accessToken)
	}
	logger.Log(0, "Updating the token for the client request ... ")
	// Give the user the access token via Pass in the DB
	reqKeyIf.Pass = answer
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
	ncache.Pass = message
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

// RegisterNodeSSO redirects to the IDP for authentication
// Puts machine key in cache so the callback can retrieve it using the oidc state param
// Listens in /oidc/register/:regKey.
func RegisterNodeSSO(w http.ResponseWriter, r *http.Request) {

	if auth_provider == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid login attempt"))
		return
	}
	vars := mux.Vars(r)

	// machineKeyStr this is not key but state
	machineKeyStr := vars["regKey"]
	logger.Log(1, "requested key:", machineKeyStr)

	if machineKeyStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid login attempt"))
		return
	}

	// machineKeyStr this not key but state
	authURL := auth_provider.AuthCodeURL(machineKeyStr)
	//authURL = authURL + "&connector_id=" + "google"
	logger.Log(0, "Redirecting to ", authURL, " for authentication")

	http.Redirect(w, r, authURL, http.StatusSeeOther)

}

// == private ==
// API to create an access key for a given network with a given name
func requestAccessKey(network string, uses int, name string) (accessKey string, err error) {

	var sAccessKey models.AccessKey
	var sNetwork models.Network

	sNetwork, err = logic.GetParentNetwork(network)
	if err != nil {
		logger.Log(0, "err calling GetParentNetwork API=%s", err.Error())
		return "", fmt.Errorf("internal controller error %s", err.Error())
	}
	// If a key already exists, we recreate it.
	// @TODO Is that a preferred handling ? We could also trying to re-use.
	// can happen if user started log in but did not finish
	for _, currentkey := range sNetwork.AccessKeys {
		if currentkey.Name == name {
			logger.Log(0, "erasing existing AccessKey for: ", name)
			err = logic.DeleteKey(currentkey.Name, network)
			if err != nil {
				logger.Log(0, "err calling CreateAccessKey API ", err.Error())
				return "", fmt.Errorf("key already exists. Contact admin to resolve")
			}
			break
		}
	}
	// Only one usage is needed - for the next time new access key will be required
	// it will be created next time after another IdP approval
	sAccessKey.Uses = 1
	sAccessKey.Name = name

	accessToken, err := logic.CreateAccessKey(sAccessKey, sNetwork)
	if err != nil {
		logger.Log(0, "err calling CreateAccessKey API ", err.Error())
		return "", fmt.Errorf("error from the netmaker controller %s", err.Error())
	} else {
		logger.Log(1, "created access key", sAccessKey.Name, "on", network)
	}
	return accessToken.AccessString, nil
}

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

	if !user.IsAdmin { // perform check to see if user is allowed to join a node to network
		netUser, err := pro.GetNetworkUser(network, promodels.NetworkUserID(user.UserName))
		if err != nil {
			logger.Log(0, "failed to get net user details for user", user.UserName, "during node SSO")
			return nil, fmt.Errorf("failed to verify network user")
		}
		if netUser.AccessLevel != pro.NET_ADMIN { // if user is a net admin on network, good to go
			// otherwise, check if they have node access + haven't reached node limit on network
			if netUser.AccessLevel == pro.NODE_ACCESS {
				if len(netUser.Nodes) >= netUser.NodeLimit {
					logger.Log(0, "user", user.UserName, "has reached their node limit on network", network)
					return nil, fmt.Errorf("user node limit exceeded")
				}
			} else {
				logger.Log(0, "user", user.UserName, "attempted to access network", network, "via node SSO")
				return nil, fmt.Errorf("network user not allowed")
			}
		}
	}

	return user, nil
}
