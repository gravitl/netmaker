package auth

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/pro/netcache"
	"github.com/gravitl/netmaker/models"
)

// HandleHeadlessSSOCallback - handle OAuth callback for headless logins such as Netmaker CLI
func HandleHeadlessSSOCallback(w http.ResponseWriter, r *http.Request) {
	functions := getCurrentAuthFunctions()
	if functions == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad conf"))
		logger.Log(0, "Missing Oauth config in HandleHeadlessSSOCallback")
		return
	}
	state, code := getStateAndCode(r)

	userClaims, err := functions[get_user_info].(func(string, string) (*OAuthUser, error))(state, code)
	if err != nil {
		logger.Log(0, "error when getting user info from callback:", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Failed to retrieve OAuth user claims"))
		return
	}

	if code == "" || state == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Wrong params"))
		logger.Log(0, "Missing params in HandleHeadlessSSOCallback")
		return
	}

	// all responses should be in html format from here on out
	w.Header().Add("content-type", "text/html; charset=utf-8")

	// retrieve machinekey from state cache
	reqKeyIf, machineKeyFoundErr := netcache.Get(state)
	if machineKeyFoundErr != nil {
		logger.Log(0, "requested machine state key expired before authorisation completed -", machineKeyFoundErr.Error())
		response := returnErrTemplate("", "requested machine state key expired before authorisation completed", state, reqKeyIf)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(response)
		return
	}

	_, err = logic.GetUser(userClaims.getUserName())
	if err != nil { // user must not exists, so try to make one
		if err = addUser(userClaims.getUserName()); err != nil {
			logger.Log(1, "could not create new user: ", userClaims.getUserName())
			return
		}
	}
	newPass, fetchErr := logic.FetchPassValue("")
	if fetchErr != nil {
		return
	}
	jwt, jwtErr := logic.VerifyAuthRequest(models.UserAuthParams{
		UserName: userClaims.getUserName(),
		Password: newPass,
	})
	if jwtErr != nil {
		logger.Log(1, "could not parse jwt for user", userClaims.getUserName())
		return
	}

	logger.Log(1, "headless SSO login by user:", userClaims.getUserName())

	// Send OK to user in the browser
	var response bytes.Buffer
	if err := ssoCallbackTemplate.Execute(&response, ssoCallbackTemplateConfig{
		User: userClaims.getUserName(),
		Verb: "Authenticated",
	}); err != nil {
		logger.Log(0, "Could not render SSO callback template ", err.Error())
		response := returnErrTemplate(userClaims.getUserName(), "Could not render SSO callback template", state, reqKeyIf)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(response)
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write(response.Bytes())
	}
	reqKeyIf.Pass = fmt.Sprintf("JWT: %s", jwt)
	if err = netcache.Set(state, reqKeyIf); err != nil {
		logger.Log(0, "failed to set netcache for user", reqKeyIf.User, "-", err.Error())
	}
}
